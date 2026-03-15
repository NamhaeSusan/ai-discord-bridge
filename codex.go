package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type CodexProvider struct {
	cfg BotConfig
}

type codexEvent struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Item     struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
}

func (p CodexProvider) Run(ctx context.Context, prompt, workingDir string) (*ProviderResult, error) {
	return p.run(ctx, false, "", prompt, workingDir)
}

func (p CodexProvider) Resume(ctx context.Context, sessionID, prompt, workingDir string) (*ProviderResult, error) {
	return p.run(ctx, true, sessionID, prompt, workingDir)
}

func (p CodexProvider) run(ctx context.Context, resume bool, sessionID, prompt, workingDir string) (*ProviderResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	outputPath, err := createOutputFile()
	if err != nil {
		return nil, err
	}
	defer os.Remove(outputPath)

	args := buildCodexArgs(p.cfg, resume, sessionID, prompt, outputPath, workingDir)
	cmd := exec.CommandContext(ctx, "codex", args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codex command failed: %w\nstderr: %s", err, stderr.String())
	}

	session, fallbackText, err := parseCodexJSONL(stdout.Bytes())
	if err != nil {
		return nil, err
	}

	result, err := readLastMessage(outputPath)
	if err != nil {
		return nil, err
	}
	if result == "" {
		result = fallbackText
	}

	return &ProviderResult{
		Provider:  "codex",
		SessionID: session,
		Result:    result,
		Duration:  time.Since(start),
	}, nil
}

func buildCodexArgs(cfg BotConfig, resume bool, sessionID, prompt, outputPath, workingDir string) []string {
	args := []string{"exec"}
	if dir := effectiveWorkingDir(cfg, workingDir); dir != "" {
		args = append(args, "-C", dir)
	}
	if resume {
		args = append(args, "resume", "--json", "-o", outputPath)
		if cfg.Model != "" {
			args = append(args, "-m", cfg.Model)
		}
		args = append(args, sessionID)
		args = append(args, "--")
		return append(args, prompt)
	}

	if cfg.Model != "" {
		args = append(args, "-m", cfg.Model)
	}
	if cfg.Sandbox != "" {
		args = append(args, "--sandbox", cfg.Sandbox)
	}

	args = append(args, "--json", "-o", outputPath, "--")
	return append(args, prompt)
}

func parseCodexJSONL(data []byte) (string, string, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var threadID string
	var lastMessage string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event codexEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return "", "", fmt.Errorf("parse codex event: %w", err)
		}

		if event.Type == "thread.started" && event.ThreadID != "" {
			threadID = event.ThreadID
		}
		if event.Item.Type == "agent_message" && event.Item.Text != "" {
			lastMessage = event.Item.Text
		}
	}

	return threadID, lastMessage, nil
}

func createOutputFile() (string, error) {
	f, err := os.CreateTemp("", "codex-last-message-*")
	if err != nil {
		return "", fmt.Errorf("create codex output file: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close codex output file: %w", err)
	}
	return name, nil
}

func readLastMessage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read codex output file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
