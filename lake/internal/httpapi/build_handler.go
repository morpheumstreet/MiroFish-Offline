package httpapi

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
	"github.com/mirofish-offline/lake/internal/domain"
	"github.com/mirofish-offline/lake/internal/ports"
)

type buildGraphRequest struct {
	ProjectID    string `json:"project_id"`
	GraphName    string `json:"graph_name"`
	ChunkSize    *int   `json:"chunk_size"`
	ChunkOverlap *int   `json:"chunk_overlap"`
	Force        bool   `json:"force"`
}

func (s *Server) handleGraphBuild(c *fiber.Ctx) error {
	if !s.deps.GraphReady {
		return failResp(c, fiber.StatusServiceUnavailable, "Graph storage not initialized — check Neo4j connection")
	}
	var req buildGraphRequest
	if err := c.BodyParser(&req); err != nil {
		return failResp(c, fiber.StatusBadRequest, "invalid JSON body")
	}
	if req.ProjectID == "" {
		return failResp(c, fiber.StatusBadRequest, "Please provide project_id")
	}
	ctx := s.reqCtx(c)
	proj, err := s.deps.Projects.GetProject(ctx, req.ProjectID)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	if proj == nil {
		return failResp(c, fiber.StatusNotFound, "Project does not exist: "+req.ProjectID)
	}

	if proj.Status == domain.StatusCreated {
		return failResp(c, fiber.StatusBadRequest, "Project has not generated ontology yet. Please call /ontology/generate first")
	}
	if proj.Status == domain.StatusGraphBuilding && !req.Force {
		return sendJSON(c, fiber.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "Graph is being built. Do not submit repeatedly. To force rebuild, add force: true",
			"task_id": nullStr(proj.GraphBuildTaskID),
		})
	}
	if req.Force && (proj.Status == domain.StatusGraphBuilding || proj.Status == domain.StatusFailed || proj.Status == domain.StatusGraphCompleted) {
		proj.Status = domain.StatusOntologyGenerated
		proj.GraphID = ""
		proj.GraphBuildTaskID = ""
		proj.Error = ""
	}

	graphName := req.GraphName
	if graphName == "" {
		graphName = proj.Name
	}
	if graphName == "" {
		graphName = "MiroFish Graph"
	}
	chunkSize := proj.ChunkSize
	if chunkSize == 0 {
		chunkSize = 500
	}
	if req.ChunkSize != nil && *req.ChunkSize > 0 {
		chunkSize = *req.ChunkSize
	}
	overlap := proj.ChunkOverlap
	if overlap == 0 {
		overlap = 50
	}
	if req.ChunkOverlap != nil && *req.ChunkOverlap >= 0 {
		overlap = *req.ChunkOverlap
	}
	proj.ChunkSize = chunkSize
	proj.ChunkOverlap = overlap

	text, err := s.deps.Projects.GetExtractedText(ctx, req.ProjectID)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	if text == "" {
		return failResp(c, fiber.StatusBadRequest, "Extracted text not found")
	}
	if len(proj.Ontology) == 0 {
		return failResp(c, fiber.StatusBadRequest, "Ontology definition not found")
	}

	taskType := fmt.Sprintf("Build graph: %s", graphName)
	taskID, err := s.deps.Tasks.CreateTask(ctx, taskType, map[string]any{
		"project_id": req.ProjectID,
		"graph_name": graphName,
	})
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}

	proj.Status = domain.StatusGraphBuilding
	proj.GraphBuildTaskID = taskID
	if err := s.deps.Projects.SaveProject(ctx, proj); err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}

	projectID := req.ProjectID
	ontology := proj.Ontology
	go s.runGraphBuild(context.Background(), taskID, projectID, graphName, text, ontology, chunkSize, overlap)

	return sendJSON(c, fiber.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"project_id": projectID,
			"task_id":    taskID,
			"message":    "Graph build task started. Query progress via /task/{task_id}",
		},
	})
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Server) runGraphBuild(
	ctx context.Context,
	taskID, projectID, graphName, text string,
	ontology map[string]any,
	chunkSize, overlap int,
) {
	st := domain.TaskProcessing
	p0, p5, p10, p15, p90, p95 := 0, 5, 10, 15, 90, 95
	msg := "Initializing graph build service..."
	_ = s.deps.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{Status: &st, Progress: &p0, Message: &msg})

	msg = "Chunking text..."
	_ = s.deps.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{Status: &st, Progress: &p5, Message: &msg})
	chunks := s.deps.Text.Split(text, chunkSize, overlap)
	totalChunks := len(chunks)

	msg = "Creating Zep graph..."
	_ = s.deps.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{Status: &st, Progress: &p10, Message: &msg})

	graphID, err := s.deps.Graph.CreateGraph(ctx, graphName)
	if err != nil {
		s.failBuild(ctx, taskID, projectID, err)
		return
	}

	proj, err := s.deps.Projects.GetProject(ctx, projectID)
	if err == nil && proj != nil {
		proj.GraphID = graphID
		_ = s.deps.Projects.SaveProject(ctx, proj)
	}

	msg = "Setting ontology definition..."
	_ = s.deps.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{Status: &st, Progress: &p15, Message: &msg})
	if err := s.deps.Graph.SetOntology(ctx, graphID, ontology); err != nil {
		s.failBuild(ctx, taskID, projectID, err)
		return
	}

	msg = fmt.Sprintf("Starting to add %d text chunks...", totalChunks)
	_ = s.deps.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{Status: &st, Progress: &p15, Message: &msg})

	onProgress := func(m string, ratio float64) {
		prog := 15 + int(ratio*40)
		mm := m
		_ = s.deps.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{Status: &st, Progress: &prog, Message: &mm})
	}
	_, err = s.deps.Graph.AddTextBatches(ctx, graphID, chunks, 3, onProgress)
	if err != nil {
		s.failBuild(ctx, taskID, projectID, err)
		return
	}

	msg = "Text processing completed, generating graph data..."
	_ = s.deps.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{Status: &st, Progress: &p90, Message: &msg})

	msg = "Retrieving graph data..."
	_ = s.deps.Tasks.UpdateTask(ctx, taskID, ports.TaskPatch{Status: &st, Progress: &p95, Message: &msg})

	graphData, err := s.deps.Graph.GetGraphData(ctx, graphID)
	if err != nil {
		s.failBuild(ctx, taskID, projectID, err)
		return
	}

	proj, err = s.deps.Projects.GetProject(ctx, projectID)
	if err == nil && proj != nil {
		proj.Status = domain.StatusGraphCompleted
		proj.Error = ""
		_ = s.deps.Projects.SaveProject(ctx, proj)
	}

	nodeCount := intFromAny(graphData["node_count"])
	edgeCount := intFromAny(graphData["edge_count"])
	_ = s.deps.Tasks.CompleteTask(ctx, taskID, map[string]any{
		"project_id":  projectID,
		"graph_id":    graphID,
		"node_count":  nodeCount,
		"edge_count":  edgeCount,
		"chunk_count": totalChunks,
	})
}

func (s *Server) failBuild(ctx context.Context, taskID, projectID string, err error) {
	em := err.Error() + "\n" + string(debug.Stack())
	_ = s.deps.Tasks.FailTask(ctx, taskID, em)
	proj, gerr := s.deps.Projects.GetProject(ctx, projectID)
	if gerr == nil && proj != nil {
		proj.Status = domain.StatusFailed
		proj.Error = err.Error()
		_ = s.deps.Projects.SaveProject(ctx, proj)
	}
}

func intFromAny(v any) int {
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
