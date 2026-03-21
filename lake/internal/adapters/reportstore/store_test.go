package reportstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mirofish-offline/lake/internal/ports"
)

func TestListReportsFilterAndSort(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	// Older report
	_ = s.EnsureFolder("report_old")
	_ = s.SaveMeta("report_old", map[string]any{
		"report_id": "report_old", "simulation_id": "sim_a", "status": "completed",
		"created_at": "2020-01-01T00:00:00Z",
	})
	// Newer, different simulation
	_ = s.EnsureFolder("report_new")
	_ = s.SaveMeta("report_new", map[string]any{
		"report_id": "report_new", "simulation_id": "sim_b", "status": "completed",
		"created_at": "2025-06-01T00:00:00Z", "markdown_content": "# Hi",
	})
	_ = s.SaveFullMarkdown("report_new", "# Full")

	all, err := s.ListReports(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 reports, got %d", len(all))
	}
	if all[0]["report_id"] != "report_new" {
		t.Fatalf("expected newest first, got %v", all[0]["report_id"])
	}

	filtered, err := s.ListReports(context.Background(), "sim_a", 10)
	if err != nil || len(filtered) != 1 || filtered[0]["report_id"] != "report_old" {
		t.Fatalf("filter: %+v err=%v", filtered, err)
	}
}

func TestListGeneratedSections(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.EnsureFolder("r1")
	_ = s.SaveSection("r1", 2, "Second", "body2")
	_ = s.SaveSection("r1", 1, "First", "body1")
	sec, err := s.ListGeneratedSections("r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sec) != 2 {
		t.Fatalf("sections len=%d", len(sec))
	}
	if sec[0]["section_index"] != 1 || sec[1]["section_index"] != 2 {
		t.Fatalf("order: %#v", sec)
	}
}

func TestDeleteReportNotFound(t *testing.T) {
	t.Parallel()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = s.DeleteReport("nope")
	if err != ports.ErrReportNotFound {
		t.Fatalf("got %v want ErrReportNotFound", err)
	}
}

func TestReadSectionMarkdownMissing(t *testing.T) {
	t.Parallel()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ReadSectionMarkdown("x", 1)
	if err != ports.ErrReportNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestListReportsSkipsInvalidMeta(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(root, "empty_dir"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "not_dir"), []byte("x"), 0o644)
	out, err := s.ListReports(context.Background(), "", 50)
	if err != nil || len(out) != 0 {
		t.Fatalf("list=%v err=%v", out, err)
	}
}
