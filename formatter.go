package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const maxChunkSize = 2000

func FormatResponse(result *ProviderResult) []string {
	text := result.Result
	meta := formatResultMeta(result)

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
	duration := result.Duration.Truncate(100 * time.Millisecond)
	if result.HasCost {
		return fmt.Sprintf("\n\n> Provider: %s | Cost: $%.4f | Duration: %s", result.Provider, result.CostUSD, duration)
	}
	return fmt.Sprintf("\n\n> Provider: %s | Duration: %s", result.Provider, duration)
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
