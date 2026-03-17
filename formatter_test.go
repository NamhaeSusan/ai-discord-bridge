package main

import (
	"strings"
	"testing"
	"time"
)

func TestFormatResultMetaWithAllFields(t *testing.T) {
	result := &ProviderResult{
		WorkingDir: "/tmp/testdir",
		HasCost:    true,
		CostUSD:    0.0312,
		Duration:   3200 * time.Millisecond,
	}
	got := formatResultMeta(result)

	if !strings.Contains(got, "📂") {
		t.Errorf("expected dir emoji, got %q", got)
	}
	if !strings.Contains(got, "💰 $0.0312") {
		t.Errorf("expected cost, got %q", got)
	}
	if !strings.Contains(got, "⏱ 3.2s") {
		t.Errorf("expected duration, got %q", got)
	}
}

func TestFormatResultMetaNoCost(t *testing.T) {
	result := &ProviderResult{
		WorkingDir: "/tmp/testdir",
		Duration:   1500 * time.Millisecond,
	}
	got := formatResultMeta(result)

	if strings.Contains(got, "💰") {
		t.Errorf("expected no cost section, got %q", got)
	}
	if !strings.Contains(got, "⏱ 1.5s") {
		t.Errorf("expected duration, got %q", got)
	}
}

func TestFormatResultMetaNoWorkingDir(t *testing.T) {
	result := &ProviderResult{
		HasCost:  true,
		CostUSD:  0.01,
		Duration: 2 * time.Second,
	}
	got := formatResultMeta(result)

	if strings.Contains(got, "📂") {
		t.Errorf("expected no dir section, got %q", got)
	}
	if !strings.Contains(got, "💰 $0.0100") {
		t.Errorf("expected cost, got %q", got)
	}
}

func TestFormatResultMetaDurationOnly(t *testing.T) {
	result := &ProviderResult{
		Duration: 500 * time.Millisecond,
	}
	got := formatResultMeta(result)

	if !strings.HasPrefix(got, "\n\n> ⏱") {
		t.Errorf("expected only duration, got %q", got)
	}
}

func TestShortenHome(t *testing.T) {
	tests := []struct {
		dir  string
		want string
	}{
		{"/tmp/foo", "/tmp/foo"},
		{"/usr/local", "/usr/local"},
	}
	for _, tt := range tests {
		got := shortenHome(tt.dir)
		if got != tt.want {
			t.Errorf("shortenHome(%q) = %q, want %q", tt.dir, got, tt.want)
		}
	}
}

func TestSplitIntoChunksShort(t *testing.T) {
	chunks := splitIntoChunks("hello world")
	if len(chunks) != 1 || chunks[0] != "hello world" {
		t.Fatalf("expected single chunk, got %v", chunks)
	}
}

func TestSplitIntoChunksLong(t *testing.T) {
	text := strings.Repeat("a", 3000)
	chunks := splitIntoChunks(text)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(c) > maxChunkSize+10 { // small tolerance for code block closure
			t.Fatalf("chunk too large: %d bytes", len(c))
		}
	}
}

func TestCountOpenCodeBlocks(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"no code", 0},
		{"```go\nfmt.Println()\n```", 0},
		{"```go\nfmt.Println()\n", 1},
	}
	for _, tt := range tests {
		got := countOpenCodeBlocks(tt.input)
		if got != tt.want {
			t.Errorf("countOpenCodeBlocks(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
