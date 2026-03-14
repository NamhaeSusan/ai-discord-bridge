package main

import "testing"

func TestConfigNormalizeLegacyToBots(t *testing.T) {
	cfg := &Config{
		BotToken:             "legacy-token",
		AllowedChannels:      []string{"c1"},
		AllowedUsers:         []string{"u1"},
		ClaudeWorkingDir:     "/tmp/project",
		ClaudeModel:          "sonnet",
		ClaudeTimeoutSeconds: 123,
		MaxConcurrent:        7,
		SessionTTLMinutes:    45,
		ClaudeMaxBudgetUSD:   4.2,
		ClaudeAllowedTools:   []string{"Read"},
	}

	cfg.normalize()

	if len(cfg.Bots) != 1 {
		t.Fatalf("expected 1 bot, got %d", len(cfg.Bots))
	}

	bot := cfg.Bots[0]
	if bot.Provider != "claude" {
		t.Fatalf("expected claude provider, got %q", bot.Provider)
	}
	if bot.BotToken != "legacy-token" {
		t.Fatalf("expected token to be copied")
	}
	if bot.WorkingDir != "/tmp/project" || bot.Model != "sonnet" {
		t.Fatalf("expected legacy claude config to be copied")
	}
	if bot.TimeoutSeconds != 123 || bot.MaxConcurrent != 7 || bot.SessionTTLMinutes != 45 {
		t.Fatalf("expected legacy limits to be copied")
	}
	if bot.MaxBudgetUSD != 4.2 {
		t.Fatalf("expected budget to be copied")
	}
	if len(bot.AllowedChannels) != 1 || bot.AllowedChannels[0] != "c1" {
		t.Fatalf("expected allowed channels to be copied")
	}
}

func TestBotConfigNormalizeDefaults(t *testing.T) {
	bot := BotConfig{Provider: "codex"}
	bot.normalize(0)

	if bot.Name != "codex-1" {
		t.Fatalf("expected generated name, got %q", bot.Name)
	}
	if bot.TimeoutSeconds != defaultTimeoutSeconds {
		t.Fatalf("expected default timeout, got %d", bot.TimeoutSeconds)
	}
	if bot.MaxConcurrent != defaultMaxConcurrent {
		t.Fatalf("expected default concurrency, got %d", bot.MaxConcurrent)
	}
	if bot.SessionTTLMinutes != defaultSessionTTL {
		t.Fatalf("expected default session ttl, got %d", bot.SessionTTLMinutes)
	}
	if bot.Sandbox != defaultCodexSandbox {
		t.Fatalf("expected default sandbox, got %q", bot.Sandbox)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := &Config{
		Bots: []BotConfig{{
			Name:     "codex",
			Provider: "codex",
			BotToken: "token",
		}},
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
}
