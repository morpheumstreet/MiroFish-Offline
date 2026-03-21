package simrunner

import (
	"context"
	"fmt"

	"github.com/mirofish-offline/lake/internal/ports"
)

// Disabled implements ports.SimulationRuntime with errors (no Python / scripts).
type Disabled struct{}

func (Disabled) Start(ctx context.Context, in ports.SimulationStartInput) (map[string]any, error) {
	_ = ctx
	_ = in
	return nil, fmt.Errorf("simulation runtime disabled: set LAKE_BACKEND_ROOT to backend/ or LAKE_SCRIPTS_DIR")
}

func (Disabled) Stop(ctx context.Context, simulationID string) (map[string]any, error) {
	_ = ctx
	return map[string]any{"simulation_id": simulationID, "runner_status": "stopped"}, nil
}

func (Disabled) CleanupLogs(ctx context.Context, simulationID string) map[string]any {
	_ = ctx
	_ = simulationID
	return map[string]any{"success": true}
}

func (Disabled) RunState(ctx context.Context, simulationID string) map[string]any {
	_ = ctx
	return map[string]any{"simulation_id": simulationID, "runner_status": "idle"}
}

func (Disabled) RunStateDetail(ctx context.Context, simulationID string, platform string) map[string]any {
	_ = ctx
	_ = platform
	return map[string]any{
		"simulation_id": simulationID, "runner_status": "idle",
		"all_actions": []any{}, "twitter_actions": []any{}, "reddit_actions": []any{},
	}
}

func (Disabled) Actions(ctx context.Context, simulationID string, q ports.ActionQuery) map[string]any {
	_ = ctx
	_ = simulationID
	_ = q
	return map[string]any{"count": 0, "actions": []any{}}
}

func (Disabled) Timeline(ctx context.Context, simulationID string, startRound int, endRound *int) map[string]any {
	_ = ctx
	_ = simulationID
	_ = startRound
	_ = endRound
	return map[string]any{"rounds_count": 0, "timeline": []any{}}
}

func (Disabled) AgentStats(ctx context.Context, simulationID string) map[string]any {
	_ = ctx
	_ = simulationID
	return map[string]any{"agents_count": 0, "stats": []any{}}
}

func (Disabled) Posts(ctx context.Context, simulationID, platform string, limit, offset int) map[string]any {
	_ = ctx
	_ = simulationID
	_ = platform
	_ = limit
	_ = offset
	return map[string]any{"count": 0, "posts": []any{}}
}

func (Disabled) EnvStatus(ctx context.Context, simulationID string) map[string]any {
	_ = ctx
	return map[string]any{
		"simulation_id": simulationID, "env_alive": false,
		"twitter_available": false, "reddit_available": false,
		"message": "Environment not running or closed",
	}
}

func (Disabled) CloseEnv(ctx context.Context, simulationID string, timeoutSec int) map[string]any {
	_ = ctx
	_ = simulationID
	_ = timeoutSec
	return map[string]any{"success": true, "message": "Environment already closed"}
}

func (Disabled) InterviewBatch(ctx context.Context, simulationID string, interviews []map[string]any, platform *string, timeout float64) map[string]any {
	_ = ctx
	_ = simulationID
	_ = interviews
	_ = platform
	_ = timeout
	return map[string]any{"success": false, "error": "simulation runtime disabled"}
}
