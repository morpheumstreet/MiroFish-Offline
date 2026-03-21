package simulationprep

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/mirofish-offline/lake/internal/adapters/openai"
	"github.com/mirofish-offline/lake/internal/config"
	"github.com/mirofish-offline/lake/internal/ports"
)

// ProfileBuilder implements ports.ProfileBuilder.
type ProfileBuilder struct {
	llm *openai.Client
	cfg config.Config
}

func NewProfileBuilder(cfg config.Config, llm *openai.Client) *ProfileBuilder {
	return &ProfileBuilder{llm: llm, cfg: cfg}
}

var _ ports.ProfileBuilder = (*ProfileBuilder)(nil)

func (p *ProfileBuilder) BuildProfiles(
	ctx context.Context,
	graphID string,
	entities []map[string]any,
	useLLM bool,
	parallel int,
	onProgress func(current, total int, msg string),
	appendRedditFile func(profiles []map[string]any) error,
) ([]map[string]any, error) {
	_ = graphID
	if parallel < 1 {
		parallel = 3
	}
	n := len(entities)
	out := make([]map[string]any, n)
	var mu sync.Mutex
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for i, ent := range entities {
		i, ent := i, ent
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			uid := i + 1
			var prof map[string]any
			var err error
			if useLLM {
				prof, err = p.profileLLM(ctx, uid, ent)
			} else {
				prof = p.profileHeuristic(uid, ent)
			}
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}
			mu.Lock()
			out[i] = prof
			done := 0
			for _, x := range out {
				if x != nil {
					done++
				}
			}
			mu.Unlock()
			if onProgress != nil {
				onProgress(done, n, fmt.Sprintf("profile user_id=%v", prof["user_id"]))
			}
			if appendRedditFile != nil {
				mu.Lock()
				partial := make([]map[string]any, 0, done)
				for _, x := range out {
					if x != nil {
						partial = append(partial, x)
					}
				}
				mu.Unlock()
				_ = appendRedditFile(partial)
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	// Compact in order
	final := make([]map[string]any, 0, n)
	for _, x := range out {
		if x != nil {
			final = append(final, x)
		}
	}
	return final, nil
}

func (p *ProfileBuilder) profileHeuristic(uid int, ent map[string]any) map[string]any {
	name := fmt.Sprint(ent["name"])
	summary := fmt.Sprint(ent["summary"])
	uu := fmt.Sprint(ent["uuid"])
	etype := ""
	if ls, ok := ent["labels"].([]any); ok && len(ls) > 0 {
		etype = fmt.Sprint(ls[0])
	}
	if ls, ok := ent["labels"].([]string); ok && len(ls) > 0 {
		etype = ls[0]
	}
	uname := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	if uname == "" {
		uname = fmt.Sprintf("agent_%d", uid)
	}
	return map[string]any{
		"user_id":            uid,
		"username":           uname,
		"name":               name,
		"bio":                truncate(summary, 280),
		"persona":            truncate(summary, 800),
		"karma":              500 + uid*10,
		"created_at":         "2024-01-01",
		"source_entity_uuid": uu,
		"source_entity_type": etype,
		"interested_topics":  []any{},
	}
}

func (p *ProfileBuilder) profileLLM(ctx context.Context, uid int, ent map[string]any) (map[string]any, error) {
	name := fmt.Sprint(ent["name"])
	summary := fmt.Sprint(ent["summary"])
	uu := fmt.Sprint(ent["uuid"])
	etype := ""
	if ls, ok := ent["labels"].([]any); ok && len(ls) > 0 {
		etype = fmt.Sprint(ls[0])
	}
	sys := `You output JSON only for one social simulation agent profile.
Fields: username (short snake_case), name (display), bio (<=280 chars), persona (rich paragraph), karma (int 100-5000), interested_topics (array of 3-8 short strings).
Optional: age, gender, mbti, country, profession (omit if unknown).`
	user := fmt.Sprintf("Entity type: %s\nName: %s\nSummary: %s\nGraph uuid: %s\nAssign user_id=%d in output.",
		etype, name, summary, uu, uid)
	m, err := p.llm.ChatJSON(ctx, sys, user, 0.4, 2048)
	if err != nil {
		return p.profileHeuristic(uid, ent), nil
	}
	m["user_id"] = uid
	if _, ok := m["username"]; !ok {
		m["username"] = strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	}
	m["source_entity_uuid"] = uu
	m["source_entity_type"] = etype
	return m, nil
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func (p *ProfileBuilder) SaveRedditJSON(path string, profiles []map[string]any) error {
	raw, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func (p *ProfileBuilder) SaveTwitterCSV(path string, profiles []map[string]any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	headers := []string{
		"user_id", "username", "name", "bio", "persona", "friend_count", "follower_count", "statuses_count", "created_at",
	}
	if err := w.Write(headers); err != nil {
		return err
	}
	for _, pr := range profiles {
		row := []string{
			fmt.Sprint(pr["user_id"]),
			fmt.Sprint(pr["username"]),
			fmt.Sprint(pr["name"]),
			fmt.Sprint(pr["bio"]),
			fmt.Sprint(pr["persona"]),
			"100", "150", "500",
			fmt.Sprint(pr["created_at"]),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
