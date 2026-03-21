package textproc

import (
	"regexp"
	"strings"
)

// Processor matches backend TextProcessor behavior.
type Processor struct{}

func New() *Processor { return &Processor{} }

var multiNL = regexp.MustCompile(`\n{3,}`)

func (Processor) Preprocess(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = multiNL.ReplaceAllString(text, "\n\n")
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	text = strings.Join(lines, "\n")
	return strings.TrimSpace(text)
}

func (Processor) Split(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap < 0 {
		overlap = 0
	}
	if len(text) <= chunkSize {
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []string{text}
	}
	seps := []string{"。", "！", "？", ".\n", "!\n", "?\n", "\n\n", ". ", "! ", "? "}
	var chunks []string
	start := 0
	for start < len(text) {
		end := start + chunkSize
		if end < len(text) {
			window := text[start:end]
			for _, sep := range seps {
				last := strings.LastIndex(window, sep)
				if last != -1 && last > chunkSize*3/10 {
					end = start + last + len(sep)
					break
				}
			}
		}
		chunk := strings.TrimSpace(text[start:end])
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end >= len(text) {
			break
		}
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}
	return chunks
}
