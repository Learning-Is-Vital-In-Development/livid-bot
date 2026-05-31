package bot

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscordRESTCallsUseActiveContext(t *testing.T) {
	methods := map[string]bool{
		"Channel":                   true,
		"ChannelDelete":             true,
		"ChannelEdit":               true,
		"ChannelMessage":            true,
		"ChannelMessageDelete":      true,
		"ChannelMessageEdit":        true,
		"ChannelMessageSend":        true,
		"ChannelPermissionSet":      true,
		"GuildChannelCreateComplex": true,
		"GuildChannels":             true,
		"GuildMember":               true,
		"GuildMemberRoleAdd":        true,
		"GuildMemberRoleRemove":     true,
		"GuildRoleCreate":           true,
		"GuildRoleDelete":           true,
		"GuildRoles":                true,
		"InteractionRespond":        true,
		"InteractionResponseEdit":   true,
		"MessageReactionAdd":        true,
	}

	files := []string{
		"archive_category.go",
		"handler_archive.go",
		"handler_create_study.go",
		"handler_help.go",
		"handler_members.go",
		"handler_recruit.go",
		"handler_studies.go",
		"handler_study_start.go",
		"handler_suggest.go",
		"handler_vote.go",
		"helpers.go",
		"reaction.go",
		"voice_attendance.go",
	}

	fset := token.NewFileSet()
	for _, name := range files {
		path := filepath.Join(".", name)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !methods[selector.Sel.Name] {
				return true
			}
			if hasDiscordWithContext(call.Args) {
				return true
			}
			pos := fset.Position(call.Pos())
			t.Errorf("%s must pass discordgo.WithContext(ctx) so REST spans inherit the active trace", pos)
			return true
		})
	}
}

func hasDiscordWithContext(args []ast.Expr) bool {
	for _, arg := range args {
		call, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "WithContext" {
			continue
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && strings.EqualFold(ident.Name, "discordgo") {
			return true
		}
	}
	return false
}
