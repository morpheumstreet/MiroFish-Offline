package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/mirofish-offline/lake/internal/adapters/noop"
	"github.com/mirofish-offline/lake/internal/adapters/openai"
	"github.com/mirofish-offline/lake/internal/adapters/reportstore"
	"github.com/mirofish-offline/lake/internal/adapters/taskstore"
	"github.com/mirofish-offline/lake/internal/app"
	"github.com/mirofish-offline/lake/internal/config"
	"github.com/mirofish-offline/lake/internal/domain"
	"github.com/mirofish-offline/lake/internal/ports"
	"github.com/mirofish-offline/lake/internal/usecase/report"
)

type fakeSimRepo struct{}

var _ ports.SimulationRepository = fakeSimRepo{}

func (fakeSimRepo) SimulationsRoot() string { return "" }

func (fakeSimRepo) EnsureSimulationDir(simulationID string) (string, error) { return "", nil }

func (fakeSimRepo) Create(ctx context.Context, projectID, graphID string, enableTwitter, enableReddit bool) (*domain.SimulationState, error) {
	return nil, domain.ErrNotImplemented
}

func (fakeSimRepo) Load(ctx context.Context, simulationID string) (*domain.SimulationState, error) {
	return nil, domain.ErrNotImplemented
}

func (fakeSimRepo) Save(ctx context.Context, s *domain.SimulationState) error { return nil }

func (fakeSimRepo) List(ctx context.Context, projectID string) ([]*domain.SimulationState, error) {
	return nil, nil
}

func (fakeSimRepo) ListSimulationIDs(ctx context.Context) ([]string, error) { return nil, nil }

func (fakeSimRepo) ReadFile(ctx context.Context, simulationID, rel string) ([]byte, error) {
	return nil, os.ErrNotExist
}

func (fakeSimRepo) WriteFile(ctx context.Context, simulationID, rel string, data []byte) error { return nil }

func (fakeSimRepo) StatFile(ctx context.Context, simulationID, rel string) (time.Time, bool) {
	return time.Time{}, false
}

func (fakeSimRepo) PromotePreparingToReady(ctx context.Context, simulationID string) error { return nil }

type mockGraphTools struct{}

func (mockGraphTools) SearchGraph(ctx context.Context, graphID, query string, limit int) (map[string]any, error) {
	return map[string]any{
		"facts": []string{"test fact"}, "nodes": []any{}, "edges": []any{},
		"query": query, "total_count": 1,
	}, nil
}

func (mockGraphTools) GetGraphStatistics(ctx context.Context, graphID string) (map[string]any, error) {
	return map[string]any{"graph_id": graphID, "node_count": 3, "edge_count": 2}, nil
}

func TestReportHTTPListDownloadSectionsToolsDelete(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rs, err := reportstore.New(root)
	if err != nil {
		t.Fatal(err)
	}
	rid := "report_test123"
	_ = rs.EnsureFolder(rid)
	_ = rs.SaveMeta(rid, map[string]any{
		"report_id": rid, "simulation_id": "sim1", "status": "completed",
		"created_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	_ = rs.SaveProgress(rid, map[string]any{"status": "completed", "progress": 100, "message": "done"})
	_ = rs.SaveSection(rid, 1, "Intro", "Hello **world**")
	_ = rs.SaveFullMarkdown(rid, "# Title\n\nHello")
	_ = rs.AppendAgentLog(rid, map[string]any{"action": "report_complete", "details": map[string]any{}})
	_ = rs.AppendConsoleLog(rid, "[12:00:00] INFO: ok")

	cfg := config.Config{LLMAPIKey: "k", LLMBaseURL: "http://127.0.0.1:9/v1", LLMModelName: "m"}
	deps := &app.Deps{
		Config:     cfg,
		GraphReady: true,
		Entity:     noop.Deps{},
		Tools:      mockGraphTools{},
		Tasks:      taskstore.New(),
		Reports: &report.Service{
			Projects: noop.Deps{},
			Tasks:    taskstore.New(),
			SimRepo:  fakeSimRepo{},
			Repo:     rs,
			Tools:    mockGraphTools{},
			LLM:      openai.New(cfg),
			GraphOK:  true,
		},
	}
	srv := NewServer(deps)
	h := srv.Handler()

	t.Run("list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/report/list", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("list status %d body=%s", rec.Code, rec.Body.String())
		}
		var env struct {
			Success bool           `json:"success"`
			Data    []map[string]any `json:"data"`
			Count   int            `json:"count"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if !env.Success || env.Count != 1 || len(env.Data) != 1 {
			t.Fatalf("list envelope: %+v", env)
		}
	})

	t.Run("sections", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/report/"+rid+"/sections", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
	})

	t.Run("section", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/report/"+rid+"/section/1", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
	})

	t.Run("progress", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/report/"+rid+"/progress", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
	})

	t.Run("download", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/report/"+rid+"/download", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		body := rec.Body.Bytes()
		if len(body) == 0 || rec.Header().Get("Content-Disposition") == "" {
			t.Fatalf("bad download: disp=%q len=%d", rec.Header().Get("Content-Disposition"), len(body))
		}
	})

	t.Run("agent_log_stream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/report/"+rid+"/agent-log/stream", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
	})

	t.Run("tools_search", func(t *testing.T) {
		req := jsonBody(t, "/api/report/tools/search", map[string]any{
			"graph_id": "g1", "query": "q", "limit": 5.0,
		})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("tools search %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tools_statistics", func(t *testing.T) {
		req := jsonBody(t, "/api/report/tools/statistics", map[string]any{
			"graph_id": "g1",
		})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("tools stats %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("delete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/report/"+rid, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		req2 := httptest.NewRequest(http.MethodGet, "/api/report/list", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		var env struct {
			Count int `json:"count"`
		}
		_ = json.Unmarshal(rec2.Body.Bytes(), &env)
		if env.Count != 0 {
			t.Fatalf("after delete count=%d", env.Count)
		}
	})
}

func jsonBody(t *testing.T, path string, v map[string]any) *http.Request {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}
