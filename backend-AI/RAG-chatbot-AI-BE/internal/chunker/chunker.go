package chunker

import (
	"regexp"
	"strings"
)

var wordRegex = regexp.MustCompile(`\w+|[.,!?;:]`)

func ChunkText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap < 0 {
		overlap = 0
	}

	tokens := wordRegex.FindAllString(text, -1)
	if len(tokens) == 0 {
		return []string{}
	}
	step := chunkSize - overlap
	if step <= 0 {
		step = chunkSize / 2
	}

	chunks := []string{}
	for start := 0; start < len(tokens); start += step {
		end := start + chunkSize
		if end > len(tokens) {
			end = len(tokens)
		}
		chunk := strings.Join(tokens[start:end], " ")
		chunks = append(chunks, chunk)
		if end == len(tokens) {
			break
		}
	}
	return chunks
}
