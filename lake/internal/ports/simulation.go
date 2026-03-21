package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mirofish-offline/lake/internal/domain"
)

// SimulationRepository persists simulation workspace files (Python-compatible layout).
type SimulationRepository interface {
	SimulationsRoot() string
	EnsureSimulationDir(simulationID string) (absDir string, err error)
	Create(ctx context.Context, projectID, graphID string, enableTwitter, enableReddit bool) (*domain.SimulationState, error)
	Load(ctx context.Context, simulationID string) (*domain.SimulationState, error)
	Save(ctx context.Context, s *domain.SimulationState) error
	List(ctx context.Context, projectID string) ([]*domain.SimulationState, error)
	ListSimulationIDs(ctx context.Context) ([]string, error)
	ReadFile(ctx context.Context, simulationID, rel string) ([]byte, error)
	WriteFile(ctx context.Context, simulationID, rel string, data []byte) error
	StatFile(ctx context.Context, simulationID, rel string) (mod time.Time, ok bool)
	// PromotePreparingToReady sets status ready when artifacts exist (Flask auto-fix).
	PromotePreparingToReady(ctx context.Context, simulationID string) error
}

// ProfileBuilder turns filtered entities into OASIS profile records (reddit JSON / twitter CSV source data).
type ProfileBuilder interface {
	BuildProfiles(
		ctx context.Context,
		graphID string,
		entities []map[string]any,
		useLLM bool,
		parallel int,
		onProgress func(current, total int, msg string),
		appendRedditFile func(profiles []map[string]any) error,
	) ([]map[string]any, error)
	SaveRedditJSON(path string, profiles []map[string]any) error
	SaveTwitterCSV(path string, profiles []map[string]any) error
}

// SimulationConfigBuilder produces simulation_config.json payload + reasoning text.
type SimulationConfigBuilder interface {
	Build(
		ctx context.Context,
		simulationID, projectID, graphID, simulationRequirement, documentText string,
		entities []map[string]any,
		enableTwitter, enableReddit bool,
	) (config map[string]any, reasoning string, err error)
}

// PrepareProgress reports coarse pipeline progress for task updates.
type PrepareProgress struct {
	Stage    string
	Percent  int // 0-100 within stage
	Message  string
	Current  int
	Total    int
	ItemName string
}

// SimulationRuntime starts OASIS Python runners, tails logs, and filesystem IPC.
type SimulationRuntime interface {
	Start(ctx context.Context, in SimulationStartInput) (map[string]any, error)
	Stop(ctx context.Context, simulationID string) (map[string]any, error)
	CleanupLogs(ctx context.Context, simulationID string) map[string]any
	RunState(ctx context.Context, simulationID string) map[string]any
	RunStateDetail(ctx context.Context, simulationID string, platform string) map[string]any
	Actions(ctx context.Context, simulationID string, q ActionQuery) map[string]any
	Timeline(ctx context.Context, simulationID string, startRound int, endRound *int) map[string]any
	AgentStats(ctx context.Context, simulationID string) map[string]any
	Posts(ctx context.Context, simulationID, platform string, limit, offset int) map[string]any
	Comments(ctx context.Context, simulationID, platform, postID string, limit, offset int) map[string]any
	EnvStatus(ctx context.Context, simulationID string) map[string]any
	CloseEnv(ctx context.Context, simulationID string, timeoutSec int) map[string]any
	InterviewBatch(ctx context.Context, simulationID string, interviews []map[string]any, platform *string, timeout float64) map[string]any
}

type SimulationStartInput struct {
	SimulationID            string
	Platform                string // parallel | twitter | reddit
	MaxRounds               *int
	EnableGraphMemoryUpdate bool
	GraphID                 string
	Force                   bool
}

type ActionQuery struct {
	Limit    int
	Offset   int
	Platform string
	AgentID  *int
	RoundNum *int
}

// PrepareInfo is returned when checking whether artifacts exist (Flask _check_simulation_prepared).
type PrepareInfo struct {
	Reason          string   `json:"reason,omitempty"`
	MissingFiles    []string `json:"missing_files,omitempty"`
	ExistingFiles   []string `json:"existing_files,omitempty"`
	Status          string   `json:"status,omitempty"`
	EntitiesCount   int      `json:"entities_count,omitempty"`
	ProfilesCount   int      `json:"profiles_count,omitempty"`
	EntityTypes     []string `json:"entity_types,omitempty"`
	ConfigGenerated bool     `json:"config_generated,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
}

// CheckSimulationPrepared inspects filesystem like Flask _check_simulation_prepared.
func CheckSimulationPrepared(repo SimulationRepository, simulationID string) (prepared bool, info PrepareInfo) {
	required := []string{
		"state.json",
		"simulation_config.json",
		"reddit_profiles.json",
		"twitter_profiles.csv",
	}
	ctx := context.Background()
	var existing, missing []string
	for _, f := range required {
		if _, okFile := repo.StatFile(ctx, simulationID, f); okFile {
			existing = append(existing, f)
		} else {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return false, PrepareInfo{
			Reason:        "Missing required files",
			MissingFiles:  missing,
			ExistingFiles: existing,
		}
	}
	raw, err := repo.ReadFile(ctx, simulationID, "state.json")
	if err != nil {
		return false, PrepareInfo{Reason: "Failed to read state file: " + err.Error()}
	}
	var st map[string]any
	if err := json.Unmarshal(raw, &st); err != nil {
		return false, PrepareInfo{Reason: "Invalid state.json"}
	}
	status, _ := st["status"].(string)
	cfgGen, _ := st["config_generated"].(bool)
	preparedStatuses := map[string]struct{}{
		"ready": {}, "preparing": {}, "running": {}, "completed": {}, "stopped": {}, "failed": {},
	}
	if _, okSt := preparedStatuses[status]; !okSt || !cfgGen {
		return false, PrepareInfo{
			Reason:          "Status not in prepared list or config_generated is false",
			Status:          status,
			ConfigGenerated: cfgGen,
		}
	}
	outStatus := status
	if status == "preparing" {
		outStatus = "ready"
	}
	profilesCount := 0
	if pb, err := repo.ReadFile(ctx, simulationID, "reddit_profiles.json"); err == nil {
		var arr []any
		if json.Unmarshal(pb, &arr) == nil {
			profilesCount = len(arr)
		}
	}
	et := []string{}
	if v, ok := st["entity_types"].([]any); ok {
		for _, x := range v {
			if s, ok := x.(string); ok {
				et = append(et, s)
			}
		}
	}
	ec := 0
	switch x := st["entities_count"].(type) {
	case float64:
		ec = int(x)
	case int:
		ec = x
	}
	return true, PrepareInfo{
		Status:          outStatus,
		EntitiesCount:   ec,
		ProfilesCount:   profilesCount,
		EntityTypes:     et,
		ConfigGenerated: cfgGen,
		CreatedAt:       fmtAnyString(st["created_at"]),
		UpdatedAt:       fmtAnyString(st["updated_at"]),
		ExistingFiles:   existing,
	}
}

func fmtAnyString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}
