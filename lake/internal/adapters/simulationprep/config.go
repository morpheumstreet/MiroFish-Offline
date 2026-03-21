package simulationprep

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mirofish-offline/lake/internal/adapters/openai"
	"github.com/mirofish-offline/lake/internal/config"
	"github.com/mirofish-offline/lake/internal/ports"
)

// ConfigBuilder implements ports.SimulationConfigBuilder with deterministic defaults + light LLM for events.
type ConfigBuilder struct {
	llm *openai.Client
	cfg config.Config
}

func NewConfigBuilder(cfg config.Config, llm *openai.Client) *ConfigBuilder {
	return &ConfigBuilder{llm: llm, cfg: cfg}
}

var _ ports.SimulationConfigBuilder = (*ConfigBuilder)(nil)

func (c *ConfigBuilder) Build(
	ctx context.Context,
	simulationID, projectID, graphID, simulationRequirement, documentText string,
	entities []map[string]any,
	enableTwitter, enableReddit bool,
) (map[string]any, string, error) {
	n := len(entities)
	minutesPerRound := 30
	totalHours := 72
	if n > 80 {
		totalHours = 48
	}
	if n < 10 {
		totalHours = 24
	}

	agentConfigs := make([]map[string]any, 0, n)
	for i, ent := range entities {
		uu := fmt.Sprint(ent["uuid"])
		name := fmt.Sprint(ent["name"])
		etype := entityType(ent)
		agentConfigs = append(agentConfigs, map[string]any{
			"agent_id":           i + 1,
			"entity_uuid":        uu,
			"entity_name":        name,
			"entity_type":        etype,
			"activity_level":     0.5,
			"posts_per_hour":     0.8,
			"comments_per_hour":  1.5,
			"active_hours":       rangeInts(8, 22),
			"response_delay_min": 5,
			"response_delay_max": 45,
			"sentiment_bias":     0.0,
			"stance":             "neutral",
			"influence_weight":   1.0,
		})
	}

	reasoning := "Deterministic baseline from entity count; event topics from LLM when available."

	eventCfg := map[string]any{
		"initial_posts":       []any{},
		"scheduled_events":    []any{},
		"hot_topics":          []any{},
		"narrative_direction": truncateStr(simulationRequirement, 500),
	}
	if c.llm != nil && strings.TrimSpace(simulationRequirement) != "" {
		sys := `Return JSON with keys: hot_topics (array of 5-12 short strings), narrative_direction (string), initial_posts (array of max 3 objects with keys title, content_hint).`
		user := fmt.Sprintf("Simulation goal:\n%s\n\nDocument excerpt:\n%s",
			simulationRequirement, truncateStr(documentText, 6000))
		if m, err := c.llm.ChatJSON(ctx, sys, user, 0.5, 2048); err == nil {
			if ht, ok := m["hot_topics"].([]any); ok {
				eventCfg["hot_topics"] = ht
			}
			if nd, ok := m["narrative_direction"].(string); ok {
				eventCfg["narrative_direction"] = nd
			}
			if ip, ok := m["initial_posts"].([]any); ok {
				eventCfg["initial_posts"] = ip
			}
			reasoning += " Event layer from LLM."
		}
	}

	out := map[string]any{
		"simulation_id":          simulationID,
		"project_id":             projectID,
		"graph_id":               graphID,
		"simulation_requirement": simulationRequirement,
		"time_config": map[string]any{
			"total_simulation_hours":       totalHours,
			"minutes_per_round":            minutesPerRound,
			"agents_per_hour_min":          5,
			"agents_per_hour_max":          min(20, max(8, n/3)),
			"peak_hours":                   []any{19, 20, 21, 22},
			"peak_activity_multiplier":     1.5,
			"off_peak_hours":               []any{0, 1, 2, 3, 4, 5},
			"off_peak_activity_multiplier": 0.05,
			"morning_hours":                []any{6, 7, 8},
			"morning_activity_multiplier":  0.4,
			"work_hours":                   []any{9, 10, 11, 12, 13, 14, 15, 16, 17, 18},
			"work_activity_multiplier":     0.7,
		},
		"agent_configs": agentConfigs,
		"event_config":  eventCfg,
		"llm_model":     c.cfg.LLMModelName,
		"llm_base_url":  c.cfg.LLMBaseURL,
		"generated_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	_ = ctx

	if enableTwitter {
		out["twitter_config"] = map[string]any{
			"platform": "twitter", "recency_weight": 0.4, "popularity_weight": 0.3, "relevance_weight": 0.3,
			"viral_threshold": 10, "echo_chamber_strength": 0.5,
		}
	}
	if enableReddit {
		out["reddit_config"] = map[string]any{
			"platform": "reddit", "recency_weight": 0.4, "popularity_weight": 0.3, "relevance_weight": 0.3,
			"viral_threshold": 10, "echo_chamber_strength": 0.5,
		}
	}
	out["generation_reasoning"] = reasoning
	return out, reasoning, nil
}

func entityType(ent map[string]any) string {
	if ls, ok := ent["labels"].([]any); ok {
		for _, l := range ls {
			s := fmt.Sprint(l)
			if s != "Entity" && s != "Node" {
				return s
			}
		}
	}
	if ls, ok := ent["labels"].([]string); ok {
		for _, s := range ls {
			if s != "Entity" && s != "Node" {
				return s
			}
		}
	}
	return "Entity"
}

func rangeInts(a, b int) []any {
	var o []any
	for i := a; i <= b; i++ {
		o = append(o, i)
	}
	return o
}

func truncateStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
