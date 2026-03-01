package bot

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

const (
	archiveCategoryPrefix      = "archive"
	archiveCategoryMaxChannels = 50
)

const archiveReadOnlyDenyMask = discordgo.PermissionSendMessages |
	discordgo.PermissionCreatePublicThreads |
	discordgo.PermissionCreatePrivateThreads |
	discordgo.PermissionSendMessagesInThreads

var archiveCategoryNamePattern = regexp.MustCompile(`^archive([1-9][0-9]*)$`)

type archiveCategorySlot struct {
	ID           string
	Name         string
	Number       int
	ChannelCount int
	Allow        int64
	Deny         int64
}

type archiveCategoryAllocator struct {
	session         *discordgo.Session
	guildID         string
	slots           []archiveCategorySlot
	readOnlyEnsured map[string]bool
}

type archiveCategoryReservation struct {
	slot        *archiveCategorySlot
	isCommitted bool
	isReleased  bool
}

type archiveDryRunPlan struct {
	Assignments       []string
	CategoryUseCounts map[string]int
	CreatedCategories []string
}

func newArchiveCategoryAllocator(s *discordgo.Session, guildID string) (*archiveCategoryAllocator, error) {
	channels, err := s.GuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("load guild channels: %w", err)
	}

	slots := make([]archiveCategorySlot, 0)
	slotIndexByID := make(map[string]int)
	for _, ch := range channels {
		if ch.Type != discordgo.ChannelTypeGuildCategory {
			continue
		}
		n, ok := parseArchiveCategoryNumber(ch.Name)
		if !ok {
			continue
		}
		allow, deny := everyoneOverwriteBits(ch, guildID)
		slot := archiveCategorySlot{
			ID:           ch.ID,
			Name:         ch.Name,
			Number:       n,
			ChannelCount: 0,
			Allow:        allow,
			Deny:         deny,
		}
		slotIndexByID[ch.ID] = len(slots)
		slots = append(slots, slot)
	}

	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildCategory {
			continue
		}
		if idx, ok := slotIndexByID[ch.ParentID]; ok {
			slots[idx].ChannelCount++
		}
	}

	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Number < slots[j].Number
	})

	return &archiveCategoryAllocator{
		session:         s,
		guildID:         guildID,
		slots:           slots,
		readOnlyEnsured: make(map[string]bool),
	}, nil
}

func (a *archiveCategoryAllocator) Plan(totalChannels int) archiveDryRunPlan {
	return planArchiveCategoryAssignments(a.slots, totalChannels)
}

func (a *archiveCategoryAllocator) Reserve() (categoryID, categoryName string, reservation *archiveCategoryReservation, err error) {
	slot, err := a.getOrCreateWritableSlot()
	if err != nil {
		return "", "", nil, err
	}
	if err := a.ensureReadOnly(slot); err != nil {
		return "", "", nil, err
	}
	return slot.ID, slot.Name, &archiveCategoryReservation{slot: slot}, nil
}

func (a *archiveCategoryAllocator) getOrCreateWritableSlot() (*archiveCategorySlot, error) {
	if len(a.slots) == 0 {
		return a.createSlot(1)
	}

	lastIdx := len(a.slots) - 1
	if a.slots[lastIdx].ChannelCount < archiveCategoryMaxChannels {
		return &a.slots[lastIdx], nil
	}

	return a.createSlot(a.slots[lastIdx].Number + 1)
}

func (a *archiveCategoryAllocator) createSlot(number int) (*archiveCategorySlot, error) {
	name := fmt.Sprintf("%s%d", archiveCategoryPrefix, number)
	category, err := a.session.GuildChannelCreateComplex(a.guildID, discordgo.GuildChannelCreateData{
		Name: name,
		Type: discordgo.ChannelTypeGuildCategory,
	})
	if err != nil {
		return nil, fmt.Errorf("create category %q: %w", name, err)
	}

	slot := archiveCategorySlot{
		ID:           category.ID,
		Name:         category.Name,
		Number:       number,
		ChannelCount: 0,
		Allow:        0,
		Deny:         0,
	}
	a.slots = append(a.slots, slot)
	return &a.slots[len(a.slots)-1], nil
}

func (a *archiveCategoryAllocator) ensureReadOnly(slot *archiveCategorySlot) error {
	if a.readOnlyEnsured[slot.ID] {
		return nil
	}

	allow, deny := mergeReadOnlyOverwrite(slot.Allow, slot.Deny)
	if err := a.session.ChannelPermissionSet(
		slot.ID,
		a.guildID,
		discordgo.PermissionOverwriteTypeRole,
		allow,
		deny,
	); err != nil {
		return fmt.Errorf("set read-only on %s: %w", slot.Name, err)
	}

	slot.Allow = allow
	slot.Deny = deny
	a.readOnlyEnsured[slot.ID] = true
	return nil
}

func parseArchiveCategoryNumber(name string) (int, bool) {
	match := archiveCategoryNamePattern.FindStringSubmatch(name)
	if len(match) != 2 {
		return 0, false
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

func everyoneOverwriteBits(ch *discordgo.Channel, guildID string) (allow, deny int64) {
	for _, overwrite := range ch.PermissionOverwrites {
		if overwrite.Type == discordgo.PermissionOverwriteTypeRole && overwrite.ID == guildID {
			return overwrite.Allow, overwrite.Deny
		}
	}
	return 0, 0
}

func mergeReadOnlyOverwrite(currentAllow, currentDeny int64) (newAllow, newDeny int64) {
	newAllow = currentAllow &^ archiveReadOnlyDenyMask
	newDeny = currentDeny | archiveReadOnlyDenyMask
	return newAllow, newDeny
}

func (r *archiveCategoryReservation) Commit() {
	if r == nil || r.slot == nil || r.isCommitted {
		return
	}
	r.slot.ChannelCount++
	r.isCommitted = true
}

func (r *archiveCategoryReservation) Release() {
	if r == nil || r.slot == nil || !r.isCommitted || r.isReleased {
		return
	}
	if r.slot.ChannelCount > 0 {
		r.slot.ChannelCount--
	}
	r.isReleased = true
}

func planArchiveCategoryAssignments(existingSlots []archiveCategorySlot, totalChannels int) archiveDryRunPlan {
	plan := archiveDryRunPlan{
		Assignments:       make([]string, 0, totalChannels),
		CategoryUseCounts: make(map[string]int),
		CreatedCategories: make([]string, 0),
	}
	if totalChannels <= 0 {
		return plan
	}

	countByNumber := make(map[int]int, len(existingSlots))
	nameByNumber := make(map[int]string, len(existingSlots))
	maxNumber := 0
	for _, slot := range existingSlots {
		countByNumber[slot.Number] = slot.ChannelCount
		nameByNumber[slot.Number] = slot.Name
		if slot.Number > maxNumber {
			maxNumber = slot.Number
		}
	}

	currentNumber := maxNumber
	if currentNumber == 0 {
		currentNumber = 1
		nameByNumber[currentNumber] = fmt.Sprintf("%s%d", archiveCategoryPrefix, currentNumber)
		plan.CreatedCategories = append(plan.CreatedCategories, nameByNumber[currentNumber])
	}

	for idx := 0; idx < totalChannels; idx++ {
		for countByNumber[currentNumber] >= archiveCategoryMaxChannels {
			currentNumber++
			if _, exists := nameByNumber[currentNumber]; !exists {
				nameByNumber[currentNumber] = fmt.Sprintf("%s%d", archiveCategoryPrefix, currentNumber)
				plan.CreatedCategories = append(plan.CreatedCategories, nameByNumber[currentNumber])
			}
		}

		targetName := nameByNumber[currentNumber]
		plan.Assignments = append(plan.Assignments, targetName)
		plan.CategoryUseCounts[targetName]++
		countByNumber[currentNumber]++
	}

	return plan
}
