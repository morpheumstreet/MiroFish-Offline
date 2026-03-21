package simrunner

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestQueryComments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "reddit_simulation.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE post (post_id TEXT PRIMARY KEY, created_at TEXT);
		CREATE TABLE comment (
			comment_id INTEGER PRIMARY KEY,
			post_id TEXT,
			content TEXT,
			created_at TEXT
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`INSERT INTO comment (post_id, content, created_at) VALUES ('p1', 'a', '2020-01-02')`)
	_, _ = db.Exec(`INSERT INTO comment (post_id, content, created_at) VALUES ('p2', 'b', '2020-01-03')`)
	_, _ = db.Exec(`INSERT INTO comment (post_id, content, created_at) VALUES ('p1', 'c', '2020-01-04')`)
	db.Close()

	got := QueryComments(dir, "reddit", "", 10, 0)
	if got["count"].(int) != 3 {
		t.Fatalf("count: %v", got["count"])
	}
	comments := got["comments"].([]any)
	if len(comments) != 3 {
		t.Fatalf("len comments %d", len(comments))
	}
	// Newest first: p1 c, p2 b, p1 a
	first := comments[0].(map[string]any)
	if first["content"] != "c" {
		t.Fatalf("order: %#v", first)
	}

	filtered := QueryComments(dir, "reddit", "p2", 10, 0)
	if filtered["count"].(int) != 1 {
		t.Fatalf("filtered count: %v", filtered["count"])
	}
}

func TestQueryComments_noDatabase(t *testing.T) {
	t.Parallel()
	got := QueryComments(t.TempDir(), "reddit", "", 10, 0)
	if got["count"].(int) != 0 {
		t.Fatalf("expected 0, got %v", got["count"])
	}
}
