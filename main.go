package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/BurntSushi/toml"
)

type Config struct {
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

func loadConfig() *Config {
	configPath := flag.String("config", "", "path to TOML config file")
	flag.Parse()

	cfg := &Config{
		ClaudeTimeoutSeconds: 300,
		MaxConcurrent:        5,
		SessionTTLMinutes:    60,
	}

	// Load from TOML file
	path := *configPath
	if path == "" {
		path = os.Getenv("DISCORD_BOT_CONFIG")
	}
	if path != "" {
		if _, err := toml.DecodeFile(path, cfg); err != nil {
			log.Fatalf("failed to load config file %s: %v", path, err)
		}
	}

	// Env vars override TOML values
	if v := os.Getenv("DISCORD_BOT_TOKEN"); v != "" {
		cfg.BotToken = v
	}
	if v := os.Getenv("DISCORD_ALLOWED_CHANNELS"); v != "" {
		cfg.AllowedChannels = strings.Split(v, ",")
	}
	if v := os.Getenv("DISCORD_ALLOWED_USERS"); v != "" {
		cfg.AllowedUsers = strings.Split(v, ",")
	}
	if v := os.Getenv("CLAUDE_WORKING_DIR"); v != "" {
		cfg.ClaudeWorkingDir = v
	}
	if v := os.Getenv("CLAUDE_MODEL"); v != "" {
		cfg.ClaudeModel = v
	}
	if v := os.Getenv("CLAUDE_ALLOWED_TOOLS"); v != "" {
		cfg.ClaudeAllowedTools = strings.Split(v, ",")
	}
	if v := os.Getenv("CLAUDE_MAX_BUDGET_USD"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Fatalf("invalid CLAUDE_MAX_BUDGET_USD: %v", err)
		}
		cfg.ClaudeMaxBudgetUSD = f
	}
	if v := os.Getenv("CLAUDE_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("invalid CLAUDE_TIMEOUT_SECONDS: %v", err)
		}
		cfg.ClaudeTimeoutSeconds = n
	}

	return cfg
}

func main() {
	cfg := loadConfig()

	if cfg.BotToken == "" {
		log.Fatal("bot_token is required (config file or DISCORD_BOT_TOKEN env)")
	}

	if _, err := exec.LookPath("claude"); err != nil {
		log.Fatal("claude CLI not found in PATH: ", err)
	}

	bot, err := NewBot(cfg)
	if err != nil {
		log.Fatal("failed to create bot: ", err)
	}

	if err := bot.Open(); err != nil {
		log.Fatal("failed to open bot session: ", err)
	}
	defer bot.Close()

	log.Println("Discord bot is running. Press Ctrl+C to stop.")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("Shutting down...")
}
