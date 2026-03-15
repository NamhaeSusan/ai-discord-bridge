package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestBuildClaudeArgs(t *testing.T) {
	cfg := BotConfig{
		Model:        "sonnet",
		MaxBudgetUSD: 2.5,
		AllowedTools: []string{"Read", "Grep"},
	}

	got := buildClaudeArgs(cfg, "session-1", "hello")
	want := []string{
		"-p",
		"--output-format", "json",
		"--dangerously-skip-permissions",
		"--resume", "session-1",
		"--model", "sonnet",
		"--max-budget-usd", "2.50",
		"--allowedTools", "Read",
		"--allowedTools", "Grep",
		"--",
		"hello",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildClaudeArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildCodexArgs(t *testing.T) {
	cfg := BotConfig{
		WorkingDir: "/repo",
		Model:      "gpt-5-codex",
		Sandbox:    "danger-full-access",
	}

	got := buildCodexArgs(cfg, false, "", "ship it", "/tmp/out", "")
	want := []string{
		"exec",
		"-C", "/repo",
		"-m", "gpt-5-codex",
		"--sandbox", "danger-full-access",
		"--json",
		"-o", "/tmp/out",
		"--",
		"ship it",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildCodexArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildCodexResumeArgs(t *testing.T) {
	cfg := BotConfig{
		Model:   "gpt-5-codex",
		Sandbox: "danger-full-access",
	}

	got := buildCodexArgs(cfg, true, "thread-1", "ship it", "/tmp/out", "")
	want := []string{
		"exec",
		"resume",
		"--json",
		"-o", "/tmp/out",
		"-m", "gpt-5-codex",
		"thread-1",
		"--",
		"ship it",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildCodexArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildCodexArgsWithOverrideWorkingDir(t *testing.T) {
	cfg := BotConfig{
		WorkingDir: "/repo",
		Model:      "gpt-5-codex",
		Sandbox:    "danger-full-access",
	}

	got := buildCodexArgs(cfg, false, "", "ship it", "/tmp/out", "/other")
	want := []string{
		"exec",
		"-C", "/other",
		"-m", "gpt-5-codex",
		"--sandbox", "danger-full-access",
		"--json",
		"-o", "/tmp/out",
		"--",
		"ship it",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildCodexArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestParseCodexJSONL(t *testing.T) {
	data := []byte("{\"type\":\"thread.started\",\"thread_id\":\"t-1\"}\n" +
		"{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"hello\"}}\n")

	threadID, text, err := parseCodexJSONL(data)
	if err != nil {
		t.Fatalf("parseCodexJSONL returned error: %v", err)
	}
	if threadID != "t-1" {
		t.Fatalf("expected thread id t-1, got %q", threadID)
	}
	if text != "hello" {
		t.Fatalf("expected text hello, got %q", text)
	}
}

func TestFormatResponseWithoutCost(t *testing.T) {
	result := &ProviderResult{
		Provider: "codex",
		Result:   "hello",
	}

	chunks := FormatResponse(result)
	if len(chunks) != 1 {
		t.Fatalf("expected single chunk, got %d", len(chunks))
	}
	if chunks[0] == "hello" {
		t.Fatalf("expected metadata to be appended")
	}
}

func TestFormatResponseWithEmptyResult(t *testing.T) {
	result := &ProviderResult{
		Provider: "claude",
		Result:   " \n\t",
	}

	chunks := FormatResponse(result)
	if len(chunks) != 1 {
		t.Fatalf("expected single chunk, got %d", len(chunks))
	}
	if got := chunks[0]; got == "" || got == "\n\n" {
		t.Fatalf("expected fallback message, got %q", got)
	}
	if want := "No response text was returned."; !strings.HasPrefix(chunks[0], want) {
		t.Fatalf("expected fallback prefix %q, got %q", want, chunks[0])
	}
}

func TestRunnerShouldHandleChannelMessage(t *testing.T) {
	runner := &Runner{
		session: &discordgo.Session{
			State: &discordgo.State{
				Ready: discordgo.Ready{
					User: &discordgo.User{ID: "bot-1"},
				},
			},
		},
	}

	if !runner.shouldHandleChannelMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{GuildID: ""},
	}) {
		t.Fatalf("expected DM message to be handled")
	}

	if runner.shouldHandleChannelMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			GuildID: "guild-1",
			Mentions: []*discordgo.User{
				{ID: "someone-else"},
			},
		},
	}) {
		t.Fatalf("expected guild message without bot mention to be ignored")
	}

	if !runner.shouldHandleChannelMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			GuildID: "guild-1",
			Mentions: []*discordgo.User{
				{ID: "bot-1"},
			},
		},
	}) {
		t.Fatalf("expected guild message with bot mention to be handled")
	}
}

func TestRunnerShouldHandleThreadMessage(t *testing.T) {
	registry := newThreadRegistry(nil)
	registry.Claim("thread-1", "claude")

	runner := &Runner{
		cfg:     BotConfig{Name: "claude"},
		threads: registry,
		session: &discordgo.Session{
			State: &discordgo.State{
				Ready: discordgo.Ready{
					User: &discordgo.User{ID: "bot-1"},
				},
			},
		},
	}

	if !runner.shouldHandleThreadMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{ChannelID: "thread-1", GuildID: "guild-1"},
	}) {
		t.Fatalf("expected claimed thread to be handled by owning bot")
	}

	other := &Runner{
		cfg:     BotConfig{Name: "codex"},
		threads: registry,
		session: runner.session,
	}

	if other.shouldHandleThreadMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{ChannelID: "thread-1", GuildID: "guild-1"},
	}) {
		t.Fatalf("expected claimed thread to be ignored by other bot")
	}

	if !other.shouldHandleThreadMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ChannelID: "thread-2",
			GuildID:   "guild-1",
			Mentions: []*discordgo.User{
				{ID: "bot-1"},
			},
		},
	}) {
		t.Fatalf("expected unclaimed thread with bot mention to be handled")
	}
}

func TestParseWorkdirDirectiveWithoutDirective(t *testing.T) {
	dir, prompt, err := parseWorkdirDirective("ship it")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "" {
		t.Fatalf("expected empty dir, got %q", dir)
	}
	if prompt != "ship it" {
		t.Fatalf("expected original prompt, got %q", prompt)
	}
}

func TestParseWorkdirDirectiveWithDirective(t *testing.T) {
	dir, prompt, err := parseWorkdirDirective("/cwd .\n\nship it")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir == "" {
		t.Fatalf("expected working dir to be resolved")
	}
	if prompt != "ship it" {
		t.Fatalf("expected trimmed prompt, got %q", prompt)
	}
}

func TestParseWorkdirDirectiveShowCwd(t *testing.T) {
	dir, prompt, err := parseWorkdirDirective("/cwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "" {
		t.Fatalf("expected empty dir, got %q", dir)
	}
	if prompt != "" {
		t.Fatalf("expected empty prompt, got %q", prompt)
	}
}

func TestParseWorkdirDirectiveChangeOnly(t *testing.T) {
	dir, prompt, err := parseWorkdirDirective("/cwd .")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir == "" {
		t.Fatalf("expected working dir to be resolved")
	}
	if prompt != "" {
		t.Fatalf("expected empty prompt, got %q", prompt)
	}
}

func TestValidateWorkingDirWithoutBaseDir(t *testing.T) {
	if err := validateWorkingDir("", "/Users/kimtaeyun/ai-discord-bridge"); err != nil {
		t.Fatalf("expected /cwd to be allowed without base dir, got %v", err)
	}
}

func TestValidateWorkingDirRejectsOutsideBaseDir(t *testing.T) {
	if err := validateWorkingDir("/Users/kimtaeyun", "/tmp"); err == nil {
		t.Fatal("expected path outside base dir to be rejected")
	}
}

func TestStripBotMention(t *testing.T) {
	tests := []struct {
		content string
		botID   string
		want    string
	}{
		{"<@123> /cwd .", "123", "/cwd ."},
		{"<@!123> /cwd .", "123", "/cwd ."},
		{"<@123> hello world", "123", "hello world"},
		{"/cwd .", "123", "/cwd ."},
		{"<@456> /cwd .", "123", "<@456> /cwd ."},
		{"<@123>", "123", ""},
	}

	for _, tt := range tests {
		got := stripBotMention(tt.content, tt.botID)
		if got != tt.want {
			t.Errorf("stripBotMention(%q, %q) = %q, want %q", tt.content, tt.botID, got, tt.want)
		}
	}
}
