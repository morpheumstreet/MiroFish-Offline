package simulation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirofish-offline/lake/internal/config"
	"github.com/mirofish-offline/lake/internal/domain"
	"github.com/mirofish-offline/lake/internal/ports"
)

// Service orchestrates simulation APIs (thin domain + ports).
type Service struct {
	Cfg      config.Config
	Projects ports.ProjectRepository
	Tasks    ports.TaskRepository
	Repo     ports.SimulationRepository
	Entities ports.EntityReader
	Profiles ports.ProfileBuilder
	Config   ports.SimulationConfigBuilder
	Runtime  ports.SimulationRuntime
	GraphOK  bool
}

func (s *Service) Create(ctx context.Context, projectID, graphID string, enableTwitter, enableReddit bool) (*domain.SimulationState, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}
	p, err := s.Projects.GetProject(ctx, projectID)
	if err != nil || p == nil {
		return nil, fmt.Errorf("project does not exist: %s", projectID)
	}
	if graphID == "" {
		graphID = p.GraphID
	}
	if graphID == "" {
		return nil, fmt.Errorf("project has not built knowledge graph yet")
	}
	return s.Repo.Create(ctx, projectID, graphID, enableTwitter, enableReddit)
}

func (s *Service) Get(ctx context.Context, simulationID string) (*domain.SimulationState, map[string]any, error) {
	st, err := s.Repo.Load(ctx, simulationID)
	if err != nil || st == nil {
		return nil, nil, fmt.Errorf("simulation does not exist: %s", simulationID)
	}
	m := stateToMap(st)
	if st.Status == "ready" {
		m["run_instructions"] = s.runInstructions(st.SimulationID)
	}
	return st, m, nil
}

func (s *Service) runInstructions(simulationID string) map[string]any {
	simDir := filepath.Join(s.Repo.SimulationsRoot(), simulationID)
	cfgPath := filepath.Join(simDir, "simulation_config.json")
	scripts := s.Cfg.ScriptsDir()
	return map[string]any{
		"simulation_dir": simDir,
		"scripts_dir":    scripts,
		"config_file":    cfgPath,
		"commands": map[string]any{
			"twitter":  fmt.Sprintf("python %s/run_twitter_simulation.py --config %s", scripts, cfgPath),
			"reddit":   fmt.Sprintf("python %s/run_reddit_simulation.py --config %s", scripts, cfgPath),
			"parallel": fmt.Sprintf("python %s/run_parallel_simulation.py --config %s", scripts, cfgPath),
		},
		"instructions": fmt.Sprintf("Run from %s with --config %s", scripts, cfgPath),
	}
}

func stateToMap(st *domain.SimulationState) map[string]any {
	b, _ := json.Marshal(st)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// StateAsMap exports simulation state for JSON API responses.
func StateAsMap(st *domain.SimulationState) map[string]any { return stateToMap(st) }

func (s *Service) List(ctx context.Context, projectID string) ([]map[string]any, error) {
	list, err := s.Repo.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for _, st := range list {
		out = append(out, stateToMap(st))
	}
	return out, nil
}

func (s *Service) History(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 20
	}
	list, err := s.Repo.List(ctx, "")
	if err != nil {
		return nil, err
	}
	var enriched []map[string]any
	for _, sim := range list {
		if len(enriched) >= limit {
			break
		}
		m := stateToMap(sim)
		if raw, err := s.Repo.ReadFile(ctx, sim.SimulationID, "simulation_config.json"); err == nil {
			var cfg map[string]any
			if json.Unmarshal(raw, &cfg) == nil {
				m["simulation_requirement"] = cfg["simulation_requirement"]
				if tc, ok := cfg["time_config"].(map[string]any); ok {
					m["total_simulation_hours"] = tc["total_simulation_hours"]
					th := intFromAny(tc["total_simulation_hours"])
					mpr := intFromAny(tc["minutes_per_round"])
					if mpr <= 0 {
						mpr = 60
					}
					m["total_rounds"] = int(float64(th*60) / float64(mpr))
				}
			}
		}
		rs := s.Runtime.RunState(ctx, sim.SimulationID)
		m["current_round"] = intFromAny(rs["current_round"])
		m["runner_status"] = fmt.Sprint(rs["runner_status"])
		if tr, ok := rs["total_rounds"].(int); ok && tr > 0 {
			m["total_rounds"] = tr
		}
		if p, err := s.Projects.GetProject(ctx, sim.ProjectID); err == nil && p != nil && len(p.Files) > 0 {
			var files []map[string]any
			for _, f := range p.Files {
				if len(files) >= 3 {
					break
				}
				files = append(files, map[string]any{"filename": f.Filename})
			}
			m["files"] = files
		} else {
			m["files"] = []map[string]any{}
		}
		m["report_id"] = s.latestReportID(sim.SimulationID)
		m["version"] = "v1.0.2"
		if ca := fmt.Sprint(m["created_at"]); len(ca) >= 10 {
			m["created_date"] = ca[:10]
		}
		enriched = append(enriched, m)
	}
	return enriched, nil
}

func (s *Service) latestReportID(simulationID string) any {
	dir := s.Cfg.ReportsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type row struct {
		id, created string
	}
	var rows []row
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(dir, e.Name(), "meta.json")
		raw, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta map[string]any
		if json.Unmarshal(raw, &meta) != nil {
			continue
		}
		if fmt.Sprint(meta["simulation_id"]) != simulationID {
			continue
		}
		rows = append(rows, row{id: fmt.Sprint(meta["report_id"]), created: fmt.Sprint(meta["created_at"])})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].created > rows[j].created })
	if len(rows) == 0 {
		return nil
	}
	return rows[0].id
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	default:
		return 0
	}
}

// Prepare starts background task; mirrors Flask /prepare.
func (s *Service) Prepare(ctx context.Context, simulationID string, entityTypes []string, useLLM bool, parallel int, force bool) (map[string]any, error) {
	if !s.GraphOK {
		return nil, fmt.Errorf("graph storage not available")
	}
	st, err := s.Repo.Load(ctx, simulationID)
	if err != nil || st == nil {
		return nil, fmt.Errorf("simulation does not exist: %s", simulationID)
	}
	if !force {
		if ok, info := ports.CheckSimulationPrepared(s.Repo, simulationID); ok {
			_ = s.promotePreparingIfNeeded(ctx, simulationID)
			return map[string]any{
				"simulation_id":    simulationID,
				"status":           "ready",
				"message":          "Preparation already completed",
				"already_prepared": true,
				"prepare_info":     info,
			}, nil
		}
	}
	p, err := s.Projects.GetProject(ctx, st.ProjectID)
	if err != nil || p == nil {
		return nil, fmt.Errorf("project does not exist: %s", st.ProjectID)
	}
	req := strings.TrimSpace(p.SimulationRequirement)
	if req == "" {
		return nil, fmt.Errorf("project missing simulation requirement description")
	}
	doc, _ := s.Projects.GetExtractedText(ctx, st.ProjectID)

	filtered, err := s.Entities.FilterDefinedEntities(ctx, st.GraphID, entityTypes, false)
	if err == nil {
		st.EntitiesCount = intFromAny(filtered["filtered_count"])
		if et, ok := filtered["entity_types"].([]any); ok {
			st.EntityTypes = nil
			for _, x := range et {
				st.EntityTypes = append(st.EntityTypes, fmt.Sprint(x))
			}
		}
	}
	st.Status = "preparing"
	_ = s.Repo.Save(ctx, st)

	taskID, err := s.Tasks.CreateTask(ctx, "simulation_prepare", map[string]any{
		"simulation_id": simulationID,
		"project_id":    st.ProjectID,
	})
	if err != nil {
		return nil, err
	}

	go s.runPrepare(taskID, simulationID, st, entityTypes, useLLM, parallel, req, doc)

	return map[string]any{
		"simulation_id":    simulationID,
		"task_id":          taskID,
		"status":           "preparing",
		"message":          "Preparation task started",
		"already_prepared": false,
	}, nil
}

func (s *Service) promotePreparingIfNeeded(ctx context.Context, simulationID string) error {
	return s.Repo.PromotePreparingToReady(ctx, simulationID)
}

func (s *Service) runPrepare(taskID, simulationID string, st *domain.SimulationState, entityTypes []string, useLLM bool, parallel int, req, doc string) {
	ctx := context.Background()
	stProc := domain.TaskProcessing
	z := 0
	startMsg := "Start preparing simulation environment..."
	_ = s.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{Status: &stProc, Progress: &z, Message: &startMsg})

	patch := func(progress int, msg string, detail map[string]any) {
		_ = s.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{
			Progress:       &progress,
			Message:        &msg,
			ProgressDetail: detail,
		})
	}
	fail := func(errMsg string) {
		_ = s.Tasks.FailTask(ctx, taskID, errMsg)
		st2, _ := s.Repo.Load(ctx, simulationID)
		if st2 != nil {
			st2.Status = "failed"
			st2.Error = &errMsg
			_ = s.Repo.Save(ctx, st2)
		}
	}
	complete := func() {
		st2, _ := s.Repo.Load(ctx, simulationID)
		if st2 != nil {
			_ = s.Tasks.CompleteTask(ctx, taskID, st2.ToSimpleMap())
		}
	}

	stageWeights := map[string]struct{ start, end int }{
		"reading":             {0, 20},
		"generating_profiles": {20, 70},
		"generating_config":   {70, 90},
		"copying_scripts":     {90, 100},
	}
	combine := func(stage string, pct int) int {
		w := stageWeights[stage]
		return w.start + (w.end-w.start)*pct/100
	}

	filtered, err := s.Entities.FilterDefinedEntities(ctx, st.GraphID, entityTypes, true)
	if err != nil {
		fail(err.Error())
		return
	}
	entities, _ := filtered["entities"].([]any)
	entMaps := make([]map[string]any, 0, len(entities))
	for _, x := range entities {
		if m, ok := x.(map[string]any); ok {
			entMaps = append(entMaps, m)
		}
	}
	st.EntitiesCount = len(entMaps)
	if ft, ok := filtered["entity_types"].([]any); ok {
		st.EntityTypes = nil
		for _, x := range ft {
			st.EntityTypes = append(st.EntityTypes, fmt.Sprint(x))
		}
	}
	_ = s.Repo.Save(ctx, st)

	if len(entMaps) == 0 {
		fail("No entities matching criteria found")
		return
	}
	patch(combine("reading", 100), fmt.Sprintf("Completed, total %d entities", len(entMaps)), map[string]any{
		"current_stage": "reading", "current_item": len(entMaps), "total_items": len(entMaps),
	})

	simDir := filepath.Join(s.Repo.SimulationsRoot(), simulationID)
	appendReddit := func(profiles []map[string]any) error {
		return s.Profiles.SaveRedditJSON(filepath.Join(simDir, "reddit_profiles.json"), profiles)
	}

	profProgress := func(cur, tot int, msg string) {
		pct := 0
		if tot > 0 {
			pct = cur * 100 / tot
		}
		patch(combine("generating_profiles", pct), msg, map[string]any{
			"current_stage": "generating_profiles", "current_item": cur, "total_items": tot,
		})
	}

	profiles, err := s.Profiles.BuildProfiles(ctx, st.GraphID, entMaps, useLLM, parallel, profProgress, appendReddit)
	if err != nil {
		fail(err.Error())
		return
	}
	st.ProfilesCount = len(profiles)
	st.ProfilesGenerated = true
	_ = s.Repo.Save(ctx, st)

	if st.EnableReddit {
		if err := s.Profiles.SaveRedditJSON(filepath.Join(simDir, "reddit_profiles.json"), profiles); err != nil {
			fail(err.Error())
			return
		}
	}
	if st.EnableTwitter {
		if err := s.Profiles.SaveTwitterCSV(filepath.Join(simDir, "twitter_profiles.csv"), profiles); err != nil {
			fail(err.Error())
			return
		}
	}

	patch(combine("generating_config", 30), "Calling LLM to generate config...", nil)
	cfg, reasoning, err := s.Config.Build(ctx, simulationID, st.ProjectID, st.GraphID, req, doc, entMaps, st.EnableTwitter, st.EnableReddit)
	if err != nil {
		fail(err.Error())
		return
	}
	cfg["generation_reasoning"] = reasoning
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fail(err.Error())
		return
	}
	if err := s.Repo.WriteFile(ctx, simulationID, "simulation_config.json", raw); err != nil {
		fail(err.Error())
		return
	}
	st.ConfigGenerated = true
	st.ConfigReasoning = reasoning
	st.Status = "ready"
	st.Error = nil
	_ = s.Repo.Save(ctx, st)

	patch(100, "Preparation complete", map[string]any{"current_stage": "copying_scripts"})
	complete()
}

func (s *Service) PrepareStatus(ctx context.Context, taskID, simulationID string) (map[string]any, error) {
	if simulationID != "" {
		_ = s.promotePreparingIfNeeded(ctx, simulationID)
		if ok, info := ports.CheckSimulationPrepared(s.Repo, simulationID); ok {
			return map[string]any{
				"simulation_id":    simulationID,
				"status":           "ready",
				"progress":         100,
				"message":          "Preparation already completed",
				"already_prepared": true,
				"prepare_info":     info,
			}, nil
		}
		if taskID == "" {
			return map[string]any{
				"simulation_id":    simulationID,
				"status":           "not_started",
				"progress":         0,
				"message":          "Preparation not started yet",
				"already_prepared": false,
			}, nil
		}
	}
	if taskID == "" {
		return nil, fmt.Errorf("task_id or simulation_id required")
	}
	t, err := s.Tasks.GetTask(ctx, taskID)
	if err != nil || t == nil {
		if simulationID != "" {
			if ok, info := ports.CheckSimulationPrepared(s.Repo, simulationID); ok {
				return map[string]any{
					"simulation_id":    simulationID,
					"task_id":          taskID,
					"status":           "ready",
					"progress":         100,
					"already_prepared": true,
					"prepare_info":     info,
				}, nil
			}
		}
		return nil, fmt.Errorf("task does not exist: %s", taskID)
	}
	// Map task to API shape (simplified).
	b, _ := json.Marshal(t)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["already_prepared"] = false
	return m, nil
}

func (s *Service) Start(ctx context.Context, simulationID, platform string, maxRounds *int, graphMem bool, graphID string, force bool) (map[string]any, error) {
	st, err := s.Repo.Load(ctx, simulationID)
	if err != nil || st == nil {
		return nil, fmt.Errorf("simulation does not exist: %s", simulationID)
	}
	if force {
		_, _ = s.Runtime.Stop(ctx, simulationID)
		st, err = s.Repo.Load(ctx, simulationID)
		if err != nil || st == nil {
			return nil, fmt.Errorf("simulation does not exist: %s", simulationID)
		}
	}
	if st.Status != "ready" {
		if ok, _ := ports.CheckSimulationPrepared(s.Repo, simulationID); ok {
			st.Status = "ready"
			_ = s.Repo.Save(ctx, st)
		} else {
			return nil, fmt.Errorf("simulation not ready. Current status: %s", st.Status)
		}
	}
	in := ports.SimulationStartInput{
		SimulationID:            simulationID,
		Platform:                platform,
		MaxRounds:               maxRounds,
		EnableGraphMemoryUpdate: graphMem,
		GraphID:                 graphID,
		Force:                   force,
	}
	if graphMem && graphID == "" {
		graphID = st.GraphID
		in.GraphID = graphID
	}
	if graphMem && graphID == "" {
		return nil, fmt.Errorf("enable_graph_memory_update requires graph_id")
	}
	out, err := s.Runtime.Start(ctx, in)
	if err != nil {
		return nil, err
	}
	st.Status = "running"
	_ = s.Repo.Save(ctx, st)
	return out, nil
}

func (s *Service) Stop(ctx context.Context, simulationID string) (map[string]any, error) {
	out, err := s.Runtime.Stop(ctx, simulationID)
	if err != nil {
		return nil, err
	}
	st, _ := s.Repo.Load(ctx, simulationID)
	if st != nil {
		st.Status = "paused"
		_ = s.Repo.Save(ctx, st)
	}
	return out, nil
}

func (s *Service) ReadConfigJSON(ctx context.Context, simulationID string) (map[string]any, error) {
	raw, err := s.Repo.ReadFile(ctx, simulationID, "simulation_config.json")
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// GenerateProfilesStandalone implements POST /simulation/generate-profiles (no simulation dir).
func (s *Service) GenerateProfilesStandalone(ctx context.Context, graphID string, entityTypes []string, useLLM bool, platform string) (map[string]any, error) {
	if !s.GraphOK {
		return nil, fmt.Errorf("graph storage not available")
	}
	filtered, err := s.Entities.FilterDefinedEntities(ctx, graphID, entityTypes, true)
	if err != nil {
		return nil, err
	}
	raw, _ := filtered["entities"].([]any)
	entMaps := make([]map[string]any, 0, len(raw))
	for _, x := range raw {
		if m, ok := x.(map[string]any); ok {
			entMaps = append(entMaps, m)
		}
	}
	profiles, err := s.Profiles.BuildProfiles(ctx, graphID, entMaps, useLLM, 3, nil, nil)
	if err != nil {
		return nil, err
	}
	var data []map[string]any
	for _, p := range profiles {
		if platform == "twitter" {
			// flatten to twitter-ish shape
			data = append(data, map[string]any{
				"user_id": p["user_id"], "username": p["username"], "name": p["name"],
				"bio": p["bio"], "persona": p["persona"],
			})
		} else {
			data = append(data, p)
		}
	}
	et := []string{}
	if v, ok := filtered["entity_types"].([]any); ok {
		for _, x := range v {
			et = append(et, fmt.Sprint(x))
		}
	}
	return map[string]any{
		"platform":     platform,
		"entity_types": et,
		"count":        len(data),
		"profiles":     data,
	}, nil
}

func (s *Service) ProfilesFile(ctx context.Context, simulationID, platform string) ([]byte, time.Time, bool, error) {
	name := "reddit_profiles.json"
	if platform == "twitter" {
		// API get_profiles uses json path in Python manager — realtime uses csv for twitter.
		name = "twitter_profiles.csv"
	}
	mod, ok := s.Repo.StatFile(ctx, simulationID, name)
	if !ok {
		return nil, time.Time{}, false, nil
	}
	raw, err := s.Repo.ReadFile(ctx, simulationID, name)
	return raw, mod, true, err
}
