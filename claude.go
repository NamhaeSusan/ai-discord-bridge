package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type ClaudeResult struct {
	SessionID string
	Result    string
	CostUSD   float64
	Duration  time.Duration
}

type claudeOutput struct {
	SessionID string  `json:"session_id"`
	Result    string  `json:"result"`
	CostUSD   float64 `json:"cost_usd"`
}

func RunClaude(ctx context.Context, cfg *Config, prompt string) (*ClaudeResult, error) {
	return runClaude(ctx, cfg, "", prompt)
}

func ResumeClaude(ctx context.Context, cfg *Config, sessionID, prompt string) (*ClaudeResult, error) {
	return runClaude(ctx, cfg, sessionID, prompt)
}

func runClaude(ctx context.Context, cfg *Config, sessionID, prompt string) (*ClaudeResult, error) {
	args := buildArgs(cfg, sessionID, prompt)

	timeout := time.Duration(cfg.ClaudeTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	cmd := exec.CommandContext(ctx, "claude", args...)
	if cfg.ClaudeWorkingDir != "" {
		cmd.Dir = cfg.ClaudeWorkingDir
	}

	out, err := cmd.Output()
	duration := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("claude command failed: %w", err)
	}

	var parsed claudeOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse claude output: %w", err)
	}

	return &ClaudeResult{
		SessionID: parsed.SessionID,
		Result:    parsed.Result,
		CostUSD:   parsed.CostUSD,
		Duration:  duration,
	}, nil
}

func buildArgs(cfg *Config, sessionID, prompt string) []string {
	args := []string{"-p", "--output-format", "json"}

	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}
	if cfg.ClaudeModel != "" {
		args = append(args, "--model", cfg.ClaudeModel)
	}
	if cfg.ClaudeMaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", cfg.ClaudeMaxBudgetUSD))
	}
	for _, tool := range cfg.ClaudeAllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	args = append(args, prompt)
	return args
}
