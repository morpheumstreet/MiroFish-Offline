package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mirofish-offline/lake/internal/config"
)

// Client calls OpenAI-compatible chat completions (Ollama, etc.).
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
	numCtx     int
	ollama     bool
}

func New(cfg config.Config) *Client {
	base := strings.TrimRight(strings.TrimSpace(cfg.LLMBaseURL), "/")
	ollama := strings.Contains(base, "11434")
	return &Client{
		httpClient: &http.Client{Timeout: time.Duration(cfg.LLMTimeout) * time.Second},
		baseURL:    base,
		apiKey:     cfg.LLMAPIKey,
		model:      cfg.LLMModelName,
		numCtx:     cfg.OllamaNumCtx,
		ollama:     ollama,
	}
}

// ChatMessage is one OpenAI-format message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string         `json:"model"`
	Messages       []ChatMessage  `json:"messages"`
	Temperature    float64        `json:"temperature"`
	MaxTokens      int            `json:"max_tokens"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
	Options        map[string]any `json:"options,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

var (
	reThinking   = regexp.MustCompile(`(?s)<think>[\s\S]*?</think>`)
	reFenceStart = regexp.MustCompile(`(?is)^\s*` + "```" + `(?:json)?\s*\n?`)
	reFenceEnd   = regexp.MustCompile(`(?is)\n?` + "```" + `\s*$`)
)

// ChatJSON returns parsed JSON from the model (response_format json_object when supported).
func (c *Client) ChatJSON(ctx context.Context, system, user string, temperature float64, maxTokens int) (map[string]any, error) {
	return c.ChatJSONMessages(ctx, []ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, temperature, maxTokens)
}

// ChatJSONMessages is like ChatJSON but accepts a custom message list (e.g. NER prompts).
func (c *Client) ChatJSONMessages(ctx context.Context, messages []ChatMessage, temperature float64, maxTokens int) (map[string]any, error) {
	url := c.baseURL + "/chat/completions"
	body := chatRequest{
		Model:          c.model,
		Temperature:    temperature,
		MaxTokens:      maxTokens,
		Messages:       messages,
		ResponseFormat: map[string]any{"type": "json_object"},
	}
	if c.ollama && c.numCtx > 0 {
		body.Options = map[string]any{"num_ctx": c.numCtx}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llm http %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}
	var cr chatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return nil, fmt.Errorf("decode completions: %w", err)
	}
	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("llm: empty choices")
	}
	content := strings.TrimSpace(cr.Choices[0].Message.Content)
	content = reThinking.ReplaceAllString(content, "")
	content = strings.TrimSpace(content)
	content = reFenceStart.ReplaceAllString(content, "")
	content = reFenceEnd.ReplaceAllString(content, "")
	content = strings.TrimSpace(content)
	var out map[string]any
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("invalid json from llm: %w (snippet: %q)", err, truncate(content, 200))
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
