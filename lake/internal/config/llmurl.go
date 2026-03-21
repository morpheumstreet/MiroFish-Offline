package config

import "strings"

// NormalizeOpenAIv1BaseURL trims whitespace/trailing slashes and ensures the OpenAI-compatible
// chat base ends with /v1 (Lake's client POSTs to {base}/chat/completions).
// If the value already ends with /v1, it is left unchanged.
func NormalizeOpenAIv1BaseURL(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimRight(s, "/")
	if s == "" {
		return ""
	}
	if strings.HasSuffix(s, "/v1") {
		return s
	}
	return s + "/v1"
}
