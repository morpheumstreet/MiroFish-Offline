package httpapi

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/mirofish-offline/lake/internal/domain"
)

func (s *Server) handleGetProject(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return failResp(c, fiber.StatusBadRequest, "missing project id")
	}
	ctx := s.reqCtx(c)
	p, err := s.deps.Projects.GetProject(ctx, id)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	if p == nil {
		return failResp(c, fiber.StatusNotFound, "Project does not exist: "+id)
	}
	return okResp(c, projectToAPI(p))
}

func (s *Server) handleListProjects(c *fiber.Ctx) error {
	limit := 50
	if q := c.Query("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	list, err := s.deps.Projects.ListProjects(s.reqCtx(c), limit)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	rows := make([]map[string]any, 0, len(list))
	for i := range list {
		rows = append(rows, projectToAPI(&list[i]))
	}
	return sendJSON(c, fiber.StatusOK, map[string]any{
		"success": true,
		"data":    rows,
		"count":   len(rows),
	})
}

func (s *Server) handleDeleteProject(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return failResp(c, fiber.StatusBadRequest, "missing project id")
	}
	okDel, err := s.deps.Projects.DeleteProject(s.reqCtx(c), id)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	if !okDel {
		return failResp(c, fiber.StatusNotFound, "Project does not exist or deletion failed: "+id)
	}
	return sendJSON(c, fiber.StatusOK, map[string]any{
		"success": true,
		"message": "Project deleted: " + id,
	})
}

func (s *Server) handleResetProject(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return failResp(c, fiber.StatusBadRequest, "missing project id")
	}
	ctx := s.reqCtx(c)
	p, err := s.deps.Projects.GetProject(ctx, id)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	if p == nil {
		return failResp(c, fiber.StatusNotFound, "Project does not exist: "+id)
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
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return sendJSON(c, fiber.StatusOK, map[string]any{
		"success": true,
		"message": "Project reset: " + id,
		"data":    projectToAPI(p),
	})
}

func (s *Server) handleGetTask(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return failResp(c, fiber.StatusBadRequest, "missing task id")
	}
	t, err := s.deps.Tasks.GetTask(s.reqCtx(c), id)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	if t == nil {
		return failResp(c, fiber.StatusNotFound, "Task does not exist: "+id)
	}
	return okResp(c, taskToAPI(t))
}

func (s *Server) handleListTasks(c *fiber.Ctx) error {
	list, err := s.deps.Tasks.ListTasks(s.reqCtx(c))
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	rows := make([]map[string]any, 0, len(list))
	for i := range list {
		rows = append(rows, taskToAPI(&list[i]))
	}
	return sendJSON(c, fiber.StatusOK, map[string]any{
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

func (s *Server) handleGetGraphData(c *fiber.Ctx) error {
	gid := c.Params("graphId")
	if gid == "" {
		return failResp(c, fiber.StatusBadRequest, "missing graph id")
	}
	if !s.deps.GraphReady {
		return failResp(c, fiber.StatusServiceUnavailable, "Graph storage not initialized — check Neo4j connection")
	}
	data, err := s.deps.Graph.GetGraphData(s.reqCtx(c), gid)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return okResp(c, data)
}

func (s *Server) handleDeleteGraph(c *fiber.Ctx) error {
	gid := c.Params("graphId")
	if gid == "" {
		return failResp(c, fiber.StatusBadRequest, "missing graph id")
	}
	if !s.deps.GraphReady {
		return failResp(c, fiber.StatusServiceUnavailable, "Graph storage not initialized — check Neo4j connection")
	}
	if err := s.deps.Graph.DeleteGraph(s.reqCtx(c), gid); err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	return sendJSON(c, fiber.StatusOK, map[string]any{
		"success": true,
		"message": "Graph deleted: " + gid,
	})
}
