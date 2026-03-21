package ports

import (
	"context"
	"io"

	"github.com/mirofish-offline/lake/internal/domain"
)

// UploadedFileResult matches Python save_file_to_project return shape used by the API.
type UploadedFileResult struct {
	Path             string
	OriginalFilename string
	SavedFilename    string
	Size             int64
}

// --- Cross-cutting ---

// Neo4jHealth optional connectivity check for /health.
type Neo4jHealth interface {
	Ping(ctx context.Context) error
}

// --- Graph / ontology / build (backend: graph routes + graph_builder + ontology_generator) ---

type ProjectRepository interface {
	CreateProject(ctx context.Context, name string) (*domain.Project, error)
	GetProject(ctx context.Context, id string) (*domain.Project, error)
	ListProjects(ctx context.Context, limit int) ([]domain.Project, error)
	SaveProject(ctx context.Context, p *domain.Project) error
	DeleteProject(ctx context.Context, id string) (bool, error)
	SaveUploadedFile(ctx context.Context, projectID, originalName string, r io.Reader, size int64) (*UploadedFileResult, error)
	SaveExtractedText(ctx context.Context, projectID, text string) error
	GetExtractedText(ctx context.Context, projectID string) (string, error)
}

type TaskRepository interface {
	CreateTask(ctx context.Context, taskType string, metadata map[string]any) (taskID string, err error)
	GetTask(ctx context.Context, taskID string) (*domain.Task, error)
	ListTasks(ctx context.Context) ([]domain.Task, error)
	UpdateTask(ctx context.Context, taskID string, patch TaskPatch) error
	CompleteTask(ctx context.Context, taskID string, result map[string]any) error
	FailTask(ctx context.Context, taskID string, errMsg string) error
}

type TaskPatch struct {
	Status         *domain.TaskStatus
	Progress       *int
	Message        *string
	ProgressDetail map[string]any
	Result         map[string]any
	Error          *string
}

type TextProcessor interface {
	Preprocess(text string) string
	Split(text string, chunkSize, overlap int) []string
}

type FileParser interface {
	ExtractText(path string) (string, error)
}

type OntologyGenerator interface {
	Generate(ctx context.Context, documentTexts []string, simulationRequirement string, additionalContext *string) (ontology map[string]any, err error)
}

type GraphBuilder interface {
	CreateGraph(ctx context.Context, name string) (graphID string, err error)
	SetOntology(ctx context.Context, graphID string, ontology map[string]any) error
	AddTextBatches(ctx context.Context, graphID string, chunks []string, batchSize int, onProgress func(msg string, ratio float64)) ([]string, error)
	GetGraphData(ctx context.Context, graphID string) (map[string]any, error)
	DeleteGraph(ctx context.Context, graphID string) error
}

// --- Simulation (entity graph reads; orchestration in usecase/simulation, runtime in adapters/simrunner) ---

type EntityReader interface {
	FilterDefinedEntities(ctx context.Context, graphID string, entityTypes []string, enrich bool) (map[string]any, error)
	GetEntityWithContext(ctx context.Context, graphID, entityUUID string) (map[string]any, error)
	GetEntitiesByType(ctx context.Context, graphID, entityType string, enrich bool) ([]map[string]any, error)
}

// --- Report (backend: report routes + report_agent + graph_tools) ---

type GraphTools interface {
	SearchGraph(ctx context.Context, graphID, query string, limit int) (map[string]any, error)
	GetGraphStatistics(ctx context.Context, graphID string) (map[string]any, error)
}
