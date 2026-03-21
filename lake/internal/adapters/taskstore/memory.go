package taskstore

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mirofish-offline/lake/internal/domain"
	"github.com/mirofish-offline/lake/internal/ports"
)

// Memory is an in-process task store (matches Python TaskManager singleton semantics per process).
type Memory struct {
	mu    sync.RWMutex
	tasks map[string]*domain.Task
}

func New() *Memory {
	return &Memory{tasks: make(map[string]*domain.Task)}
}

func (m *Memory) CreateTask(ctx context.Context, taskType string, metadata map[string]any) (string, error) {
	_ = ctx
	id := uuid.NewString()
	now := time.Now().Format(time.RFC3339Nano)
	t := &domain.Task{
		TaskID:    id,
		TaskType:  taskType,
		Status:    domain.TaskPending,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  cloneMeta(metadata),
	}
	m.mu.Lock()
	m.tasks[id] = t
	m.mu.Unlock()
	return id, nil
}

func cloneMeta(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (m *Memory) GetTask(ctx context.Context, taskID string) (*domain.Task, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	t := m.tasks[taskID]
	if t == nil {
		return nil, nil
	}
	cp := *t
	return &cp, nil
}

func (m *Memory) ListTasks(ctx context.Context) ([]domain.Task, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]domain.Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		cp := *t
		list = append(list, cp)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt > list[j].CreatedAt
	})
	return list, nil
}

func (m *Memory) UpdateTask(ctx context.Context, taskID string, patch ports.TaskPatch) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tasks[taskID]
	if t == nil {
		return domain.ErrNotFound
	}
	t.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	if patch.Status != nil {
		t.Status = *patch.Status
	}
	if patch.Progress != nil {
		t.Progress = *patch.Progress
	}
	if patch.Message != nil {
		t.Message = *patch.Message
	}
	if patch.ProgressDetail != nil {
		t.ProgressDetail = patch.ProgressDetail
	}
	if patch.Result != nil {
		t.Result = patch.Result
	}
	if patch.Error != nil {
		t.Error = *patch.Error
	}
	return nil
}

func (m *Memory) CompleteTask(ctx context.Context, taskID string, result map[string]any) error {
	msg := "Task completed"
	prog := 100
	st := domain.TaskCompleted
	return m.UpdateTask(ctx, taskID, ports.TaskPatch{
		Status:   &st,
		Progress: &prog,
		Message:  &msg,
		Result:   result,
	})
}

func (m *Memory) FailTask(ctx context.Context, taskID string, errMsg string) error {
	msg := "Task failed"
	st := domain.TaskFailed
	return m.UpdateTask(ctx, taskID, ports.TaskPatch{
		Status:  &st,
		Message: &msg,
		Error:   &errMsg,
	})
}
