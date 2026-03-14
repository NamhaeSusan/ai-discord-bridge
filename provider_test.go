package main

import (
	"reflect"
	"testing"
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
		"--resume", "session-1",
		"--model", "sonnet",
		"--max-budget-usd", "2.50",
		"--allowedTools", "Read",
		"--allowedTools", "Grep",
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

	got := buildCodexArgs(cfg, false, "", "ship it", "/tmp/out")
	want := []string{
		"exec",
		"-C", "/repo",
		"-m", "gpt-5-codex",
		"--sandbox", "danger-full-access",
		"--json",
		"-o", "/tmp/out",
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

	got := buildCodexArgs(cfg, true, "thread-1", "ship it", "/tmp/out")
	want := []string{
		"exec",
		"resume",
		"--json",
		"-o", "/tmp/out",
		"-m", "gpt-5-codex",
		"thread-1",
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
