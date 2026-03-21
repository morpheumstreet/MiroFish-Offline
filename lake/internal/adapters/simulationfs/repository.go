package simulationfs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mirofish-offline/lake/internal/domain"
	"github.com/mirofish-offline/lake/internal/ports"
)

// Repository implements ports.SimulationRepository on disk.
type Repository struct {
	root string
}

func New(root string) (*Repository, error) {
	if root == "" {
		return nil, fmt.Errorf("simulation repository: empty root")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Repository{root: abs}, nil
}

var _ ports.SimulationRepository = (*Repository)(nil)

func (r *Repository) SimulationsRoot() string { return r.root }

func (r *Repository) EnsureSimulationDir(simulationID string) (string, error) {
	dir := filepath.Join(r.root, simulationID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (r *Repository) Create(ctx context.Context, projectID, graphID string, enableTwitter, enableReddit bool) (*domain.SimulationState, error) {
	_ = ctx
	simulationID := "sim_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	now := time.Now().UTC().Format(time.RFC3339Nano)
	st := &domain.SimulationState{
		SimulationID:  simulationID,
		ProjectID:     projectID,
		GraphID:       graphID,
		EnableTwitter: enableTwitter,
		EnableReddit:  enableReddit,
		Status:        "created",
		TwitterStatus: "not_started",
		RedditStatus:  "not_started",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := r.Save(context.Background(), st); err != nil {
		return nil, err
	}
	return st, nil
}

func (r *Repository) Load(ctx context.Context, simulationID string) (*domain.SimulationState, error) {
	_ = ctx
	raw, err := os.ReadFile(filepath.Join(r.root, simulationID, "state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var st domain.SimulationState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (r *Repository) Save(ctx context.Context, s *domain.SimulationState) error {
	_ = ctx
	if s == nil {
		return fmt.Errorf("nil state")
	}
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	dir := filepath.Join(r.root, s.SimulationID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "state.json"), raw, 0o644)
}

func (r *Repository) List(ctx context.Context, projectID string) ([]*domain.SimulationState, error) {
	ids, err := r.ListSimulationIDs(ctx)
	if err != nil {
		return nil, err
	}
	var out []*domain.SimulationState
	for _, id := range ids {
		st, err := r.Load(ctx, id)
		if err != nil || st == nil {
			continue
		}
		if projectID == "" || st.ProjectID == projectID {
			out = append(out, st)
		}
	}
	return out, nil
}

func (r *Repository) ListSimulationIDs(ctx context.Context) ([]string, error) {
	_ = ctx
	entries, err := os.ReadDir(r.root)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(r.root, e.Name(), "state.json")); err == nil {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

func (r *Repository) ReadFile(ctx context.Context, simulationID, rel string) ([]byte, error) {
	_ = ctx
	if strings.Contains(rel, "..") {
		return nil, fmt.Errorf("invalid path")
	}
	return os.ReadFile(filepath.Join(r.root, simulationID, rel))
}

func (r *Repository) WriteFile(ctx context.Context, simulationID, rel string, data []byte) error {
	_ = ctx
	if strings.Contains(rel, "..") {
		return fmt.Errorf("invalid path")
	}
	dir := filepath.Join(r.root, simulationID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, rel), data, 0o644)
}

func (r *Repository) StatFile(ctx context.Context, simulationID, rel string) (time.Time, bool) {
	_ = ctx
	if strings.Contains(rel, "..") {
		return time.Time{}, false
	}
	fi, err := os.Stat(filepath.Join(r.root, simulationID, rel))
	if err != nil {
		return time.Time{}, false
	}
	return fi.ModTime(), true
}

// PromotePreparingToReady updates state.json when artifacts exist but status stuck on preparing.
func (r *Repository) PromotePreparingToReady(ctx context.Context, simulationID string) error {
	st, err := r.Load(ctx, simulationID)
	if err != nil || st == nil || st.Status != "preparing" {
		return err
	}
	prepared, _ := ports.CheckSimulationPrepared(r, simulationID)
	if !prepared {
		return nil
	}
	st.Status = "ready"
	return r.Save(ctx, st)
}
