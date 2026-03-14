package main

import (
	"fmt"
	"strings"
)

const maxChunkSize = 2000

func FormatResponse(result *ClaudeResult) []string {
	text := result.Result
	costInfo := fmt.Sprintf("\n\n> Cost: $%.4f | Duration: %s", result.CostUSD, result.Duration.Truncate(100*1e6))

	if len(text)+len(costInfo) <= maxChunkSize {
		return []string{text + costInfo}
	}

	chunks := splitIntoChunks(text)
	if len(chunks) == 0 {
		return []string{costInfo}
	}

	// Ensure last chunk has room for cost info
	last := chunks[len(chunks)-1]
	if len(last)+len(costInfo) > maxChunkSize {
		chunks = append(chunks, costInfo)
	} else {
		chunks[len(chunks)-1] = last + costInfo
	}
	return chunks
}

func splitIntoChunks(text string) []string {
	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= maxChunkSize {
			chunks = append(chunks, remaining)
			break
		}

		chunk := remaining[:maxChunkSize]
		remaining = remaining[maxChunkSize:]

		// Handle code block splitting
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
