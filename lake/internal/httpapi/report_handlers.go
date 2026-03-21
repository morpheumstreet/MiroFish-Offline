package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mirofish-offline/lake/internal/ports"
)

func (s *Server) handleReportGenerate(w http.ResponseWriter, r *http.Request) {
	if !s.requireGraph(w) {
		return
	}
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	var body struct {
		SimulationID    string `json:"simulation_id"`
		ForceRegenerate bool   `json:"force_regenerate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	data, err := s.deps.Reports.Generate(r.Context(), body.SimulationID, body.ForceRegenerate)
	if err != nil {
		reportErr(w, err)
		return
	}
	ok(w, data)
}

func (s *Server) handleReportGenerateStatusGET(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := strings.TrimSpace(r.URL.Query().Get("report_id"))
	if rid == "" {
		fail(w, http.StatusBadRequest, "report_id query required")
		return
	}
	data, err := s.deps.Reports.GenerateStatusGET(r.Context(), rid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			fail(w, http.StatusNotFound, "report does not exist: "+rid)
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleReportGenerateStatusPOST(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	var body struct {
		TaskID       string `json:"task_id"`
		SimulationID string `json:"simulation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	data, err := s.deps.Reports.GenerateStatusPOST(r.Context(), body.TaskID, body.SimulationID)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "does not exist") {
			fail(w, http.StatusNotFound, msg)
			return
		}
		if strings.Contains(msg, "provide") {
			fail(w, http.StatusBadRequest, msg)
			return
		}
		fail(w, http.StatusInternalServerError, msg)
		return
	}
	ok(w, data)
}

func (s *Server) handleReportCheck(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("simulation_id"))
	if sid == "" {
		fail(w, http.StatusBadRequest, "simulation_id query required")
		return
	}
	ok(w, s.deps.Reports.CheckBySimulation(r.Context(), sid))
}

func (s *Server) handleReportBySimulation(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("simulation_id"))
	if sid == "" {
		fail(w, http.StatusBadRequest, "simulation_id query required")
		return
	}
	data, err := s.deps.Reports.GetBySimulation(r.Context(), sid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"success":    false,
				"error":      "No report available for this simulation: " + sid,
				"has_report": false,
			})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleReportGet(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	data, err := s.deps.Reports.GetReport(r.Context(), rid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			fail(w, http.StatusNotFound, "report does not exist: "+rid)
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleReportAgentLog(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	from := 0
	if v := r.URL.Query().Get("from_line"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			from = n
		}
	}
	ok(w, s.deps.Reports.AgentLog(rid, from))
}

func (s *Server) handleReportConsoleLog(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	from := 0
	if v := r.URL.Query().Get("from_line"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			from = n
		}
	}
	ok(w, s.deps.Reports.ConsoleLog(rid, from))
}

func (s *Server) handleReportList(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	simID := strings.TrimSpace(r.URL.Query().Get("simulation_id"))
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	data, err := s.deps.Reports.ListReports(r.Context(), simID, limit)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	okCount(w, data, len(data))
}

func (s *Server) handleReportDownload(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	body, err := s.deps.Reports.DownloadMarkdown(rid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			fail(w, http.StatusNotFound, "report does not exist: "+rid)
			return
		}
		if strings.Contains(err.Error(), "empty report markdown") {
			fail(w, http.StatusNotFound, "report has no markdown content: "+rid)
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, rid))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) handleReportDelete(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	if err := s.deps.Reports.Delete(rid); err != nil {
		if errors.Is(err, ports.ErrReportNotFound) {
			fail(w, http.StatusNotFound, "report does not exist: "+rid)
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Report deleted: " + rid})
}

func (s *Server) handleReportProgress(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	data, err := s.deps.Reports.Progress(r.Context(), rid)
	if err != nil {
		if err == ports.ErrReportNotFound {
			fail(w, http.StatusNotFound, "report does not exist: "+rid)
			return
		}
		if strings.Contains(err.Error(), "progress info unavailable") {
			fail(w, http.StatusNotFound, err.Error())
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleReportSections(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	data, err := s.deps.Reports.ReportSections(r.Context(), rid)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleReportSection(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	idx, err := strconv.Atoi(r.PathValue("sectionIndex"))
	if err != nil || idx < 1 {
		fail(w, http.StatusBadRequest, "invalid section index")
		return
	}
	data, err := s.deps.Reports.ReportSection(rid, idx)
	if err != nil {
		if strings.Contains(err.Error(), "section does not exist") {
			fail(w, http.StatusNotFound, err.Error())
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleReportAgentLogStream(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	logs, n := s.deps.Reports.AgentLogStream(rid)
	ok(w, map[string]any{"logs": logs, "count": n})
}

func (s *Server) handleReportConsoleLogStream(w http.ResponseWriter, r *http.Request) {
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	rid := r.PathValue("reportId")
	logs, n := s.deps.Reports.ConsoleLogStream(rid)
	ok(w, map[string]any{"logs": logs, "count": n})
}

func (s *Server) handleReportToolsSearch(w http.ResponseWriter, r *http.Request) {
	if !s.requireGraph(w) {
		return
	}
	if s.deps.Tools == nil {
		fail(w, http.StatusServiceUnavailable, "graph tools not available")
		return
	}
	var body struct {
		GraphID string  `json:"graph_id"`
		Query   string  `json:"query"`
		Limit   float64 `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.GraphID) == "" || strings.TrimSpace(body.Query) == "" {
		fail(w, http.StatusBadRequest, "Please provide graph_id and query")
		return
	}
	limit := 10
	if body.Limit > 0 {
		limit = int(body.Limit)
	}
	data, err := s.deps.Tools.SearchGraph(r.Context(), body.GraphID, body.Query, limit)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleReportToolsStatistics(w http.ResponseWriter, r *http.Request) {
	if !s.requireGraph(w) {
		return
	}
	if s.deps.Tools == nil {
		fail(w, http.StatusServiceUnavailable, "graph tools not available")
		return
	}
	var body struct {
		GraphID string `json:"graph_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.GraphID) == "" {
		fail(w, http.StatusBadRequest, "Please provide graph_id")
		return
	}
	data, err := s.deps.Tools.GetGraphStatistics(r.Context(), body.GraphID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleReportChat(w http.ResponseWriter, r *http.Request) {
	if !s.requireGraph(w) {
		return
	}
	if s.deps.Reports == nil {
		fail(w, http.StatusInternalServerError, "report service not configured")
		return
	}
	var body struct {
		SimulationID string              `json:"simulation_id"`
		Message      string              `json:"message"`
		ChatHistory  []map[string]string `json:"chat_history"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	reply, err := s.deps.Reports.Chat(r.Context(), body.SimulationID, body.Message, body.ChatHistory)
	if err != nil {
		reportErr(w, err)
		return
	}
	ok(w, map[string]any{"response": reply, "simulation_id": body.SimulationID})
}

func reportErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "does not exist"):
		fail(w, http.StatusNotFound, msg)
	case strings.Contains(msg, "required"), strings.Contains(msg, "missing"):
		fail(w, http.StatusBadRequest, msg)
	case strings.Contains(msg, "not available"):
		fail(w, http.StatusServiceUnavailable, msg)
	default:
		fail(w, http.StatusInternalServerError, msg)
	}
}
