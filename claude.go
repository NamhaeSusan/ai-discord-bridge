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
	SessionID string  `json:"session_id"`
	Result    string  `json:"result"`
	CostUSD   float64 `json:"cost_usd"`
}

func (p ClaudeProvider) Run(ctx context.Context, prompt string) (*ProviderResult, error) {
	return p.run(ctx, "", prompt)
}

func (p ClaudeProvider) Resume(ctx context.Context, sessionID, prompt string) (*ProviderResult, error) {
	return p.run(ctx, sessionID, prompt)
}

func (p ClaudeProvider) run(ctx context.Context, sessionID, prompt string) (*ProviderResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "claude", buildClaudeArgs(p.cfg, sessionID, prompt)...)
	if p.cfg.WorkingDir != "" {
		cmd.Dir = p.cfg.WorkingDir
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
		CostUSD:   parsed.CostUSD,
		HasCost:   true,
		Duration:  duration,
	}, nil
}

func buildClaudeArgs(cfg BotConfig, sessionID, prompt string) []string {
	args := []string{"-p", "--output-format", "json"}

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

	return append(args, prompt)
}
