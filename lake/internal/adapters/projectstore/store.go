package projectstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirofish-offline/lake/internal/domain"
	"github.com/mirofish-offline/lake/internal/ports"
)

// Store persists projects under {uploadRoot}/projects/{project_id}/ (Python-compatible layout).
type Store struct {
	uploadRoot  string
	projectsDir string
}

func New(uploadRoot string) (*Store, error) {
	abs, err := filepath.Abs(uploadRoot)
	if err != nil {
		return nil, fmt.Errorf("upload root: %w", err)
	}
	s := &Store{
		uploadRoot:  abs,
		projectsDir: filepath.Join(abs, "projects"),
	}
	if err := os.MkdirAll(s.projectsDir, 0o755); err != nil {
		return nil, fmt.Errorf("projects dir: %w", err)
	}
	return s, nil
}

func (s *Store) projectDir(id string) string {
	return filepath.Join(s.projectsDir, id)
}

func (s *Store) metaPath(id string) string {
	return filepath.Join(s.projectDir(id), "project.json")
}

func (s *Store) filesDir(id string) string {
	return filepath.Join(s.projectDir(id), "files")
}

func (s *Store) textPath(id string) string {
	return filepath.Join(s.projectDir(id), "extracted_text.txt")
}

func randomProjID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "proj_" + hex.EncodeToString(b), nil
}

func (s *Store) CreateProject(ctx context.Context, name string) (*domain.Project, error) {
	_ = ctx
	id, err := randomProjID()
	if err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339Nano)
	p := &domain.Project{
		ProjectID:    id,
		Name:         name,
		Status:       domain.StatusCreated,
		CreatedAt:    now,
		UpdatedAt:    now,
		Files:        []domain.FileRef{},
		ChunkSize:    500,
		ChunkOverlap: 50,
	}
	dir := s.projectDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.filesDir(id), 0o755); err != nil {
		return nil, err
	}
	if err := s.writeMeta(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	_ = ctx
	data, err := os.ReadFile(s.metaPath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var p domain.Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.ChunkSize == 0 {
		p.ChunkSize = 500
	}
	if p.ChunkOverlap == 0 {
		p.ChunkOverlap = 50
	}
	if p.Files == nil {
		p.Files = []domain.FileRef{}
	}
	return &p, nil
}

func (s *Store) ListProjects(ctx context.Context, limit int) ([]domain.Project, error) {
	_ = ctx
	if limit <= 0 {
		limit = 50
	}
	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []domain.Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := s.GetProject(ctx, e.Name())
		if err != nil || p == nil {
			continue
		}
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.Compare(out[i].CreatedAt, out[j].CreatedAt) > 0
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) SaveProject(ctx context.Context, p *domain.Project) error {
	_ = ctx
	if p == nil {
		return errors.New("nil project")
	}
	p.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	return s.writeMeta(p)
}

func (s *Store) writeMeta(p *domain.Project) error {
	if err := os.MkdirAll(s.projectDir(p.ProjectID), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.metaPath(p.ProjectID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.metaPath(p.ProjectID))
}

func (s *Store) DeleteProject(ctx context.Context, id string) (bool, error) {
	_ = ctx
	dir := s.projectDir(id)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, os.RemoveAll(dir)
}

func (s *Store) SaveUploadedFile(ctx context.Context, projectID, originalName string, r io.Reader, size int64) (*ports.UploadedFileResult, error) {
	_ = ctx
	_ = size
	if err := os.MkdirAll(s.filesDir(projectID), 0o755); err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	var rand6 [6]byte
	if _, err := rand.Read(rand6[:]); err != nil {
		return nil, err
	}
	safeName := hex.EncodeToString(rand6[:]) + ext
	dest := filepath.Join(s.filesDir(projectID), safeName)
	f, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(f, r)
	_ = f.Close()
	if err != nil {
		_ = os.Remove(dest)
		return nil, err
	}
	abs, err := filepath.Abs(dest)
	if err != nil {
		abs = dest
	}
	return &ports.UploadedFileResult{
		Path:             abs,
		OriginalFilename: originalName,
		SavedFilename:    safeName,
		Size:             n,
	}, nil
}

func (s *Store) SaveExtractedText(ctx context.Context, projectID, text string) error {
	_ = ctx
	if err := os.MkdirAll(s.projectDir(projectID), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.textPath(projectID), []byte(text), 0o644)
}

func (s *Store) GetExtractedText(ctx context.Context, projectID string) (string, error) {
	_ = ctx
	b, err := os.ReadFile(s.textPath(projectID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}
