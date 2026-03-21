package report

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mirofish-offline/lake/internal/adapters/openai"
	"github.com/mirofish-offline/lake/internal/domain"
	"github.com/mirofish-offline/lake/internal/ports"
)

// Service orchestrates report generation and chat (Go counterpart to ReportAgent + ReportManager routes).
type Service struct {
	Projects ports.ProjectRepository
	Tasks    ports.TaskRepository
	SimRepo  ports.SimulationRepository
	Repo     ports.ReportRepository
	Tools    ports.GraphTools
	LLM      *openai.Client
	GraphOK  bool
}

func newReportID() string {
	s := strings.ReplaceAll(uuid.New().String(), "-", "")
	return "report_" + s[:12]
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// Generate starts background generation (Flask-compatible response shape).
func (s *Service) Generate(ctx context.Context, simulationID string, forceRegenerate bool) (map[string]any, error) {
	if simulationID == "" {
		return nil, fmt.Errorf("simulation_id required")
	}
	if !s.GraphOK || s.Tools == nil {
		return nil, fmt.Errorf("graph storage not available")
	}
	st, err := s.SimRepo.Load(ctx, simulationID)
	if err != nil || st == nil {
		return nil, fmt.Errorf("simulation does not exist: %s", simulationID)
	}
	if !forceRegenerate {
		rid, meta, err := s.Repo.LatestReportBySimulation(ctx, simulationID)
		if err == nil && meta != nil && asString(meta["status"]) == "completed" {
			return map[string]any{
				"simulation_id":     simulationID,
				"report_id":         rid,
				"status":            "completed",
				"message":           "Report already exists",
				"already_generated": true,
			}, nil
		}
	}
	p, err := s.Projects.GetProject(ctx, st.ProjectID)
	if err != nil || p == nil {
		return nil, fmt.Errorf("project does not exist: %s", st.ProjectID)
	}
	graphID := st.GraphID
	if graphID == "" {
		graphID = p.GraphID
	}
	if graphID == "" {
		return nil, fmt.Errorf("missing graph ID, please ensure graph is built")
	}
	req := strings.TrimSpace(p.SimulationRequirement)
	if req == "" {
		return nil, fmt.Errorf("missing simulation requirement description")
	}
	reportID := newReportID()
	taskID, err := s.Tasks.CreateTask(ctx, "report_generate", map[string]any{
		"simulation_id": simulationID,
		"graph_id":      graphID,
		"report_id":     reportID,
	})
	if err != nil {
		return nil, err
	}
	go s.runGenerate(context.Background(), runParams{
		simulationID: simulationID,
		graphID:      graphID,
		reportID:     reportID,
		taskID:       taskID,
		requirement:  req,
	})
	return map[string]any{
		"simulation_id":     simulationID,
		"report_id":         reportID,
		"task_id":           taskID,
		"status":            "generating",
		"message":           "Report generation task started. Query progress via /api/report/generate/status",
		"already_generated": false,
	}, nil
}

func (s *Service) patchTask(taskID string, pct int, msg string) {
	ctx := context.Background()
	st := domain.TaskProcessing
	_ = s.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{
		Status:   &st,
		Progress: &pct,
		Message:  &msg,
	})
}

// GetReport returns meta.json payload merged with markdown from disk when needed.
func (s *Service) GetReport(ctx context.Context, reportID string) (map[string]any, error) {
	meta, err := s.Repo.LoadMeta(reportID)
	if err != nil {
		return nil, err
	}
	md := asString(meta["markdown_content"])
	if strings.TrimSpace(md) == "" {
		full, _ := s.Repo.LoadFullMarkdown(reportID)
		meta["markdown_content"] = full
	}
	return meta, nil
}

// AgentLog proxies ReportManager.get_agent_log.
func (s *Service) AgentLog(reportID string, fromLine int) map[string]any {
	logs, total, from, hasMore := s.Repo.ReadAgentLog(reportID, fromLine)
	return map[string]any{
		"logs":        logs,
		"total_lines": total,
		"from_line":   from,
		"has_more":    hasMore,
	}
}

// ConsoleLog proxies ReportManager.get_console_log.
func (s *Service) ConsoleLog(reportID string, fromLine int) map[string]any {
	logs, total, from, hasMore := s.Repo.ReadConsoleLog(reportID, fromLine)
	return map[string]any{
		"logs":        logs,
		"total_lines": total,
		"from_line":   from,
		"has_more":    hasMore,
	}
}

// GenerateStatusGET merges progress.json and meta for the fishtank GET ?report_id= poller.
func (s *Service) GenerateStatusGET(ctx context.Context, reportID string) (map[string]any, error) {
	meta, err := s.Repo.LoadMeta(reportID)
	if err != nil {
		return nil, err
	}
	prog, _ := s.Repo.LoadProgress(reportID)
	out := map[string]any{
		"report_id":     reportID,
		"simulation_id": meta["simulation_id"],
		"status":        meta["status"],
		"progress":      0,
		"message":       "",
	}
	if prog != nil {
		out["progress"] = coerceInt(prog["progress"])
		out["message"] = asString(prog["message"])
		if prog["status"] != nil {
			out["status"] = prog["status"]
		}
	}
	if asString(meta["status"]) == "completed" {
		out["progress"] = 100
		out["status"] = "completed"
	}
	if asString(meta["status"]) == "failed" {
		out["status"] = "failed"
		out["error"] = meta["error"]
	}
	_ = ctx
	return out, nil
}

// GenerateStatusPOST matches Flask POST /generate/status (task_id or simulation_id).
func (s *Service) GenerateStatusPOST(ctx context.Context, taskID, simulationID string) (map[string]any, error) {
	if simulationID != "" {
		rid, meta, err := s.Repo.LatestReportBySimulation(ctx, simulationID)
		if err == nil && meta != nil && asString(meta["status"]) == "completed" {
			return map[string]any{
				"simulation_id":     simulationID,
				"report_id":         rid,
				"status":            "completed",
				"progress":          100,
				"message":           "Report generated",
				"already_completed": true,
			}, nil
		}
	}
	if taskID == "" {
		return nil, fmt.Errorf("please provide task_id or simulation_id")
	}
	t, err := s.Tasks.GetTask(ctx, taskID)
	if err != nil || t == nil {
		return nil, fmt.Errorf("task does not exist: %s", taskID)
	}
	return taskToMap(t), nil
}

func taskToMap(t *domain.Task) map[string]any {
	b, _ := json.Marshal(t)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func coerceInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

// Chat answers using stored report text + graph stats (string response for Step5Interaction).
func (s *Service) Chat(ctx context.Context, simulationID, userMsg string, history []map[string]string) (string, error) {
	if simulationID == "" || strings.TrimSpace(userMsg) == "" {
		return "", fmt.Errorf("simulation_id and message required")
	}
	if !s.GraphOK || s.Tools == nil {
		return "", fmt.Errorf("graph storage not available")
	}
	st, err := s.SimRepo.Load(ctx, simulationID)
	if err != nil || st == nil {
		return "", fmt.Errorf("simulation does not exist: %s", simulationID)
	}
	p, err := s.Projects.GetProject(ctx, st.ProjectID)
	if err != nil || p == nil {
		return "", fmt.Errorf("project does not exist: %s", st.ProjectID)
	}
	graphID := st.GraphID
	if graphID == "" {
		graphID = p.GraphID
	}
	req := strings.TrimSpace(p.SimulationRequirement)

	reportMD := ""
	if rid, _, err := s.Repo.LatestReportBySimulation(ctx, simulationID); err == nil && rid != "" {
		meta, _ := s.Repo.LoadMeta(rid)
		if meta != nil {
			reportMD = asString(meta["markdown_content"])
		}
		if len(reportMD) < 80 {
			full, _ := s.Repo.LoadFullMarkdown(rid)
			if full != "" {
				reportMD = full
			}
		}
	}
	if len(reportMD) > 15000 {
		reportMD = reportMD[:15000] + "\n\n... [truncated] ..."
	}
	if reportMD == "" {
		reportMD = "（no report）"
	}

	stats, err := s.Tools.GetGraphStatistics(ctx, graphID)
	statsLine := ""
	if err == nil && stats != nil {
		b, _ := json.Marshal(stats)
		statsLine = string(b)
	}

	system := fmt.Sprintf(`You are an assistant for a social simulation project.
Simulation requirement:
%s

Generated report (may be incomplete if still running):
%s

Graph statistics (JSON): %s

Answer the user clearly and concisely in markdown when helpful.`, req, reportMD, statsLine)

	var msgs []openai.ChatMessage
	msgs = append(msgs, openai.ChatMessage{Role: "system", Content: system})
	n := len(history)
	if n > 10 {
		history = history[n-10:]
	}
	for _, h := range history {
		role := h["role"]
		content := h["content"]
		if (role == "user" || role == "assistant") && content != "" {
			msgs = append(msgs, openai.ChatMessage{Role: role, Content: content})
		}
	}
	msgs = append(msgs, openai.ChatMessage{Role: "user", Content: userMsg})
	return s.LLM.ChatText(ctx, msgs, 0.5, 2048)
}

func (s *Service) console(reportID, msg string) {
	line := fmt.Sprintf("[%s] INFO: %s", time.Now().Format("15:04:05"), msg)
	_ = s.Repo.AppendConsoleLog(reportID, line)
}
