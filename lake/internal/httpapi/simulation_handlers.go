package httpapi

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
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

func (s *Server) requireGraph(c *fiber.Ctx) bool {
	if !s.deps.GraphReady || s.deps.Entity == nil {
		_ = failResp(c, fiber.StatusServiceUnavailable, "Neo4j graph storage not available")
		return false
	}
	return true
}

func (s *Server) handleSimEntities(c *fiber.Ctx) error {
	if !s.requireGraph(c) {
		return nil
	}
	graphID := c.Params("graphId")
	enrich := c.Query("enrich", "true") != "false"
	var types []string
	if ts := c.Query("entity_types"); ts != "" {
		for _, p := range strings.Split(ts, ",") {
			if t := strings.TrimSpace(p); t != "" {
				types = append(types, t)
			}
		}
	}
	m, err := s.deps.Entity.FilterDefinedEntities(s.reqCtx(c), graphID, types, enrich)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, m)
}

func (s *Server) handleSimEntityDetail(c *fiber.Ctx) error {
	if !s.requireGraph(c) {
		return nil
	}
	graphID := c.Params("graphId")
	uuid := c.Params("entityUUID")
	m, err := s.deps.Entity.GetEntityWithContext(s.reqCtx(c), graphID, uuid)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	if m == nil {
		return failResp(c, fiber.StatusNotFound, "Entity does not exist: "+uuid)
	}
	return okResp(c, m)
}

func (s *Server) handleSimEntitiesByType(c *fiber.Ctx) error {
	if !s.requireGraph(c) {
		return nil
	}
	graphID := c.Params("graphId")
	etype := c.Params("entityType")
	enrich := c.Query("enrich", "true") != "false"
	list, err := s.deps.Entity.GetEntitiesByType(s.reqCtx(c), graphID, etype, enrich)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, map[string]any{"entity_type": etype, "count": len(list), "entities": list})
}

func (s *Server) handleSimCreate(c *fiber.Ctx) error {
	var body struct {
		ProjectID     string `json:"project_id"`
		GraphID       string `json:"graph_id"`
		EnableTwitter *bool  `json:"enable_twitter"`
		EnableReddit  *bool  `json:"enable_reddit"`
	}
	if err := c.BodyParser(&body); err != nil {
		return failResp(c, fiber.StatusBadRequest, "invalid JSON")
	}
	tw, rd := true, true
	if body.EnableTwitter != nil {
		tw = *body.EnableTwitter
	}
	if body.EnableReddit != nil {
		rd = *body.EnableReddit
	}
	st, err := s.deps.Sim.Create(s.reqCtx(c), body.ProjectID, body.GraphID, tw, rd)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return failResp(c, fiber.StatusNotFound, err.Error())
		}
		if strings.Contains(err.Error(), "not built") || strings.Contains(err.Error(), "required") {
			return failResp(c, fiber.StatusBadRequest, err.Error())
		}
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, simulation.StateAsMap(st))
}

func (s *Server) handleSimPrepare(c *fiber.Ctx) error {
	var body struct {
		SimulationID         string   `json:"simulation_id"`
		EntityTypes          []string `json:"entity_types"`
		UseLLMForProfiles    *bool    `json:"use_llm_for_profiles"`
		ParallelProfileCount *int     `json:"parallel_profile_count"`
		ForceRegenerate      bool     `json:"force_regenerate"`
	}
	if err := c.BodyParser(&body); err != nil {
		return failResp(c, fiber.StatusBadRequest, "invalid JSON")
	}
	if body.SimulationID == "" {
		return failResp(c, fiber.StatusBadRequest, "Please provide simulation_id")
	}
	useLLM := true
	if body.UseLLMForProfiles != nil {
		useLLM = *body.UseLLMForProfiles
	}
	parallel := 5
	if body.ParallelProfileCount != nil && *body.ParallelProfileCount > 0 {
		parallel = *body.ParallelProfileCount
	}
	data, err := s.deps.Sim.Prepare(s.reqCtx(c), body.SimulationID, body.EntityTypes, useLLM, parallel, body.ForceRegenerate)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return failResp(c, fiber.StatusNotFound, err.Error())
		}
		if strings.Contains(err.Error(), "missing") || strings.Contains(err.Error(), "not available") {
			return failResp(c, fiber.StatusBadRequest, err.Error())
		}
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleSimPrepareStatus(c *fiber.Ctx) error {
	var body struct {
		TaskID       string `json:"task_id"`
		SimulationID string `json:"simulation_id"`
	}
	_ = c.BodyParser(&body)
	data, err := s.deps.Sim.PrepareStatus(s.reqCtx(c), body.TaskID, body.SimulationID)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			return failResp(c, fiber.StatusBadRequest, err.Error())
		}
		return failResp(c, fiber.StatusNotFound, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleSimGet(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	_, m, err := s.deps.Sim.Get(s.reqCtx(c), id)
	if err != nil {
		return failResp(c, fiber.StatusNotFound, err.Error())
	}
	return okResp(c, m)
}

func (s *Server) handleSimList(c *fiber.Ctx) error {
	pid := c.Query("project_id")
	list, err := s.deps.Sim.List(s.reqCtx(c), pid)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okCountResp(c, list, len(list))
}

func (s *Server) handleSimHistory(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	list, err := s.deps.Sim.History(s.reqCtx(c), limit)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okCountResp(c, list, len(list))
}

func (s *Server) handleSimProfiles(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	platform := c.Query("platform", "reddit")
	raw, _, okFile, err := s.deps.Sim.ProfilesFile(s.reqCtx(c), id, platform)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	if !okFile {
		return okResp(c, map[string]any{"platform": platform, "count": 0, "profiles": []any{}})
	}
	profiles, err := decodeProfilesFile(platform, raw)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, map[string]any{"platform": platform, "count": len(profiles), "profiles": profiles})
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

func (s *Server) handleSimProfilesRealtime(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	platform := c.Query("platform", "reddit")
	raw, mod, okFile, err := s.deps.Sim.ProfilesFile(s.reqCtx(c), id, platform)
	st, _ := s.deps.Sim.Repo.Load(s.reqCtx(c), id)
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
	return okResp(c, map[string]any{
		"simulation_id": id, "platform": platform, "count": len(profiles),
		"total_expected": totalExpected, "is_generating": isGen,
		"file_exists": okFile, "file_modified_at": modStr, "profiles": profiles,
	})
}

func (s *Server) handleSimConfig(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	cfg, err := s.deps.Sim.ReadConfigJSON(s.reqCtx(c), id)
	if err != nil || cfg == nil {
		return failResp(c, fiber.StatusNotFound, "Simulation configuration does not exist. Please call /prepare first")
	}
	return okResp(c, cfg)
}

func (s *Server) handleSimConfigRealtime(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	ctx := s.reqCtx(c)
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
	return okResp(c, out)
}

func (s *Server) handleSimConfigDownload(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	path := filepath.Join(s.deps.Sim.Repo.SimulationsRoot(), id, "simulation_config.json")
	return c.SendFile(path)
}

func (s *Server) handleSimScriptDownload(c *fiber.Ctx) error {
	name := c.Params("scriptName")
	allowed := map[string]struct{}{
		"run_twitter_simulation.py": {}, "run_reddit_simulation.py": {},
		"run_parallel_simulation.py": {}, "action_logger.py": {},
	}
	if _, ok := allowed[name]; !ok {
		return failResp(c, fiber.StatusBadRequest, "Unknown script: "+name)
	}
	dir := s.deps.Config.ScriptsDir()
	if dir == "" {
		return failResp(c, fiber.StatusInternalServerError, "scripts directory not configured")
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		return failResp(c, fiber.StatusNotFound, "Script file does not exist")
	}
	return c.Download(path, name)
}

func (s *Server) handleSimGenerateProfiles(c *fiber.Ctx) error {
	if !s.requireGraph(c) {
		return nil
	}
	var body struct {
		GraphID     string   `json:"graph_id"`
		EntityTypes []string `json:"entity_types"`
		UseLLM      *bool    `json:"use_llm"`
		Platform    string   `json:"platform"`
	}
	if err := c.BodyParser(&body); err != nil || body.GraphID == "" {
		return failResp(c, fiber.StatusBadRequest, "graph_id required")
	}
	useLLM := true
	if body.UseLLM != nil {
		useLLM = *body.UseLLM
	}
	pl := body.Platform
	if pl == "" {
		pl = "reddit"
	}
	data, err := s.deps.Sim.GenerateProfilesStandalone(s.reqCtx(c), body.GraphID, body.EntityTypes, useLLM, pl)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleSimStart(c *fiber.Ctx) error {
	var body struct {
		SimulationID            string  `json:"simulation_id"`
		Platform                string  `json:"platform"`
		MaxRounds               float64 `json:"max_rounds"`
		EnableGraphMemoryUpdate bool    `json:"enable_graph_memory_update"`
		Force                   bool    `json:"force"`
	}
	if err := c.BodyParser(&body); err != nil || body.SimulationID == "" {
		return failResp(c, fiber.StatusBadRequest, "Please provide simulation_id")
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
	st, _ := s.deps.Sim.Repo.Load(s.reqCtx(c), body.SimulationID)
	gid := ""
	if st != nil {
		gid = st.GraphID
	}
	out, err := s.deps.Sim.Start(s.reqCtx(c), body.SimulationID, pl, mr, body.EnableGraphMemoryUpdate, gid, body.Force)
	if err != nil {
		return failResp(c, fiber.StatusBadRequest, err.Error())
	}
	return okResp(c, out)
}

func (s *Server) handleSimStop(c *fiber.Ctx) error {
	var body struct {
		SimulationID string `json:"simulation_id"`
	}
	if err := c.BodyParser(&body); err != nil || body.SimulationID == "" {
		return failResp(c, fiber.StatusBadRequest, "Please provide simulation_id")
	}
	out, err := s.deps.Sim.Stop(s.reqCtx(c), body.SimulationID)
	if err != nil {
		return failResp(c, fiber.StatusBadRequest, err.Error())
	}
	return okResp(c, out)
}

func (s *Server) handleSimRunStatus(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	return okResp(c, s.deps.Sim.Runtime.RunState(s.reqCtx(c), id))
}

func (s *Server) handleSimRunStatusDetail(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	pl := c.Query("platform")
	return okResp(c, s.deps.Sim.Runtime.RunStateDetail(s.reqCtx(c), id, pl))
}

func (s *Server) handleSimActions(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	q := parseActionQuery(c)
	return okResp(c, s.deps.Sim.Runtime.Actions(s.reqCtx(c), id, q))
}

func parseActionQuery(c *fiber.Ctx) ports.ActionQuery {
	q := ports.ActionQuery{Limit: 100, Offset: 0}
	if v := c.Query("limit"); v != "" {
		q.Limit, _ = strconv.Atoi(v)
	}
	if v := c.Query("offset"); v != "" {
		q.Offset, _ = strconv.Atoi(v)
	}
	q.Platform = c.Query("platform")
	if v := c.Query("agent_id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.AgentID = &n
		}
	}
	if v := c.Query("round_num"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q.RoundNum = &n
		}
	}
	return q
}

func (s *Server) handleSimTimeline(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	start, _ := strconv.Atoi(c.Query("start_round"))
	var end *int
	if v := c.Query("end_round"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			end = &n
		}
	}
	return okResp(c, s.deps.Sim.Runtime.Timeline(s.reqCtx(c), id, start, end))
}

func (s *Server) handleSimAgentStats(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	return okResp(c, s.deps.Sim.Runtime.AgentStats(s.reqCtx(c), id))
}

func (s *Server) handleSimPosts(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	pl := c.Query("platform", "reddit")
	limit, _ := strconv.Atoi(c.Query("limit"))
	off, _ := strconv.Atoi(c.Query("offset"))
	return okResp(c, s.deps.Sim.Runtime.Posts(s.reqCtx(c), id, pl, limit, off))
}

func (s *Server) handleSimComments(c *fiber.Ctx) error {
	id := c.Params("simulationId")
	pl := c.Query("platform", "reddit")
	postID := c.Query("post_id")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	off, _ := strconv.Atoi(c.Query("offset", "0"))
	return okResp(c, s.deps.Sim.Runtime.Comments(s.reqCtx(c), id, pl, postID, limit, off))
}

func (s *Server) handleSimEnvStatus(c *fiber.Ctx) error {
	var body struct {
		SimulationID string `json:"simulation_id"`
	}
	_ = c.BodyParser(&body)
	if body.SimulationID == "" {
		return failResp(c, fiber.StatusBadRequest, "Please provide simulation_id")
	}
	return okResp(c, s.deps.Sim.Runtime.EnvStatus(s.reqCtx(c), body.SimulationID))
}

func (s *Server) handleSimCloseEnv(c *fiber.Ctx) error {
	var body struct {
		SimulationID string `json:"simulation_id"`
		Timeout      int    `json:"timeout"`
	}
	_ = c.BodyParser(&body)
	if body.SimulationID == "" {
		return failResp(c, fiber.StatusBadRequest, "Please provide simulation_id")
	}
	return okResp(c, s.deps.Sim.Runtime.CloseEnv(s.reqCtx(c), body.SimulationID, body.Timeout))
}

func (s *Server) handleSimInterviewBatch(c *fiber.Ctx) error {
	var body struct {
		SimulationID string           `json:"simulation_id"`
		Interviews   []map[string]any `json:"interviews"`
		Platform     *string          `json:"platform"`
		Timeout      float64          `json:"timeout"`
	}
	if err := c.BodyParser(&body); err != nil || body.SimulationID == "" {
		return failResp(c, fiber.StatusBadRequest, "invalid body")
	}
	if len(body.Interviews) == 0 {
		return failResp(c, fiber.StatusBadRequest, "Please provide interviews")
	}
	for i := range body.Interviews {
		if p, ok := body.Interviews[i]["prompt"].(string); ok {
			body.Interviews[i]["prompt"] = optimizeInterviewPrompt(p)
		}
	}
	res := s.deps.Sim.Runtime.InterviewBatch(s.reqCtx(c), body.SimulationID, body.Interviews, body.Platform, body.Timeout)
	if fmt.Sprint(res["success"]) == "false" {
		return sendJSON(c, fiber.StatusBadRequest, envelope{Success: false, Error: fmt.Sprint(res["error"])})
	}
	return sendJSON(c, fiber.StatusOK, envelope{Success: true, Data: res})
}
