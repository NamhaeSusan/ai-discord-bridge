package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestParseCwdCommand(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    cwdCommand
		wantErr bool
	}{
		{
			name:    "plain prompt",
			content: "ship it",
			want: cwdCommand{
				Kind:   cwdCommandPrompt,
				Prompt: "ship it",
			},
		},
		{
			name:    "show current dir",
			content: "/cwd",
			want: cwdCommand{
				Kind: cwdCommandShow,
			},
		},
		{
			name:    "set path with prompt",
			content: "/cwd .\n\nship it",
			want: cwdCommand{
				Kind:   cwdCommandSet,
				Target: ".",
				Prompt: "ship it",
			},
		},
		{
			name:    "recent list",
			content: "/cwd recent",
			want: cwdCommand{
				Kind: cwdCommandRecentList,
			},
		},
		{
			name:    "recent pick",
			content: "/cwd recent 2",
			want: cwdCommand{
				Kind:  cwdCommandRecentUse,
				Index: 2,
			},
		},
		{
			name:    "alias add",
			content: "/cwd alias add api ./config",
			want: cwdCommand{
				Kind:  cwdCommandAliasAdd,
				Alias: "api",
				Path:  "./config",
			},
		},
		{
			name:    "alias remove",
			content: "/cwd alias rm api",
			want: cwdCommand{
				Kind:  cwdCommandAliasRemove,
				Alias: "api",
			},
		},
		{
			name:    "alias list",
			content: "/cwd alias list",
			want: cwdCommand{
				Kind: cwdCommandAliasList,
			},
		},
		{
			name:    "other slash command stays prompt",
			content: "/help",
			want: cwdCommand{
				Kind:   cwdCommandPrompt,
				Prompt: "/help",
			},
		},
		{
			name:    "invalid recent index",
			content: "/cwd recent 0",
			wantErr: true,
		},
		{
			name:    "invalid alias name",
			content: "/cwd alias add bad/name ./config",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCwdCommand(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseCwdCommand mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}

func TestResolveCwdTargetPrefersAlias(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	got, err := resolveCwdTarget("config", wd, map[string]string{
		"config": wd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wd {
		t.Fatalf("expected alias path %q, got %q", wd, got)
	}
}

func TestResolveCwdTargetResolvesRelativeToBaseDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	got, err := resolveCwdTarget("config", wd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(wd, "config")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestValidateWorkingDirAllowsOnlyHomeSubtree(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("user home: %v", err)
	}
	if err := validateWorkingDir("", home); err != nil {
		t.Fatalf("expected home path to be allowed, got %v", err)
	}
	if err := validateWorkingDir("", "/tmp"); err == nil {
		t.Fatal("expected path outside home to be rejected")
	}
}

func TestBuildCwdComponents(t *testing.T) {
	components := buildCwdComponents("/repo", map[string]string{
		"api": "/repo",
	}, []string{"/repo", "/repo/config"}, "user-1")

	if len(components) != 3 {
		t.Fatalf("expected 3 component rows, got %d", len(components))
	}

	aliasRow, ok := components[0].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("expected alias row, got %T", components[0])
	}
	aliasMenu, ok := aliasRow.Components[0].(discordgo.SelectMenu)
	if !ok {
		t.Fatalf("expected alias menu, got %T", aliasRow.Components[0])
	}
	if aliasMenu.CustomID != makeCwdComponentID(cwdComponentAliasSelect, "user-1") {
		t.Fatalf("unexpected alias custom id %q", aliasMenu.CustomID)
	}

	recentRow, ok := components[1].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("expected recent row, got %T", components[1])
	}
	recentMenu, ok := recentRow.Components[0].(discordgo.SelectMenu)
	if !ok {
		t.Fatalf("expected recent menu, got %T", recentRow.Components[0])
	}
	if len(recentMenu.Options) != 2 {
		t.Fatalf("expected 2 recent options, got %d", len(recentMenu.Options))
	}

	refreshRow, ok := components[2].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("expected refresh row, got %T", components[2])
	}
	button, ok := refreshRow.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("expected refresh button, got %T", refreshRow.Components[0])
	}
	if button.CustomID != makeCwdComponentID(cwdComponentRefreshButton, "user-1") {
		t.Fatalf("unexpected refresh custom id %q", button.CustomID)
	}
}

func TestParseCwdComponentID(t *testing.T) {
	action, userID, err := parseCwdComponentID(makeCwdComponentID(cwdComponentRecentSelect, "user-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != cwdComponentRecentSelect {
		t.Fatalf("expected recent action, got %q", action)
	}
	if userID != "user-1" {
		t.Fatalf("expected user-1, got %q", userID)
	}
}
