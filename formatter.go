package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

const maxChunkSize = 2000

func FormatResponse(result *ProviderResult) []string {
	text := strings.TrimSpace(result.Result)
	meta := formatResultMeta(result)

	if text == "" {
		return []string{"No response text was returned.\n\n" + strings.TrimPrefix(meta, "\n\n")}
	}

	if len(text)+len(meta) <= maxChunkSize {
		return []string{text + meta}
	}

	chunks := splitIntoChunks(text)
	if len(chunks) == 0 {
		return []string{meta}
	}

	last := chunks[len(chunks)-1]
	if len(last)+len(meta) > maxChunkSize {
		chunks = append(chunks, meta)
	} else {
		chunks[len(chunks)-1] = last + meta
	}
	return chunks
}

func formatResultMeta(result *ProviderResult) string {
	var parts []string

	if result.WorkingDir != "" {
		dir := shortenHome(result.WorkingDir)
		git := gitInfo(result.WorkingDir)
		if git != "" {
			parts = append(parts, fmt.Sprintf("📂 %s (%s)", dir, git))
		} else {
			parts = append(parts, fmt.Sprintf("📂 %s", dir))
		}
	}

	if result.HasCost {
		parts = append(parts, fmt.Sprintf("💰 $%.4f", result.CostUSD))
	}

	duration := result.Duration.Truncate(100 * time.Millisecond)
	parts = append(parts, fmt.Sprintf("⏱ %s", duration))

	return "\n\n> " + strings.Join(parts, " | ")
}

func shortenHome(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return dir
	}
	if dir == home {
		return "~"
	}
	if strings.HasPrefix(dir, home+string(os.PathSeparator)) {
		return "~" + dir[len(home):]
	}
	return dir
}

func gitInfo(dir string) string {
	branch, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	info := strings.TrimSpace(string(branch))

	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 && lines[0] != "" {
			info += fmt.Sprintf(" ✎%d", len(lines))
		}
	}
	return info
}

func splitIntoChunks(text string) []string {
	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= maxChunkSize {
			chunks = append(chunks, remaining)
			break
		}

		splitAt := truncateUTF8(remaining, maxChunkSize)
		chunk := remaining[:splitAt]
		remaining = remaining[splitAt:]

		openBlocks := countOpenCodeBlocks(chunk)
		if openBlocks > 0 {
			if idx := strings.LastIndex(chunk, "\n"); idx > maxChunkSize/2 {
				remaining = chunk[idx:] + remaining
				chunk = chunk[:idx]
			}

			openBlocks = countOpenCodeBlocks(chunk)
			if openBlocks > 0 {
				chunk += "\n```"
				remaining = "```\n" + remaining
			}
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}

func truncateUTF8(s string, maxBytes int) int {
	if maxBytes >= len(s) {
		return len(s)
	}
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}
	return maxBytes
}

func countOpenCodeBlocks(s string) int {
	count := 0
	idx := 0
	for {
		pos := strings.Index(s[idx:], "```")
		if pos == -1 {
			break
		}
		count++
		idx += pos + 3
		for idx < len(s) && s[idx] != '\n' && s[idx] != '`' {
			idx++
		}
	}
	return count % 2
}
