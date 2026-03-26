package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type ClaudeProvider struct {
	cfg BotConfig
}

type claudeOutput struct {
	SessionID string `json:"session_id"`
	Result    string `json:"result"`
}

func (p ClaudeProvider) Run(ctx context.Context, prompt, workingDir string) (*ProviderResult, error) {
	return p.run(ctx, "", prompt, workingDir)
}

func (p ClaudeProvider) Resume(ctx context.Context, sessionID, prompt, workingDir string) (*ProviderResult, error) {
	return p.run(ctx, sessionID, prompt, workingDir)
}

func (p ClaudeProvider) run(ctx context.Context, sessionID, prompt, workingDir string) (*ProviderResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "claude", buildClaudeArgs(p.cfg, sessionID, prompt)...)
	if dir := effectiveWorkingDir(p.cfg, workingDir); dir != "" {
		cmd.Dir = dir
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	duration := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("claude command failed: %w\nstderr: %s", err, stderr.String())
	}

	var parsed claudeOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse claude output: %w", err)
	}

	return &ProviderResult{
		Provider:  "claude",
		SessionID: parsed.SessionID,
		Result:    parsed.Result,
		Duration:  duration,
	}, nil
}

func effectiveWorkingDir(cfg BotConfig, override string) string {
	if override != "" {
		return override
	}
	return cfg.WorkingDir
}

func buildClaudeArgs(cfg BotConfig, sessionID, prompt string) []string {
	args := []string{"-p", "--output-format", "json", "--dangerously-skip-permissions"}

	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.MaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", cfg.MaxBudgetUSD))
	}
	for _, tool := range cfg.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	args = append(args, "--")
	return append(args, prompt)
}
