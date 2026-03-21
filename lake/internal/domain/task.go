package domain

// TaskStatus mirrors backend/app/models/task.py TaskStatus.
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskProcessing TaskStatus = "processing"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
)

type Task struct {
	TaskID         string         `json:"task_id"`
	TaskType       string         `json:"task_type"`
	Status         TaskStatus     `json:"status"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
	Progress       int            `json:"progress"`
	Message        string         `json:"message"`
	ProgressDetail map[string]any `json:"progress_detail,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
	Error          string         `json:"error,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}
