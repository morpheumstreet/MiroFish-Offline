package domain

// SimulationState mirrors backend SimulationManager state.json (subset used by API).
type SimulationState struct {
	SimulationID    string   `json:"simulation_id"`
	ProjectID       string   `json:"project_id"`
	GraphID         string   `json:"graph_id"`
	EnableTwitter   bool     `json:"enable_twitter"`
	EnableReddit    bool     `json:"enable_reddit"`
	Status          string   `json:"status"`
	EntitiesCount   int      `json:"entities_count"`
	ProfilesCount   int      `json:"profiles_count"`
	EntityTypes     []string `json:"entity_types"`
	ConfigGenerated bool     `json:"config_generated"`
	ConfigReasoning string   `json:"config_reasoning"`
	CurrentRound    int      `json:"current_round"`
	TwitterStatus   string   `json:"twitter_status"`
	RedditStatus    string   `json:"reddit_status"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	Error           *string  `json:"error,omitempty"`
	// ProfilesGenerated helps /config/realtime stage detection (optional extension).
	ProfilesGenerated bool `json:"profiles_generated,omitempty"`
}

func (s *SimulationState) ToSimpleMap() map[string]any {
	if s == nil {
		return nil
	}
	return map[string]any{
		"simulation_id":    s.SimulationID,
		"project_id":       s.ProjectID,
		"graph_id":         s.GraphID,
		"status":           s.Status,
		"entities_count":   s.EntitiesCount,
		"profiles_count":   s.ProfilesCount,
		"entity_types":     s.EntityTypes,
		"config_generated": s.ConfigGenerated,
		"error":            s.Error,
	}
}
