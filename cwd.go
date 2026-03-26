package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const cwdRecentLimit = 10

type cwdCommandKind string

const (
	cwdCommandPrompt      cwdCommandKind = "prompt"
	cwdCommandShow        cwdCommandKind = "show"
	cwdCommandSet         cwdCommandKind = "set"
	cwdCommandRecentList  cwdCommandKind = "recent_list"
	cwdCommandRecentUse   cwdCommandKind = "recent_use"
	cwdCommandAliasAdd    cwdCommandKind = "alias_add"
	cwdCommandAliasRemove cwdCommandKind = "alias_remove"
	cwdCommandAliasList   cwdCommandKind = "alias_list"
)

type cwdCommand struct {
	Kind   cwdCommandKind
	Prompt string
	Target string
	Alias  string
	Path   string
	Index  int
}

type cwdComponentAction string

const (
	cwdComponentAliasSelect   cwdComponentAction = "alias"
	cwdComponentRecentSelect  cwdComponentAction = "recent"
	cwdComponentRefreshButton cwdComponentAction = "refresh"
)

func parseCwdCommand(content string) (cwdCommand, error) {
	if content == "" {
		return cwdCommand{}, fmt.Errorf("message cannot be empty")
	}

	lines := strings.Split(content, "\n")
	first := strings.TrimSpace(lines[0])
	if first == "" || !strings.HasPrefix(first, "/") {
		return cwdCommand{
			Kind:   cwdCommandPrompt,
			Prompt: content,
		}, nil
	}

	if first == "/cwd" {
		if prompt := joinLines(lines[1:]); prompt != "" {
			return cwdCommand{}, fmt.Errorf("`/cwd` does not accept a prompt")
		}
		return cwdCommand{Kind: cwdCommandShow}, nil
	}

	const prefix = "/cwd "
	if !strings.HasPrefix(first, prefix) {
		return cwdCommand{
			Kind:   cwdCommandPrompt,
			Prompt: content,
		}, nil
	}

	rest := strings.TrimSpace(strings.TrimPrefix(first, prefix))
	if rest == "" {
		return cwdCommand{Kind: cwdCommandShow}, nil
	}

	fields := strings.Fields(rest)
	switch fields[0] {
	case "recent":
		if prompt := joinLines(lines[1:]); prompt != "" {
			return cwdCommand{}, fmt.Errorf("`/cwd recent` does not accept a prompt")
		}
		if len(fields) == 1 {
			return cwdCommand{Kind: cwdCommandRecentList}, nil
		}
		if len(fields) != 2 {
			return cwdCommand{}, fmt.Errorf("usage: `/cwd recent` or `/cwd recent <n>`")
		}
		index, err := strconv.Atoi(fields[1])
		if err != nil || index <= 0 {
			return cwdCommand{}, fmt.Errorf("recent index must be a positive number")
		}
		return cwdCommand{
			Kind:  cwdCommandRecentUse,
			Index: index,
		}, nil
	case "alias":
		if prompt := joinLines(lines[1:]); prompt != "" {
			return cwdCommand{}, fmt.Errorf("`/cwd alias` commands do not accept a prompt")
		}
		if len(fields) < 2 {
			return cwdCommand{}, fmt.Errorf("usage: `/cwd alias list|add|rm ...`")
		}
		switch fields[1] {
		case "list":
			if len(fields) != 2 {
				return cwdCommand{}, fmt.Errorf("usage: `/cwd alias list`")
			}
			return cwdCommand{Kind: cwdCommandAliasList}, nil
		case "add":
			args := strings.TrimSpace(strings.TrimPrefix(rest, "alias add"))
			parts := strings.Fields(args)
			if len(parts) < 2 {
				return cwdCommand{}, fmt.Errorf("usage: `/cwd alias add <name> <path>`")
			}
			alias := normalizeAlias(parts[0])
			if !isValidAlias(alias) {
				return cwdCommand{}, fmt.Errorf("invalid alias name `%s`", parts[0])
			}
			path := strings.TrimSpace(args[len(parts[0]):])
			if path == "" {
				return cwdCommand{}, fmt.Errorf("usage: `/cwd alias add <name> <path>`")
			}
			return cwdCommand{
				Kind:  cwdCommandAliasAdd,
				Alias: alias,
				Path:  path,
			}, nil
		case "rm":
			if len(fields) != 3 {
				return cwdCommand{}, fmt.Errorf("usage: `/cwd alias rm <name>`")
			}
			alias := normalizeAlias(fields[2])
			if !isValidAlias(alias) {
				return cwdCommand{}, fmt.Errorf("invalid alias name `%s`", fields[2])
			}
			return cwdCommand{
				Kind:  cwdCommandAliasRemove,
				Alias: alias,
			}, nil
		default:
			return cwdCommand{}, fmt.Errorf("usage: `/cwd alias list|add|rm ...`")
		}
	default:
		return cwdCommand{
			Kind:   cwdCommandSet,
			Target: rest,
			Prompt: joinLines(lines[1:]),
		}, nil
	}
}

func parseWorkdirDirective(content string) (string, string, error) {
	cmd, err := parseCwdCommand(content)
	if err != nil {
		return "", "", err
	}
	if cmd.Kind == cwdCommandPrompt {
		return "", cmd.Prompt, nil
	}
	if cmd.Kind != cwdCommandSet {
		return "", "", nil
	}

	baseDir, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("resolve working directory: %w", err)
	}
	dir, err := resolveCwdTarget(cmd.Target, baseDir, nil)
	if err != nil {
		return "", "", err
	}
	return dir, cmd.Prompt, nil
}

func resolveCwdTarget(target, baseDir string, aliases map[string]string) (string, error) {
	if path, ok := aliases[normalizeAlias(target)]; ok {
		return ensureDirectory(path)
	}

	expanded, err := expandUserHome(target)
	if err != nil {
		return "", err
	}

	if !filepath.IsAbs(expanded) {
		if baseDir == "" {
			baseDir, err = os.Getwd()
			if err != nil {
				return "", fmt.Errorf("resolve base working directory: %w", err)
			}
		}
		expanded = filepath.Join(baseDir, expanded)
	}

	return ensureDirectory(expanded)
}

func validateWorkingDir(_ string, targetDir string) error {
	if targetDir == "" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve user home directory: %v", err)
	}
	absHome, err := filepath.Abs(home)
	if err != nil {
		return fmt.Errorf("invalid home directory: %v", err)
	}
	if resolvedHome, err := filepath.EvalSymlinks(absHome); err == nil {
		absHome = resolvedHome
	}
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("invalid working directory: %v", err)
	}
	if resolvedTarget, err := filepath.EvalSymlinks(absTarget); err == nil {
		absTarget = resolvedTarget
	}
	if absTarget == absHome {
		return nil
	}
	if !strings.HasPrefix(absTarget, absHome+string(filepath.Separator)) {
		return fmt.Errorf("path `%s` is outside allowed home directory `%s`", absTarget, absHome)
	}
	return nil
}

func buildCwdComponents(currentDir string, aliases map[string]string, recents []string, userID string) []discordgo.MessageComponent {
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				buildAliasSelect(currentDir, aliases, userID),
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				buildRecentSelect(currentDir, recents, userID),
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Refresh",
					Style:    discordgo.SecondaryButton,
					CustomID: makeCwdComponentID(cwdComponentRefreshButton, userID),
				},
			},
		},
	}
	return components
}

func makeCwdComponentID(action cwdComponentAction, userID string) string {
	return "cwd:" + string(action) + ":" + userID
}

func parseCwdComponentID(customID string) (cwdComponentAction, string, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 3 || parts[0] != "cwd" || parts[2] == "" {
		return "", "", fmt.Errorf("invalid cwd component id")
	}
	return cwdComponentAction(parts[1]), parts[2], nil
}

func buildCwdPickerMessage(dir string, aliases map[string]string, recents []string, userID string) *discordgo.MessageSend {
	return &discordgo.MessageSend{
		Content:    cwdPickerMessage(dir),
		Components: buildCwdComponents(dir, aliases, recents, userID),
	}
}

func cwdPickerMessage(dir string) string {
	if dir == "" {
		return "Working directory not set. Select an alias or recent path, or use `/cwd <alias|path>`."
	}
	return fmt.Sprintf("Working directory: `%s`\nSelect an alias or recent path, or use `/cwd <alias|path>`.", dir)
}

func formatAliasList(aliases map[string]string) string {
	if len(aliases) == 0 {
		return "No aliases configured."
	}

	names := sortedAliasNames(aliases)
	lines := make([]string, 0, len(names)+1)
	lines = append(lines, "Aliases:")
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("- `%s` -> `%s`", name, aliases[name]))
	}
	return strings.Join(lines, "\n")
}

func formatRecentList(recents []string) string {
	if len(recents) == 0 {
		return "No recent working directories."
	}

	lines := make([]string, 0, len(recents)+1)
	lines = append(lines, "Recent working directories:")
	for i, dir := range recents {
		lines = append(lines, fmt.Sprintf("%d. `%s`", i+1, dir))
	}
	return strings.Join(lines, "\n")
}

func normalizeAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

func isValidAlias(alias string) bool {
	if alias == "" {
		return false
	}
	switch alias {
	case "recent", "alias", "list":
		return false
	}
	for _, r := range alias {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func ensureDirectory(path string) (string, error) {
	absDir, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid `/cwd` path: %v", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return "", fmt.Errorf("working directory not found: `%s`", absDir)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("working directory is not a directory: `%s`", absDir)
	}
	return absDir, nil
}

func expandUserHome(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %v", err)
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}

func buildAliasSelect(currentDir string, aliases map[string]string, userID string) discordgo.SelectMenu {
	names := sortedAliasNames(aliases)
	options := make([]discordgo.SelectMenuOption, 0, min(len(names), 25))
	for _, name := range names {
		options = append(options, discordgo.SelectMenuOption{
			Label:       name,
			Value:       name,
			Description: truncateComponentText(aliases[name], 100),
			Default:     aliases[name] == currentDir,
		})
		if len(options) == 25 {
			break
		}
	}
	if len(options) == 0 {
		options = []discordgo.SelectMenuOption{{
			Label: "No aliases configured",
			Value: "none",
		}}
	}
	return discordgo.SelectMenu{
		CustomID:    makeCwdComponentID(cwdComponentAliasSelect, userID),
		Placeholder: "Select alias",
		Options:     options,
		Disabled:    len(names) == 0,
	}
}

func buildRecentSelect(currentDir string, recents []string, userID string) discordgo.SelectMenu {
	options := make([]discordgo.SelectMenuOption, 0, min(len(recents), 25))
	for i, dir := range recents {
		options = append(options, discordgo.SelectMenuOption{
			Label:       fmt.Sprintf("%d", i+1),
			Value:       strconv.Itoa(i + 1),
			Description: truncateComponentText(dir, 100),
			Default:     dir == currentDir,
		})
		if len(options) == 25 {
			break
		}
	}
	if len(options) == 0 {
		options = []discordgo.SelectMenuOption{{
			Label: "No recent paths",
			Value: "none",
		}}
	}
	return discordgo.SelectMenu{
		CustomID:    makeCwdComponentID(cwdComponentRecentSelect, userID),
		Placeholder: "Select recent path",
		Options:     options,
		Disabled:    len(recents) == 0,
	}
}

func sortedAliasNames(aliases map[string]string) []string {
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func truncateComponentText(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
