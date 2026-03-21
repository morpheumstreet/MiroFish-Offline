package simrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/mirofish-offline/lake/internal/config"
	"github.com/mirofish-offline/lake/internal/ports"
)

// Runtime runs OASIS Python scripts and tails action logs.
type Runtime struct {
	root    string
	scripts string
	python  string
	mu      sync.Mutex
	procs   map[string]*exec.Cmd
}

func NewRuntime(cfg config.Config) (*Runtime, error) {
	root := cfg.SimulationsDir()
	scripts := cfg.ScriptsDir()
	if scripts == "" {
		return nil, fmt.Errorf("scripts directory not found: set LAKE_BACKEND_ROOT or LAKE_SCRIPTS_DIR")
	}
	py, err := exec.LookPath("python3")
	if err != nil {
		py, err = exec.LookPath("python")
	}
	if err != nil {
		return nil, fmt.Errorf("python not found in PATH")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Runtime{
		root:    root,
		scripts: scripts,
		python:  py,
		procs:   map[string]*exec.Cmd{},
	}, nil
}

var _ ports.SimulationRuntime = (*Runtime)(nil)

func (r *Runtime) simDir(id string) string { return filepath.Join(r.root, id) }

func (r *Runtime) Start(ctx context.Context, in ports.SimulationStartInput) (map[string]any, error) {
	_ = ctx
	simDir := r.simDir(in.SimulationID)
	cfgPath := filepath.Join(simDir, "simulation_config.json")
	if _, err := os.Stat(cfgPath); err != nil {
		return nil, fmt.Errorf("simulation config does not exist, call /prepare first")
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var cfgFile map[string]any
	if err := json.Unmarshal(raw, &cfgFile); err != nil {
		return nil, err
	}
	tc, _ := cfgFile["time_config"].(map[string]any)
	totalHours := intFromAny(tc["total_simulation_hours"])
	if totalHours <= 0 {
		totalHours = 72
	}
	minPerRound := intFromAny(tc["minutes_per_round"])
	if minPerRound <= 0 {
		minPerRound = 30
	}
	totalRounds := int(float64(totalHours*60) / float64(minPerRound))
	if in.MaxRounds != nil && *in.MaxRounds > 0 && *in.MaxRounds < totalRounds {
		totalRounds = *in.MaxRounds
	}

	r.mu.Lock()
	existing, exists := r.procs[in.SimulationID]
	r.mu.Unlock()
	if exists && existing != nil && existing.Process != nil && existing.ProcessState == nil {
		return nil, fmt.Errorf("simulation already running: %s", in.SimulationID)
	}

	if in.Force {
		r.CleanupLogs(context.Background(), in.SimulationID)
	}

	st := loadRunState(r.simDir(in.SimulationID), in.SimulationID)
	if st != nil && (st.RunnerStatus == "running" || st.RunnerStatus == "starting") {
		if pid := st.ProcessPID; pid != nil && procAlive(*pid) {
			return nil, fmt.Errorf("simulation already running: %s", in.SimulationID)
		}
	}

	platform := in.Platform
	if platform == "" {
		platform = "parallel"
	}
	script := "run_parallel_simulation.py"
	expectTw, expectRd := true, true
	switch platform {
	case "twitter":
		script = "run_twitter_simulation.py"
		expectTw, expectRd = true, false
	case "reddit":
		script = "run_reddit_simulation.py"
		expectTw, expectRd = false, true
	}
	scriptPath := filepath.Join(r.scripts, script)
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("script does not exist: %s", scriptPath)
	}

	logPath := filepath.Join(simDir, "simulation.log")
	logF, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	//nolint:gosec // OASIS entrypoint from configured backend/scripts path.
	cmd := exec.Command(r.python, scriptPath, "--config", cfgPath)
	if in.MaxRounds != nil && *in.MaxRounds > 0 {
		cmd.Args = append(cmd.Args, "--max-rounds", fmt.Sprint(*in.MaxRounds))
	}
	cmd.Dir = simDir
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.Env = append(os.Environ(), "PYTHONUTF8=1", "PYTHONIOENCODING=utf-8")
	setProcGroup(cmd)
	if err := cmd.Start(); err != nil {
		_ = logF.Close()
		return nil, err
	}
	pid := cmd.Process.Pid
	r.mu.Lock()
	r.procs[in.SimulationID] = cmd
	r.mu.Unlock()

	now := rawTimeNow()
	state := &runStateData{
		SimulationID:         in.SimulationID,
		RunnerStatus:         "running",
		CurrentRound:         0,
		TotalRounds:          totalRounds,
		TotalSimulationHours: totalHours,
		TwitterRunning:       expectTw,
		RedditRunning:        expectRd,
		ExpectTwitter:        expectTw,
		ExpectReddit:         expectRd,
		StartedAt:            &now,
		UpdatedAt:            now,
		ProcessPID:           &pid,
	}
	_ = saveRunStateDetail(simDir, state)
	_ = logF.Close()

	go r.monitor(in.SimulationID, simDir, cmd, state)

	out := state.toSummaryMap()
	out["max_rounds_applied"] = in.MaxRounds
	out["graph_memory_update_enabled"] = in.EnableGraphMemoryUpdate
	out["force_restarted"] = in.Force
	if in.EnableGraphMemoryUpdate {
		out["graph_id"] = in.GraphID
	}
	return out, nil
}

func (r *Runtime) monitor(simulationID, simDir string, cmd *exec.Cmd, state *runStateData) {
	twPath := filepath.Join(simDir, "twitter", "actions.jsonl")
	rdPath := filepath.Join(simDir, "reddit", "actions.jsonl")
	var twOff, rdOff int64
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case err := <-done:
			var ntw, nrd int64
			ntw, _ = ReadActionsFromJSONL(twPath, twOff, "twitter", state)
			nrd, _ = ReadActionsFromJSONL(rdPath, rdOff, "reddit", state)
			twOff, rdOff = ntw, nrd
			state.TwitterRunning = false
			state.RedditRunning = false
			if err != nil {
				if state.RunnerStatus != "completed" {
					state.RunnerStatus = "failed"
					msg := err.Error()
					state.Error = &msg
				}
			} else {
				if state.RunnerStatus != "completed" {
					state.RunnerStatus = "completed"
				}
				if state.CompletedAt == nil {
					now := rawTimeNow()
					state.CompletedAt = &now
				}
			}
			_ = saveRunStateDetail(simDir, state)
			r.mu.Lock()
			delete(r.procs, simulationID)
			r.mu.Unlock()
			return
		case <-tick.C:
			var ntw, nrd int64
			ntw, _ = ReadActionsFromJSONL(twPath, twOff, "twitter", state)
			nrd, _ = ReadActionsFromJSONL(rdPath, rdOff, "reddit", state)
			twOff, rdOff = ntw, nrd
			_ = saveRunStateDetail(simDir, state)
		}
	}
}

func (r *Runtime) Stop(ctx context.Context, simulationID string) (map[string]any, error) {
	_ = ctx
	r.mu.Lock()
	cmd := r.procs[simulationID]
	r.mu.Unlock()
	st := loadRunState(r.simDir(simulationID), simulationID)
	if st == nil {
		return nil, fmt.Errorf("simulation does not exist: %s", simulationID)
	}
	if st.RunnerStatus != "running" && st.RunnerStatus != "starting" && st.RunnerStatus != "paused" {
		// allow stop if marked running in file but proc missing
		if st.RunnerStatus != "stopping" && cmd == nil && (st.ProcessPID == nil || !procAlive(*st.ProcessPID)) {
			return nil, fmt.Errorf("simulation not running: %s, status=%s", simulationID, st.RunnerStatus)
		}
	}
	st.RunnerStatus = "stopping"
	_ = saveRunStateDetail(r.simDir(simulationID), st)

	if cmd != nil && cmd.Process != nil {
		_ = killProcessGroup(cmd.Process.Pid)
		_, _ = cmd.Process.Wait()
	}
	if st.ProcessPID != nil && cmd == nil {
		_ = killProcessGroup(*st.ProcessPID)
	}

	r.mu.Lock()
	delete(r.procs, simulationID)
	r.mu.Unlock()

	st.RunnerStatus = "stopped"
	st.TwitterRunning = false
	st.RedditRunning = false
	now := rawTimeNow()
	st.CompletedAt = &now
	_ = saveRunStateDetail(r.simDir(simulationID), st)
	return st.toSummaryMap(), nil
}

func killProcessGroup(pid int) error {
	return unixKillGroup(pid)
}

func (r *Runtime) CleanupLogs(ctx context.Context, simulationID string) map[string]any {
	_ = ctx
	simDir := r.simDir(simulationID)
	paths := []string{
		"run_state.json",
		"simulation.log",
		"stdout.log",
		"stderr.log",
		"env_status.json",
		"twitter/actions.jsonl",
		"reddit/actions.jsonl",
		"actions.jsonl",
		"twitter_simulation.db",
		"reddit_simulation.db",
	}
	var errs []string
	for _, p := range paths {
		if err := os.Remove(filepath.Join(simDir, p)); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err.Error())
		}
	}
	return map[string]any{"success": len(errs) == 0, "errors": errs}
}

func (r *Runtime) RunState(ctx context.Context, simulationID string) map[string]any {
	_ = ctx
	st := loadRunState(r.simDir(simulationID), simulationID)
	if st == nil {
		return map[string]any{
			"simulation_id":         simulationID,
			"runner_status":         "idle",
			"current_round":         0,
			"total_rounds":          0,
			"progress_percent":      0,
			"twitter_actions_count": 0,
			"reddit_actions_count":  0,
			"total_actions_count":   0,
		}
	}
	return st.toSummaryMap()
}

func (r *Runtime) RunStateDetail(ctx context.Context, simulationID string, platform string) map[string]any {
	_ = ctx
	st := loadRunState(r.simDir(simulationID), simulationID)
	simDir := r.simDir(simulationID)
	if st == nil {
		return map[string]any{
			"simulation_id":   simulationID,
			"runner_status":   "idle",
			"all_actions":     []any{},
			"twitter_actions": []any{},
			"reddit_actions":  []any{},
		}
	}
	all := collectAllActions(simDir, platform, nil, nil)
	sort.Slice(all, func(i, j int) bool { return all[i].Timestamp > all[j].Timestamp })
	tw := collectAllActions(simDir, "twitter", nil, nil)
	sort.Slice(tw, func(i, j int) bool { return tw[i].Timestamp > tw[j].Timestamp })
	rd := collectAllActions(simDir, "reddit", nil, nil)
	sort.Slice(rd, func(i, j int) bool { return rd[i].Timestamp > rd[j].Timestamp })

	var recent []Action
	if st.CurrentRound > 0 {
		rn := st.CurrentRound
		recent = collectAllActions(simDir, platform, nil, &rn)
		sort.Slice(recent, func(i, j int) bool { return recent[i].Timestamp > recent[j].Timestamp })
	}
	out := st.toSummaryMap()
	out["all_actions"] = actionsToMaps(all)
	out["twitter_actions"] = actionsToMaps(tw)
	out["reddit_actions"] = actionsToMaps(rd)
	out["rounds_count"] = 0
	out["recent_actions"] = actionsToMaps(recent)
	return out
}

func actionsToMaps(aa []Action) []map[string]any {
	out := make([]map[string]any, len(aa))
	for i := range aa {
		out[i] = aa[i].ToMap()
	}
	return out
}

func (r *Runtime) Actions(ctx context.Context, simulationID string, q ports.ActionQuery) map[string]any {
	_ = ctx
	acts := collectAllActions(r.simDir(simulationID), q.Platform, q.AgentID, q.RoundNum)
	sort.Slice(acts, func(i, j int) bool { return acts[i].Timestamp > acts[j].Timestamp })
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Limit <= 0 {
		q.Limit = 100
	}
	end := q.Offset + q.Limit
	if end > len(acts) {
		end = len(acts)
	}
	slice := acts[q.Offset:end]
	return map[string]any{"count": len(slice), "actions": actionsToMaps(slice)}
}

func (r *Runtime) Timeline(ctx context.Context, simulationID string, startRound int, endRound *int) map[string]any {
	acts := collectAllActions(r.simDir(simulationID), "", nil, nil)
	rounds := map[int]map[string]any{}
	for _, a := range acts {
		if a.RoundNum < startRound {
			continue
		}
		if endRound != nil && a.RoundNum > *endRound {
			continue
		}
		rn := a.RoundNum
		if rounds[rn] == nil {
			rounds[rn] = map[string]any{
				"round_num": rn, "twitter_actions": 0, "reddit_actions": 0,
				"active_agents": map[int]struct{}{}, "action_types": map[string]int{},
				"first_action_time": a.Timestamp, "last_action_time": a.Timestamp,
			}
		}
		r := rounds[rn]
		if a.Platform == "twitter" {
			r["twitter_actions"] = r["twitter_actions"].(int) + 1
		} else {
			r["reddit_actions"] = r["reddit_actions"].(int) + 1
		}
		agents := r["active_agents"].(map[int]struct{})
		agents[a.AgentID] = struct{}{}
		at := r["action_types"].(map[string]int)
		at[a.ActionType]++
		r["last_action_time"] = a.Timestamp
	}
	var keys []int
	for k := range rounds {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	var tl []map[string]any
	for _, k := range keys {
		r := rounds[k]
		agents := r["active_agents"].(map[int]struct{})
		ids := make([]int, 0, len(agents))
		for id := range agents {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		tl = append(tl, map[string]any{
			"round_num":           r["round_num"],
			"twitter_actions":     r["twitter_actions"],
			"reddit_actions":      r["reddit_actions"],
			"total_actions":       r["twitter_actions"].(int) + r["reddit_actions"].(int),
			"active_agents_count": len(agents),
			"active_agents":       ids,
			"action_types":        r["action_types"],
			"first_action_time":   r["first_action_time"],
			"last_action_time":    r["last_action_time"],
		})
	}
	return map[string]any{"rounds_count": len(tl), "timeline": tl}
}

func (r *Runtime) AgentStats(ctx context.Context, simulationID string) map[string]any {
	acts := collectAllActions(r.simDir(simulationID), "", nil, nil)
	by := map[int]map[string]any{}
	for _, a := range acts {
		if by[a.AgentID] == nil {
			by[a.AgentID] = map[string]any{
				"agent_id": a.AgentID, "agent_name": a.AgentName, "total_actions": 0,
				"twitter_actions": 0, "reddit_actions": 0, "action_types": map[string]int{},
				"first_action_time": a.Timestamp, "last_action_time": a.Timestamp,
			}
		}
		s := by[a.AgentID]
		s["total_actions"] = s["total_actions"].(int) + 1
		if a.Platform == "twitter" {
			s["twitter_actions"] = s["twitter_actions"].(int) + 1
		} else {
			s["reddit_actions"] = s["reddit_actions"].(int) + 1
		}
		at := s["action_types"].(map[string]int)
		at[a.ActionType]++
		s["last_action_time"] = a.Timestamp
	}
	var list []map[string]any
	for _, v := range by {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i]["total_actions"].(int) > list[j]["total_actions"].(int)
	})
	return map[string]any{"agents_count": len(list), "stats": list}
}

func (r *Runtime) Posts(ctx context.Context, simulationID, platform string, limit, offset int) map[string]any {
	_ = ctx
	if limit <= 0 {
		limit = 50
	}
	return QueryPosts(r.simDir(simulationID), platform, limit, offset)
}

func (r *Runtime) EnvStatus(ctx context.Context, simulationID string) map[string]any {
	_ = ctx
	c := NewIPCClient(r.simDir(simulationID))
	d := c.EnvDetail()
	alive := c.CheckEnvAlive()
	msg := "Environment not running or closed"
	if alive {
		msg = "Environment running, ready to receive interview requests"
	}
	return map[string]any{
		"simulation_id":     simulationID,
		"env_alive":         alive,
		"twitter_available": d["twitter_available"],
		"reddit_available":  d["reddit_available"],
		"message":           msg,
	}
}

func (r *Runtime) CloseEnv(ctx context.Context, simulationID string, timeoutSec int) map[string]any {
	c := NewIPCClient(r.simDir(simulationID))
	if !c.CheckEnvAlive() {
		return map[string]any{"success": true, "message": "Environment already closed"}
	}
	to := 30 * time.Second
	if timeoutSec > 0 {
		to = time.Duration(timeoutSec) * time.Second
	}
	resp, err := c.SendCloseEnv(to)
	if err != nil {
		return map[string]any{"success": true, "message": "Close environment command sent (timeout waiting for response, environment may be closing)"}
	}
	return map[string]any{
		"success":   fmt.Sprint(resp["status"]) == "completed",
		"message":   "Environment close command sent",
		"result":    resp["result"],
		"timestamp": resp["timestamp"],
	}
}

func (r *Runtime) InterviewBatch(ctx context.Context, simulationID string, interviews []map[string]any, platform *string, timeout float64) map[string]any {
	_ = ctx
	c := NewIPCClient(r.simDir(simulationID))
	if !c.CheckEnvAlive() {
		return map[string]any{"success": false, "error": "Simulation environment not running or closed"}
	}
	to := 120 * time.Second
	if timeout > 0 {
		to = time.Duration(timeout * float64(time.Second))
	}
	resp, err := c.SendBatchInterview(interviews, platform, to)
	if err != nil {
		return map[string]any{"success": false, "error": err.Error()}
	}
	ok := fmt.Sprint(resp["status"]) == "completed"
	if !ok {
		return map[string]any{"success": false, "interviews_count": len(interviews), "error": resp["error"], "timestamp": resp["timestamp"]}
	}
	return map[string]any{
		"success":          ok,
		"interviews_count": len(interviews),
		"result":           resp["result"],
		"timestamp":        resp["timestamp"],
	}
}
