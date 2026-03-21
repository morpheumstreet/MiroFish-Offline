package domain

// ProjectStatus mirrors backend/app/models/project.py ProjectStatus.
type ProjectStatus string

const (
	StatusCreated           ProjectStatus = "created"
	StatusOntologyGenerated ProjectStatus = "ontology_generated"
	StatusGraphBuilding     ProjectStatus = "graph_building"
	StatusGraphCompleted    ProjectStatus = "graph_completed"
	StatusFailed            ProjectStatus = "failed"
)

// Project is persisted as project.json (same shape as Python to_dict / from_dict).
type Project struct {
	ProjectID             string         `json:"project_id"`
	Name                  string         `json:"name"`
	Status                ProjectStatus  `json:"status"`
	CreatedAt             string         `json:"created_at"`
	UpdatedAt             string         `json:"updated_at"`
	Files                 []FileRef      `json:"files"`
	TotalTextLength       int            `json:"total_text_length"`
	Ontology              map[string]any `json:"ontology,omitempty"`
	AnalysisSummary       string         `json:"analysis_summary,omitempty"`
	GraphID               string         `json:"graph_id,omitempty"`
	GraphBuildTaskID      string         `json:"graph_build_task_id,omitempty"`
	SimulationRequirement string         `json:"simulation_requirement,omitempty"`
	ChunkSize             int            `json:"chunk_size"`
	ChunkOverlap          int            `json:"chunk_overlap"`
	Error                 string         `json:"error,omitempty"`
}

type FileRef struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size,omitempty"`
	Path     string `json:"path,omitempty"`
}
