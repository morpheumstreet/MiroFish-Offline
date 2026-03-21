package httpapi

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mirofish-offline/lake/internal/ports"
	"github.com/mirofish-offline/lake/internal/usecase/simulation"
)

const interviewPromptPrefix = "Based on your persona, all your past memories and actions, reply directly to me with text without calling any tools:"

func optimizeInterviewPrompt(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, interviewPromptPrefix) {
		return p
	}
	return interviewPromptPrefix + p
}

func (s *Server) requireGraph(w http.ResponseWriter) bool {
	if !s.deps.GraphReady || s.deps.Entity == nil {
		fail(w, http.StatusServiceUnavailable, "Neo4j graph storage not available")
		return false
	}
	return true
}

func (s *Server) handleSimEntities(w http.ResponseWriter, r *http.Request) {
	if !s.requireGraph(w) {
		return
	}
	graphID := r.PathValue("graphId")
	enrich := r.URL.Query().Get("enrich") != "false"
	var types []string
	if ts := r.URL.Query().Get("entity_types"); ts != "" {
		for _, p := range strings.Split(ts, ",") {
			if t := strings.TrimSpace(p); t != "" {
				types = append(types, t)
			}
		}
	}
	m, err := s.deps.Entity.FilterDefinedEntities(r.Context(), graphID, types, enrich)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, m)
}

func (s *Server) handleSimEntityDetail(w http.ResponseWriter, r *http.Request) {
	if !s.requireGraph(w) {
		return
	}
	graphID := r.PathValue("graphId")
	uuid := r.PathValue("entityUUID")
	m, err := s.deps.Entity.GetEntityWithContext(r.Context(), graphID, uuid)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		fail(w, http.StatusNotFound, "Entity does not exist: "+uuid)
		return
	}
	ok(w, m)
}

func (s *Server) handleSimEntitiesByType(w http.ResponseWriter, r *http.Request) {
	if !s.requireGraph(w) {
		return
	}
	graphID := r.PathValue("graphId")
	etype := r.PathValue("entityType")
	enrich := r.URL.Query().Get("enrich") != "false"
	list, err := s.deps.Entity.GetEntitiesByType(r.Context(), graphID, etype, enrich)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"entity_type": etype, "count": len(list), "entities": list})
}

func (s *Server) handleSimCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectID     string `json:"project_id"`
		GraphID       string `json:"graph_id"`
		EnableTwitter *bool  `json:"enable_twitter"`
		EnableReddit  *bool  `json:"enable_reddit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	tw, rd := true, true
	if body.EnableTwitter != nil {
		tw = *body.EnableTwitter
	}
	if body.EnableReddit != nil {
		rd = *body.EnableReddit
	}
	st, err := s.deps.Sim.Create(r.Context(), body.ProjectID, body.GraphID, tw, rd)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			fail(w, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(), "not built") || strings.Contains(err.Error(), "required") {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, simulation.StateAsMap(st))
}

func (s *Server) handleSimPrepare(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SimulationID         string   `json:"simulation_id"`
		EntityTypes          []string `json:"entity_types"`
		UseLLMForProfiles    *bool    `json:"use_llm_for_profiles"`
		ParallelProfileCount *int     `json:"parallel_profile_count"`
		ForceRegenerate      bool     `json:"force_regenerate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.SimulationID == "" {
		fail(w, http.StatusBadRequest, "Please provide simulation_id")
		return
	}
	useLLM := true
	if body.UseLLMForProfiles != nil {
		useLLM = *body.UseLLMForProfiles
	}
	parallel := 5
	if body.ParallelProfileCount != nil && *body.ParallelProfileCount > 0 {
		parallel = *body.ParallelProfileCount
	}
	data, err := s.deps.Sim.Prepare(r.Context(), body.SimulationID, body.EntityTypes, useLLM, parallel, body.ForceRegenerate)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			fail(w, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(), "missing") || strings.Contains(err.Error(), "not available") {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleSimPrepareStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TaskID       string `json:"task_id"`
		SimulationID string `json:"simulation_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	data, err := s.deps.Sim.PrepareStatus(r.Context(), body.TaskID, body.SimulationID)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleSimGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	_, m, err := s.deps.Sim.Get(r.Context(), id)
	if err != nil {
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	ok(w, m)
}

func (s *Server) handleSimList(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	list, err := s.deps.Sim.List(r.Context(), pid)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	okCount(w, list, len(list))
}

func (s *Server) handleSimHistory(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.deps.Sim.History(r.Context(), limit)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	okCount(w, list, len(list))
}

func (s *Server) handleSimProfiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "reddit"
	}
	raw, _, okFile, err := s.deps.Sim.ProfilesFile(r.Context(), id, platform)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !okFile {
		ok(w, map[string]any{"platform": platform, "count": 0, "profiles": []any{}})
		return
	}
	profiles, err := decodeProfilesFile(platform, raw)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"platform": platform, "count": len(profiles), "profiles": profiles})
}

func decodeProfilesFile(platform string, raw []byte) ([]any, error) {
	if platform == "reddit" || platform == "" {
		var arr []any
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}
	cr := csv.NewReader(strings.NewReader(string(raw)))
	h, err := cr.Read()
	if err != nil {
		return nil, err
	}
	var out []any
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, k := range h {
			if i < len(rec) {
				row[k] = rec[i]
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Server) handleSimProfilesRealtime(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "reddit"
	}
	raw, mod, okFile, err := s.deps.Sim.ProfilesFile(r.Context(), id, platform)
	st, _ := s.deps.Sim.Repo.Load(r.Context(), id)
	var totalExpected any
	isGen := false
	if st != nil {
		totalExpected = st.EntitiesCount
		isGen = st.Status == "preparing"
	}
	var modStr *string
	if okFile {
		ms := mod.Format("2006-01-02T15:04:05Z07:00")
		modStr = &ms
	}
	profiles := []any{}
	if okFile && err == nil {
		if p, e := decodeProfilesFile(platform, raw); e == nil {
			profiles = append(profiles, p...)
		}
	}
	ok(w, map[string]any{
		"simulation_id": id, "platform": platform, "count": len(profiles),
		"total_expected": totalExpected, "is_generating": isGen,
		"file_exists": okFile, "file_modified_at": modStr, "profiles": profiles,
	})
}

func (s *Server) handleSimConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	cfg, err := s.deps.Sim.ReadConfigJSON(r.Context(), id)
	if err != nil || cfg == nil {
		fail(w, http.StatusNotFound, "Simulation configuration does not exist. Please call /prepare first")
		return
	}
	ok(w, cfg)
}

func (s *Server) handleSimConfigRealtime(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	ctx := r.Context()
	mod, okFile := s.deps.Sim.Repo.StatFile(ctx, id, "simulation_config.json")
	var raw []byte
	var err error
	if okFile {
		raw, err = s.deps.Sim.Repo.ReadFile(ctx, id, "simulation_config.json")
	}
	st, _ := s.deps.Sim.Repo.Load(ctx, id)
	var modStr *string
	if okFile {
		ms := mod.Format("2006-01-02T15:04:05Z07:00")
		modStr = &ms
	}
	var cfg any
	if okFile && err == nil {
		_ = json.Unmarshal(raw, &cfg)
	}
	isGen := false
	var genStage any
	cfgGen := false
	if st != nil {
		isGen = st.Status == "preparing"
		cfgGen = st.ConfigGenerated
		if isGen {
			if st.ProfilesGenerated {
				genStage = "generating_config"
			} else {
				genStage = "generating_profiles"
			}
		} else if st.Status == "ready" {
			genStage = "completed"
		}
	}
	out := map[string]any{
		"simulation_id": id, "file_exists": okFile, "file_modified_at": modStr,
		"is_generating": isGen, "generation_stage": genStage, "config_generated": cfgGen,
		"config": cfg,
	}
	if cfg != nil {
		if m, ok := cfg.(map[string]any); ok {
			ac, _ := m["agent_configs"].([]any)
			tc, _ := m["time_config"].(map[string]any)
			ec, _ := m["event_config"].(map[string]any)
			var ip, ht []any
			if ec != nil {
				ip, _ = ec["initial_posts"].([]any)
				ht, _ = ec["hot_topics"].([]any)
			}
			out["summary"] = map[string]any{
				"total_agents": len(ac), "simulation_hours": tc["total_simulation_hours"],
				"initial_posts_count": len(ip), "hot_topics_count": len(ht),
				"has_twitter_config": m["twitter_config"] != nil,
				"has_reddit_config":  m["reddit_config"] != nil,
				"generated_at":       m["generated_at"], "llm_model": m["llm_model"],
			}
		}
	}
	ok(w, out)
}

func (s *Server) handleSimConfigDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	path := filepath.Join(s.deps.Sim.Repo.SimulationsRoot(), id, "simulation_config.json")
	http.ServeFile(w, r, path)
}

func (s *Server) handleSimScriptDownload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("scriptName")
	allowed := map[string]struct{}{
		"run_twitter_simulation.py": {}, "run_reddit_simulation.py": {},
		"run_parallel_simulation.py": {}, "action_logger.py": {},
	}
	if _, ok := allowed[name]; !ok {
		fail(w, http.StatusBadRequest, "Unknown script: "+name)
		return
	}
	dir := s.deps.Config.ScriptsDir()
	if dir == "" {
		fail(w, http.StatusInternalServerError, "scripts directory not configured")
		return
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		fail(w, http.StatusNotFound, "Script file does not exist")
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	http.ServeFile(w, r, path)
}

func (s *Server) handleSimGenerateProfiles(w http.ResponseWriter, r *http.Request) {
	if !s.requireGraph(w) {
		return
	}
	var body struct {
		GraphID     string   `json:"graph_id"`
		EntityTypes []string `json:"entity_types"`
		UseLLM      *bool    `json:"use_llm"`
		Platform    string   `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.GraphID == "" {
		fail(w, http.StatusBadRequest, "graph_id required")
		return
	}
	useLLM := true
	if body.UseLLM != nil {
		useLLM = *body.UseLLM
	}
	pl := body.Platform
	if pl == "" {
		pl = "reddit"
	}
	data, err := s.deps.Sim.GenerateProfilesStandalone(r.Context(), body.GraphID, body.EntityTypes, useLLM, pl)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleSimStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SimulationID            string  `json:"simulation_id"`
		Platform                string  `json:"platform"`
		MaxRounds               float64 `json:"max_rounds"`
		EnableGraphMemoryUpdate bool    `json:"enable_graph_memory_update"`
		Force                   bool    `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SimulationID == "" {
		fail(w, http.StatusBadRequest, "Please provide simulation_id")
		return
	}
	pl := body.Platform
	if pl == "" {
		pl = "parallel"
	}
	var mr *int
	if body.MaxRounds > 0 {
		m := int(body.MaxRounds)
		mr = &m
	}
	st, _ := s.deps.Sim.Repo.Load(r.Context(), body.SimulationID)
	gid := ""
	if st != nil {
		gid = st.GraphID
	}
	out, err := s.deps.Sim.Start(r.Context(), body.SimulationID, pl, mr, body.EnableGraphMemoryUpdate, gid, body.Force)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, out)
}

func (s *Server) handleSimStop(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SimulationID string `json:"simulation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SimulationID == "" {
		fail(w, http.StatusBadRequest, "Please provide simulation_id")
		return
	}
	out, err := s.deps.Sim.Stop(r.Context(), body.SimulationID)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, out)
}

func (s *Server) handleSimRunStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	ok(w, s.deps.Sim.Runtime.RunState(r.Context(), id))
}

func (s *Server) handleSimRunStatusDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	pl := r.URL.Query().Get("platform")
	ok(w, s.deps.Sim.Runtime.RunStateDetail(r.Context(), id, pl))
}

func (s *Server) handleSimActions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	q := parseActionQuery(r)
	ok(w, s.deps.Sim.Runtime.Actions(r.Context(), id, q))
}

func parseActionQuery(r *http.Request) ports.ActionQuery {
	q := ports.ActionQuery{Limit: 100, Offset: 0}
	if v := r.URL.Query().Get("limit"); v != "" {
		q.Limit, _ = strconv.Atoi(v)
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		q.Offset, _ = strconv.Atoi(v)
	}
	q.Platform = r.URL.Query().Get("platform")
	if v := r.URL.Query().Get("agent_id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.AgentID = &n
		}
	}
	if v := r.URL.Query().Get("round_num"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.RoundNum = &n
		}
	}
	return q
}

func (s *Server) handleSimTimeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	start, _ := strconv.Atoi(r.URL.Query().Get("start_round"))
	var end *int
	if v := r.URL.Query().Get("end_round"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			end = &n
		}
	}
	ok(w, s.deps.Sim.Runtime.Timeline(r.Context(), id, start, end))
}

func (s *Server) handleSimAgentStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	ok(w, s.deps.Sim.Runtime.AgentStats(r.Context(), id))
}

func (s *Server) handleSimPosts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("simulationId")
	pl := r.URL.Query().Get("platform")
	if pl == "" {
		pl = "reddit"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	ok(w, s.deps.Sim.Runtime.Posts(r.Context(), id, pl, limit, off))
}

func (s *Server) handleSimComments(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]any{"count": 0, "comments": []any{}})
}

func (s *Server) handleSimEnvStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SimulationID string `json:"simulation_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.SimulationID == "" {
		fail(w, http.StatusBadRequest, "Please provide simulation_id")
		return
	}
	ok(w, s.deps.Sim.Runtime.EnvStatus(r.Context(), body.SimulationID))
}

func (s *Server) handleSimCloseEnv(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SimulationID string `json:"simulation_id"`
		Timeout      int    `json:"timeout"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.SimulationID == "" {
		fail(w, http.StatusBadRequest, "Please provide simulation_id")
		return
	}
	ok(w, s.deps.Sim.Runtime.CloseEnv(r.Context(), body.SimulationID, body.Timeout))
}

func (s *Server) handleSimInterviewBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SimulationID string           `json:"simulation_id"`
		Interviews   []map[string]any `json:"interviews"`
		Platform     *string          `json:"platform"`
		Timeout      float64          `json:"timeout"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SimulationID == "" {
		fail(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(body.Interviews) == 0 {
		fail(w, http.StatusBadRequest, "Please provide interviews")
		return
	}
	for i := range body.Interviews {
		if p, ok := body.Interviews[i]["prompt"].(string); ok {
			body.Interviews[i]["prompt"] = optimizeInterviewPrompt(p)
		}
	}
	res := s.deps.Sim.Runtime.InterviewBatch(r.Context(), body.SimulationID, body.Interviews, body.Platform, body.Timeout)
	if fmt.Sprint(res["success"]) == "false" {
		writeJSON(w, http.StatusBadRequest, envelope{Success: false, Error: fmt.Sprint(res["error"])})
		return
	}
	writeJSON(w, http.StatusOK, envelope{Success: true, Data: res})
}
