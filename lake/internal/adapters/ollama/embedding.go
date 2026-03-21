package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mirofish-offline/lake/internal/config"
)

// Embedder calls Ollama /api/embed (same as Python EmbeddingService).
type Embedder struct {
	httpClient *http.Client
	baseURL    string
	model      string
}

func NewEmbedder(cfg config.Config) *Embedder {
	return &Embedder{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    strings.TrimRight(strings.TrimSpace(cfg.EmbeddingBaseURL), "/"),
		model:      cfg.EmbeddingModel,
	}
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

// EmbedBatch returns one vector per non-empty text; empty strings get nil slices.
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float64, len(texts))
	var nonEmpty []string
	var idxMap []int
	for i, t := range texts {
		if strings.TrimSpace(t) == "" {
			out[i] = nil
			continue
		}
		nonEmpty = append(nonEmpty, strings.TrimSpace(t))
		idxMap = append(idxMap, i)
	}
	if len(nonEmpty) == 0 {
		return out, nil
	}
	url := e.baseURL + "/api/embed"
	body, err := json.Marshal(embedRequest{Model: e.model, Input: nonEmpty})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embed http %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var er embedResponse
	if err := json.Unmarshal(raw, &er); err != nil {
		return nil, fmt.Errorf("ollama embed decode: %w", err)
	}
	if len(er.Embeddings) != len(nonEmpty) {
		return nil, fmt.Errorf("ollama embed: want %d vectors got %d", len(nonEmpty), len(er.Embeddings))
	}
	for j, vec := range er.Embeddings {
		out[idxMap[j]] = vec
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
