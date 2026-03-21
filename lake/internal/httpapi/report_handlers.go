package httpapi

import (
	"encoding/json"
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
