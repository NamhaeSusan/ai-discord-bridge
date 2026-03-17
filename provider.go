package main

import (
	"context"
	"fmt"
	"time"
)

type Provider interface {
	Run(ctx context.Context, prompt, workingDir string) (*ProviderResult, error)
	Resume(ctx context.Context, sessionID, prompt, workingDir string) (*ProviderResult, error)
}

type ProviderResult struct {
	Provider   string
	SessionID  string
	Result     string
	Duration   time.Duration
	WorkingDir string
}

func NewProvider(cfg BotConfig) (Provider, error) {
	switch cfg.Provider {
	case "claude":
		return ClaudeProvider{cfg: cfg}, nil
	case "codex":
		return CodexProvider{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}
