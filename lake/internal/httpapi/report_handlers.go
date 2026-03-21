package httpapi

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/mirofish-offline/lake/internal/ports"
)

func (s *Server) handleReportGenerate(c *fiber.Ctx) error {
	if !s.requireGraph(c) {
		return nil
	}
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	var body struct {
		SimulationID    string `json:"simulation_id"`
		ForceRegenerate bool   `json:"force_regenerate"`
	}
	if err := c.BodyParser(&body); err != nil {
		return failResp(c, fiber.StatusBadRequest, "invalid JSON body")
	}
	data, err := s.deps.Reports.Generate(s.reqCtx(c), body.SimulationID, body.ForceRegenerate)
	if err != nil {
		return reportErrC(c, err)
	}
	return okResp(c, data)
}

func (s *Server) handleReportGenerateStatusGET(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := strings.TrimSpace(c.Query("report_id"))
	if rid == "" {
		return failResp(c, fiber.StatusBadRequest, "report_id query required")
	}
	data, err := s.deps.Reports.GenerateStatusGET(s.reqCtx(c), rid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			return failResp(c, fiber.StatusNotFound, "report does not exist: "+rid)
		}
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleReportGenerateStatusPOST(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	var body struct {
		TaskID       string `json:"task_id"`
		SimulationID string `json:"simulation_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return failResp(c, fiber.StatusBadRequest, "invalid JSON body")
	}
	data, err := s.deps.Reports.GenerateStatusPOST(s.reqCtx(c), body.TaskID, body.SimulationID)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "does not exist") {
			return failResp(c, fiber.StatusNotFound, msg)
		}
		if strings.Contains(msg, "provide") {
			return failResp(c, fiber.StatusBadRequest, msg)
		}
		return failResp(c, fiber.StatusInternalServerError, msg)
	}
	return okResp(c, data)
}

func (s *Server) handleReportCheck(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	sid := strings.TrimSpace(c.Query("simulation_id"))
	if sid == "" {
		return failResp(c, fiber.StatusBadRequest, "simulation_id query required")
	}
	return okResp(c, s.deps.Reports.CheckBySimulation(s.reqCtx(c), sid))
}

func (s *Server) handleReportBySimulation(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	sid := strings.TrimSpace(c.Query("simulation_id"))
	if sid == "" {
		return failResp(c, fiber.StatusBadRequest, "simulation_id query required")
	}
	data, err := s.deps.Reports.GetBySimulation(s.reqCtx(c), sid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			return sendJSON(c, fiber.StatusNotFound, map[string]any{
				"success":    false,
				"error":      "No report available for this simulation: " + sid,
				"has_report": false,
			})
		}
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleReportGet(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	data, err := s.deps.Reports.GetReport(s.reqCtx(c), rid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			return failResp(c, fiber.StatusNotFound, "report does not exist: "+rid)
		}
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleReportAgentLog(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	from := 0
	if v := c.Query("from_line"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			from = n
		}
	}
	return okResp(c, s.deps.Reports.AgentLog(rid, from))
}

func (s *Server) handleReportConsoleLog(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	from := 0
	if v := c.Query("from_line"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			from = n
		}
	}
	return okResp(c, s.deps.Reports.ConsoleLog(rid, from))
}

func (s *Server) handleReportList(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	simID := strings.TrimSpace(c.Query("simulation_id"))
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	data, err := s.deps.Reports.ListReports(s.reqCtx(c), simID, limit)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okCountResp(c, data, len(data))
}

func (s *Server) handleReportDownload(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	body, err := s.deps.Reports.DownloadMarkdown(rid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			return failResp(c, fiber.StatusNotFound, "report does not exist: "+rid)
		}
		if strings.Contains(err.Error(), "empty report markdown") {
			return failResp(c, fiber.StatusNotFound, "report has no markdown content: "+rid)
		}
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	c.Set("Content-Type", "text/markdown; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, rid))
	return c.Status(fiber.StatusOK).Send(body)
}

func (s *Server) handleReportDelete(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	if err := s.deps.Reports.Delete(rid); err != nil {
		if errors.Is(err, ports.ErrReportNotFound) {
			return failResp(c, fiber.StatusNotFound, "report does not exist: "+rid)
		}
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return sendJSON(c, fiber.StatusOK, map[string]any{"success": true, "message": "Report deleted: " + rid})
}

func (s *Server) handleReportProgress(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	data, err := s.deps.Reports.Progress(s.reqCtx(c), rid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			return failResp(c, fiber.StatusNotFound, "report does not exist: "+rid)
		}
		if strings.Contains(err.Error(), "progress info unavailable") {
			return failResp(c, fiber.StatusNotFound, err.Error())
		}
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleReportSections(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	data, err := s.deps.Reports.ReportSections(s.reqCtx(c), rid)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleReportSection(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	idx, err := strconv.Atoi(c.Params("sectionIndex"))
	if err != nil || idx < 1 {
		return failResp(c, fiber.StatusBadRequest, "invalid section index")
	}
	data, err := s.deps.Reports.ReportSection(rid, idx)
	if err != nil {
		if strings.Contains(err.Error(), "section does not exist") {
			return failResp(c, fiber.StatusNotFound, err.Error())
		}
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleReportAgentLogStream(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	logs, n := s.deps.Reports.AgentLogStream(rid)
	return okResp(c, map[string]any{"logs": logs, "count": n})
}

func (s *Server) handleReportConsoleLogStream(c *fiber.Ctx) error {
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	rid := c.Params("reportId")
	logs, n := s.deps.Reports.ConsoleLogStream(rid)
	return okResp(c, map[string]any{"logs": logs, "count": n})
}

func (s *Server) handleReportToolsSearch(c *fiber.Ctx) error {
	if !s.requireGraph(c) {
		return nil
	}
	if s.deps.Tools == nil {
		return failResp(c, fiber.StatusServiceUnavailable, "graph tools not available")
	}
	var body struct {
		GraphID string  `json:"graph_id"`
		Query   string  `json:"query"`
		Limit   float64 `json:"limit"`
	}
	if err := c.BodyParser(&body); err != nil {
		return failResp(c, fiber.StatusBadRequest, "invalid JSON body")
	}
	if strings.TrimSpace(body.GraphID) == "" || strings.TrimSpace(body.Query) == "" {
		return failResp(c, fiber.StatusBadRequest, "Please provide graph_id and query")
	}
	limit := 10
	if body.Limit > 0 {
		limit = int(body.Limit)
	}
	data, err := s.deps.Tools.SearchGraph(s.reqCtx(c), body.GraphID, body.Query, limit)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleReportToolsStatistics(c *fiber.Ctx) error {
	if !s.requireGraph(c) {
		return nil
	}
	if s.deps.Tools == nil {
		return failResp(c, fiber.StatusServiceUnavailable, "graph tools not available")
	}
	var body struct {
		GraphID string `json:"graph_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return failResp(c, fiber.StatusBadRequest, "invalid JSON body")
	}
	if strings.TrimSpace(body.GraphID) == "" {
		return failResp(c, fiber.StatusBadRequest, "Please provide graph_id")
	}
	data, err := s.deps.Tools.GetGraphStatistics(s.reqCtx(c), body.GraphID)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleReportChat(c *fiber.Ctx) error {
	if !s.requireGraph(c) {
		return nil
	}
	if s.deps.Reports == nil {
		return failResp(c, fiber.StatusInternalServerError, "report service not configured")
	}
	var body struct {
		SimulationID string              `json:"simulation_id"`
		Message      string              `json:"message"`
		ChatHistory  []map[string]string `json:"chat_history"`
	}
	if err := c.BodyParser(&body); err != nil {
		return failResp(c, fiber.StatusBadRequest, "invalid JSON body")
	}
	reply, err := s.deps.Reports.Chat(s.reqCtx(c), body.SimulationID, body.Message, body.ChatHistory)
	if err != nil {
		return reportErrC(c, err)
	}
	return okResp(c, map[string]any{"response": reply, "simulation_id": body.SimulationID})
}

func reportErrC(c *fiber.Ctx, err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "does not exist"):
		return failResp(c, fiber.StatusNotFound, msg)
	case strings.Contains(msg, "required"), strings.Contains(msg, "missing"):
		return failResp(c, fiber.StatusBadRequest, msg)
	case strings.Contains(msg, "not available"):
		return failResp(c, fiber.StatusServiceUnavailable, msg)
	default:
		return failResp(c, fiber.StatusInternalServerError, msg)
	}
}
