package noop

import (
	"context"
	"io"

	"github.com/mirofish-offline/lake/internal/domain"
	"github.com/mirofish-offline/lake/internal/ports"
)

// Deps bundles stub adapters so the process boots while Python parity is ported.
type Deps struct{}

func (Deps) Ping(ctx context.Context) error {
	_ = ctx
	return nil
}

func (Deps) CreateProject(ctx context.Context, name string) (*domain.Project, error) {
	_ = ctx
	_ = name
	return nil, domain.ErrNotImplemented
}

func (Deps) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	_ = ctx
	_ = id
	return nil, domain.ErrNotImplemented
}

func (Deps) ListProjects(ctx context.Context, limit int) ([]domain.Project, error) {
	_ = ctx
	_ = limit
	return nil, domain.ErrNotImplemented
}

func (Deps) SaveProject(ctx context.Context, p *domain.Project) error {
	_ = ctx
	_ = p
	return domain.ErrNotImplemented
}

func (Deps) DeleteProject(ctx context.Context, id string) (bool, error) {
	_ = ctx
	_ = id
	return false, domain.ErrNotImplemented
}

func (Deps) SaveUploadedFile(ctx context.Context, projectID, originalName string, r io.Reader, size int64) (*ports.UploadedFileResult, error) {
	_ = ctx
	_ = projectID
	_ = originalName
	_ = r
	_ = size
	return nil, domain.ErrNotImplemented
}

func (Deps) SaveExtractedText(ctx context.Context, projectID, text string) error {
	_ = ctx
	_ = projectID
	_ = text
	return domain.ErrNotImplemented
}

func (Deps) GetExtractedText(ctx context.Context, projectID string) (string, error) {
	_ = ctx
	_ = projectID
	return "", domain.ErrNotImplemented
}

func (Deps) CreateTask(ctx context.Context, taskType string, metadata map[string]any) (string, error) {
	_ = ctx
	_ = taskType
	_ = metadata
	return "", domain.ErrNotImplemented
}

func (Deps) GetTask(ctx context.Context, taskID string) (*domain.Task, error) {
	_ = ctx
	_ = taskID
	return nil, domain.ErrNotImplemented
}

func (Deps) ListTasks(ctx context.Context) ([]domain.Task, error) {
	_ = ctx
	return nil, domain.ErrNotImplemented
}

func (Deps) UpdateTask(ctx context.Context, taskID string, patch ports.TaskPatch) error {
	_ = ctx
	_ = taskID
	_ = patch
	return domain.ErrNotImplemented
}

func (Deps) CompleteTask(ctx context.Context, taskID string, result map[string]any) error {
	_ = ctx
	_ = taskID
	_ = result
	return domain.ErrNotImplemented
}

func (Deps) FailTask(ctx context.Context, taskID string, errMsg string) error {
	_ = ctx
	_ = taskID
	_ = errMsg
	return domain.ErrNotImplemented
}

func (Deps) Preprocess(text string) string { return text }

func (Deps) Split(text string, chunkSize, overlap int) []string {
	_ = chunkSize
	_ = overlap
	if text == "" {
		return nil
	}
	return []string{text}
}

func (Deps) ExtractText(path string) (string, error) {
	_ = path
	return "", domain.ErrNotImplemented
}

func (Deps) Generate(ctx context.Context, documentTexts []string, simulationRequirement string, additionalContext *string) (map[string]any, error) {
	_ = ctx
	_ = documentTexts
	_ = simulationRequirement
	_ = additionalContext
	return nil, domain.ErrNotImplemented
}

func (Deps) CreateGraph(ctx context.Context, name string) (string, error) {
	_ = ctx
	_ = name
	return "", domain.ErrNotImplemented
}

func (Deps) SetOntology(ctx context.Context, graphID string, ontology map[string]any) error {
	_ = ctx
	_ = graphID
	_ = ontology
	return domain.ErrNotImplemented
}

func (Deps) AddTextBatches(ctx context.Context, graphID string, chunks []string, batchSize int, onProgress func(msg string, ratio float64)) ([]string, error) {
	_ = ctx
	_ = graphID
	_ = chunks
	_ = batchSize
	_ = onProgress
	return nil, domain.ErrNotImplemented
}

func (Deps) GetGraphData(ctx context.Context, graphID string) (map[string]any, error) {
	_ = ctx
	_ = graphID
	return nil, domain.ErrNotImplemented
}

func (Deps) DeleteGraph(ctx context.Context, graphID string) error {
	_ = ctx
	_ = graphID
	return domain.ErrNotImplemented
}

func (Deps) FilterDefinedEntities(ctx context.Context, graphID string, entityTypes []string, enrich bool) (map[string]any, error) {
	_ = ctx
	_ = graphID
	_ = entityTypes
	_ = enrich
	return nil, domain.ErrNotImplemented
}

func (Deps) GetEntityWithContext(ctx context.Context, graphID, entityUUID string) (map[string]any, error) {
	_ = ctx
	_ = graphID
	_ = entityUUID
	return nil, domain.ErrNotImplemented
}

func (Deps) GetEntitiesByType(ctx context.Context, graphID, entityType string, enrich bool) ([]map[string]any, error) {
	_ = ctx
	_ = graphID
	_ = entityType
	_ = enrich
	return nil, domain.ErrNotImplemented
}

func (Deps) Placeholder(ctx context.Context) error {
	_ = ctx
	return domain.ErrNotImplemented
}

func (Deps) SearchGraph(ctx context.Context, graphID, query string, limit int) (map[string]any, error) {
	_ = ctx
	_ = graphID
	_ = query
	_ = limit
	return nil, domain.ErrNotImplemented
}

func (Deps) GetGraphStatistics(ctx context.Context, graphID string) (map[string]any, error) {
	_ = ctx
	_ = graphID
	return nil, domain.ErrNotImplemented
}
