package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	defaultProvider       = "claude"
	defaultTimeoutSeconds = 300
	defaultMaxConcurrent  = 5
	defaultSessionTTL     = 60
	defaultCodexSandbox   = "danger-full-access"
	defaultDBPath         = "./sessions.db"
)

type Config struct {
	Bots   []BotConfig `toml:"bots"`
	DBPath string      `toml:"db_path"`

	BotToken             string   `toml:"bot_token"`
	AllowedChannels      []string `toml:"allowed_channels"`
	AllowedUsers         []string `toml:"allowed_users"`
	ClaudeWorkingDir     string   `toml:"claude_working_dir"`
	ClaudeModel          string   `toml:"claude_model"`
	ClaudeMaxBudgetUSD   float64  `toml:"claude_max_budget_usd"`
	ClaudeAllowedTools   []string `toml:"claude_allowed_tools"`
	ClaudeTimeoutSeconds int      `toml:"claude_timeout_seconds"`
	MaxConcurrent        int      `toml:"max_concurrent"`
	SessionTTLMinutes    int      `toml:"session_ttl_minutes"`
}

type BotConfig struct {
	Name              string   `toml:"name"`
	Provider          string   `toml:"provider"`
	BotToken          string   `toml:"bot_token"`
	AllowedChannels   []string `toml:"allowed_channels"`
	AllowedUsers      []string `toml:"allowed_users"`
	WorkingDir        string   `toml:"working_dir"`
	Model             string   `toml:"model"`
	TimeoutSeconds    int      `toml:"timeout_seconds"`
	MaxConcurrent     int      `toml:"max_concurrent"`
	SessionTTLMinutes int      `toml:"session_ttl_minutes"`
	MaxBudgetUSD      float64  `toml:"max_budget_usd"`
	AllowedTools      []string `toml:"allowed_tools"`
	Sandbox           string   `toml:"sandbox"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}

	path = configPathFromEnv(path)
	if path != "" {
		if _, err := toml.DecodeFile(path, cfg); err != nil {
			return nil, fmt.Errorf("load config file %s: %w", path, err)
		}
	}

	applyLegacyEnvOverrides(cfg)
	cfg.normalize()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func applyLegacyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DISCORD_BOT_TOKEN"); v != "" {
		cfg.BotToken = v
	}
	if v := os.Getenv("DISCORD_ALLOWED_CHANNELS"); v != "" {
		cfg.AllowedChannels = splitCSV(v)
	}
	if v := os.Getenv("DISCORD_ALLOWED_USERS"); v != "" {
		cfg.AllowedUsers = splitCSV(v)
	}
	if v := os.Getenv("CLAUDE_WORKING_DIR"); v != "" {
		cfg.ClaudeWorkingDir = v
	}
	if v := os.Getenv("CLAUDE_MODEL"); v != "" {
		cfg.ClaudeModel = v
	}
	if v := os.Getenv("CLAUDE_ALLOWED_TOOLS"); v != "" {
		cfg.ClaudeAllowedTools = splitCSV(v)
	}
	if v := os.Getenv("CLAUDE_MAX_BUDGET_USD"); v != "" {
		cfg.ClaudeMaxBudgetUSD = parseFloatOrExit("CLAUDE_MAX_BUDGET_USD", v)
	}
	if v := os.Getenv("CLAUDE_TIMEOUT_SECONDS"); v != "" {
		cfg.ClaudeTimeoutSeconds = parseIntOrExit("CLAUDE_TIMEOUT_SECONDS", v)
	}
}

func (c *Config) normalize() {
	if len(c.Bots) == 0 && c.BotToken != "" {
		c.Bots = []BotConfig{{
			Name:              defaultProvider,
			Provider:          defaultProvider,
			BotToken:          c.BotToken,
			AllowedChannels:   append([]string(nil), c.AllowedChannels...),
			AllowedUsers:      append([]string(nil), c.AllowedUsers...),
			WorkingDir:        c.ClaudeWorkingDir,
			Model:             c.ClaudeModel,
			TimeoutSeconds:    c.ClaudeTimeoutSeconds,
			MaxConcurrent:     c.MaxConcurrent,
			SessionTTLMinutes: c.SessionTTLMinutes,
			MaxBudgetUSD:      c.ClaudeMaxBudgetUSD,
			AllowedTools:      append([]string(nil), c.ClaudeAllowedTools...),
		}}
	}

	if c.DBPath == "" {
		c.DBPath = defaultDBPath
	}

	for i := range c.Bots {
		c.Bots[i].normalize(i)
	}
}

func (c *Config) validate() error {
	if len(c.Bots) == 0 {
		return fmt.Errorf("at least one [[bots]] entry is required")
	}

	for i, bot := range c.Bots {
		if bot.BotToken == "" {
			return fmt.Errorf("bots[%d].bot_token is required", i)
		}
		if bot.Provider != "claude" && bot.Provider != "codex" {
			return fmt.Errorf("bots[%d].provider must be claude or codex", i)
		}
	}

	return nil
}

func (b *BotConfig) normalize(index int) {
	if b.Name == "" {
		b.Name = fmt.Sprintf("%s-%d", normalizedProvider(b.Provider), index+1)
	}
	if b.Provider == "" {
		b.Provider = defaultProvider
	}
	if b.TimeoutSeconds == 0 {
		b.TimeoutSeconds = defaultTimeoutSeconds
	}
	if b.MaxConcurrent == 0 {
		b.MaxConcurrent = defaultMaxConcurrent
	}
	if b.SessionTTLMinutes == 0 {
		b.SessionTTLMinutes = defaultSessionTTL
	}
	if b.Provider == "codex" && b.Sandbox == "" {
		b.Sandbox = defaultCodexSandbox
	}
}

func normalizedProvider(provider string) string {
	if provider == "" {
		return defaultProvider
	}
	return provider
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseFloatOrExit(name, value string) float64 {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid %s: %v\n", name, err)
		os.Exit(1)
	}
	return f
}

func parseIntOrExit(name, value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid %s: %v\n", name, err)
		os.Exit(1)
	}
	return n
}

func configPathFromEnv(path string) string {
	if path != "" {
		return path
	}
	return os.Getenv("DISCORD_BOT_CONFIG")
}
