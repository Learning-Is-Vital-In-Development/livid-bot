package study

import "time"

type Study struct {
	ID          int64
	Branch      string
	Name        string
	Description string
	ChannelID   string
	RoleID      string
	CreatedAt   time.Time
	Status      string
}

type StudyMember struct {
	StudyID  int64
	UserID   string
	Username string
	JoinedAt time.Time
	LeftAt   *time.Time
}

type RecruitMessage struct {
	ID        int64
	MessageID string
	ChannelID string
	CreatedAt time.Time
}

type RecruitMapping struct {
	RecruitMessageID int64
	Emoji            string
	StudyID          int64
	RoleID           string
}
