package simrunner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func rawTimeNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// runStateData is persisted to run_state.json (Python-compatible keys).
type runStateData struct {
	SimulationID          string           `json:"simulation_id"`
	RunnerStatus          string           `json:"runner_status"`
	CurrentRound          int              `json:"current_round"`
	TotalRounds           int              `json:"total_rounds"`
	SimulatedHours        int              `json:"simulated_hours"`
	TotalSimulationHours  int              `json:"total_simulation_hours"`
	TwitterCurrentRound   int              `json:"twitter_current_round"`
	RedditCurrentRound    int              `json:"reddit_current_round"`
	TwitterSimulatedHours int              `json:"twitter_simulated_hours"`
	RedditSimulatedHours  int              `json:"reddit_simulated_hours"`
	TwitterRunning        bool             `json:"twitter_running"`
	RedditRunning         bool             `json:"reddit_running"`
	TwitterCompleted      bool             `json:"twitter_completed"`
	RedditCompleted       bool             `json:"reddit_completed"`
	TwitterActionsCount   int              `json:"twitter_actions_count"`
	RedditActionsCount    int              `json:"reddit_actions_count"`
	StartedAt             *string          `json:"started_at,omitempty"`
	UpdatedAt             string           `json:"updated_at"`
	CompletedAt           *string          `json:"completed_at,omitempty"`
	Error                 *string          `json:"error,omitempty"`
	ProcessPID            *int             `json:"process_pid,omitempty"`
	RecentActions         []map[string]any `json:"recent_actions"`
	ExpectTwitter         bool             `json:"-"`
	ExpectReddit          bool             `json:"-"`
}

func (s *runStateData) addAction(a Action) {
	if s.RecentActions == nil {
		s.RecentActions = []map[string]any{}
	}
	m := a.ToMap()
	s.RecentActions = append([]map[string]any{m}, s.RecentActions...)
	if len(s.RecentActions) > 50 {
		s.RecentActions = s.RecentActions[:50]
	}
	if a.Platform == "twitter" {
		s.TwitterActionsCount++
	} else {
		s.RedditActionsCount++
	}
	if a.RoundNum > s.CurrentRound {
		s.CurrentRound = a.RoundNum
	}
	s.UpdatedAt = rawTimeNow()
}

func (s *runStateData) allPlatformsDone() bool {
	if s.ExpectTwitter && !s.TwitterCompleted {
		return false
	}
	if s.ExpectReddit && !s.RedditCompleted {
		return false
	}
	return true
}

func loadRunState(simDir, simulationID string) *runStateData {
	raw, err := os.ReadFile(filepath.Join(simDir, "run_state.json"))
	if err != nil {
		return nil
	}
	var s runStateData
	if json.Unmarshal(raw, &s) != nil {
		return nil
	}
	if s.SimulationID == "" {
		s.SimulationID = simulationID
	}
	return &s
}

func saveRunStateDetail(simDir string, s *runStateData) error {
	s.UpdatedAt = rawTimeNow()
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(simDir, "run_state.json"), raw, 0o644)
}

func (s *runStateData) toSummaryMap() map[string]any {
	tp := 0.0
	if s.TotalRounds > 0 {
		tp = float64(s.CurrentRound) / float64(s.TotalRounds) * 100
	}
	return map[string]any{
		"simulation_id":           s.SimulationID,
		"runner_status":           s.RunnerStatus,
		"current_round":           s.CurrentRound,
		"total_rounds":            s.TotalRounds,
		"simulated_hours":         s.SimulatedHours,
		"total_simulation_hours":  s.TotalSimulationHours,
		"progress_percent":        tp,
		"twitter_current_round":   s.TwitterCurrentRound,
		"reddit_current_round":    s.RedditCurrentRound,
		"twitter_simulated_hours": s.TwitterSimulatedHours,
		"reddit_simulated_hours":  s.RedditSimulatedHours,
		"twitter_running":         s.TwitterRunning,
		"reddit_running":          s.RedditRunning,
		"twitter_completed":       s.TwitterCompleted,
		"reddit_completed":        s.RedditCompleted,
		"twitter_actions_count":   s.TwitterActionsCount,
		"reddit_actions_count":    s.RedditActionsCount,
		"total_actions_count":     s.TwitterActionsCount + s.RedditActionsCount,
		"started_at":              s.StartedAt,
		"updated_at":              s.UpdatedAt,
		"completed_at":            s.CompletedAt,
		"error":                   s.Error,
		"process_pid":             s.ProcessPID,
	}
}
