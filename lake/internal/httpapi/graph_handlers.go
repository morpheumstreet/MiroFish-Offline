package httpapi

import (
	"net/http"
	"strconv"

	"github.com/mirofish-offline/lake/internal/domain"
)

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		fail(w, http.StatusBadRequest, "missing project id")
		return
	}
	ctx := r.Context()
	p, err := s.deps.Projects.GetProject(ctx, id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		fail(w, http.StatusNotFound, "Project does not exist: "+id)
		return
	}
	ok(w, projectToAPI(p))
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	list, err := s.deps.Projects.ListProjects(r.Context(), limit)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows := make([]map[string]any, 0, len(list))
	for i := range list {
		rows = append(rows, projectToAPI(&list[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    rows,
		"count":   len(rows),
	})
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		fail(w, http.StatusBadRequest, "missing project id")
		return
	}
	okDel, err := s.deps.Projects.DeleteProject(r.Context(), id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !okDel {
		fail(w, http.StatusNotFound, "Project does not exist or deletion failed: "+id)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Project deleted: " + id,
	})
}

func (s *Server) handleResetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		fail(w, http.StatusBadRequest, "missing project id")
		return
	}
	ctx := r.Context()
	p, err := s.deps.Projects.GetProject(ctx, id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		fail(w, http.StatusNotFound, "Project does not exist: "+id)
		return
	}
	if len(p.Ontology) > 0 {
		p.Status = domain.StatusOntologyGenerated
	} else {
		p.Status = domain.StatusCreated
	}
	p.GraphID = ""
	p.GraphBuildTaskID = ""
	p.Error = ""
	if err := s.deps.Projects.SaveProject(ctx, p); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Project reset: " + id,
		"data":    projectToAPI(p),
	})
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		fail(w, http.StatusBadRequest, "missing task id")
		return
	}
	t, err := s.deps.Tasks.GetTask(r.Context(), id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if t == nil {
		fail(w, http.StatusNotFound, "Task does not exist: "+id)
		return
	}
	ok(w, taskToAPI(t))
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	list, err := s.deps.Tasks.ListTasks(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows := make([]map[string]any, 0, len(list))
	for i := range list {
		rows = append(rows, taskToAPI(&list[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    rows,
		"count":   len(rows),
	})
}

func projectToAPI(p *domain.Project) map[string]any {
	if p == nil {
		return nil
	}
	return map[string]any{
		"project_id":             p.ProjectID,
		"name":                   p.Name,
		"status":                 string(p.Status),
		"created_at":             p.CreatedAt,
		"updated_at":             p.UpdatedAt,
		"files":                  p.Files,
		"total_text_length":      p.TotalTextLength,
		"ontology":               p.Ontology,
		"analysis_summary":       p.AnalysisSummary,
		"graph_id":               nullIfEmpty(p.GraphID),
		"graph_build_task_id":    nullIfEmpty(p.GraphBuildTaskID),
		"simulation_requirement": nullIfEmpty(p.SimulationRequirement),
		"chunk_size":             p.ChunkSize,
		"chunk_overlap":          p.ChunkOverlap,
		"error":                  nullIfEmpty(p.Error),
	}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func taskToAPI(t *domain.Task) map[string]any {
	if t == nil {
		return nil
	}
	return map[string]any{
		"task_id":         t.TaskID,
		"task_type":       t.TaskType,
		"status":          string(t.Status),
		"created_at":      t.CreatedAt,
		"updated_at":      t.UpdatedAt,
		"progress":        t.Progress,
		"message":         t.Message,
		"progress_detail": t.ProgressDetail,
		"result":          t.Result,
		"error":           nullIfEmpty(t.Error),
		"metadata":        t.Metadata,
	}
}

func (s *Server) handleGetGraphData(w http.ResponseWriter, r *http.Request) {
	gid := r.PathValue("graphId")
	if gid == "" {
		fail(w, http.StatusBadRequest, "missing graph id")
		return
	}
	if !s.deps.GraphReady {
		fail(w, http.StatusServiceUnavailable, "Graph storage not initialized — check Neo4j connection")
		return
	}
	data, err := s.deps.Graph.GetGraphData(r.Context(), gid)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, data)
}

func (s *Server) handleDeleteGraph(w http.ResponseWriter, r *http.Request) {
	gid := r.PathValue("graphId")
	if gid == "" {
		fail(w, http.StatusBadRequest, "missing graph id")
		return
	}
	if !s.deps.GraphReady {
		fail(w, http.StatusServiceUnavailable, "Graph storage not initialized — check Neo4j connection")
		return
	}
	if err := s.deps.Graph.DeleteGraph(r.Context(), gid); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Graph deleted: " + gid,
	})
}
