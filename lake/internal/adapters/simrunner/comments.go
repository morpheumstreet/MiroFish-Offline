package simrunner

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// QueryComments reads OASIS sqlite comment table (Reddit DB; Flask: GET .../comments).
func QueryComments(simDir, platform, postID string, limit, offset int) map[string]any {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	dbName := platform + "_simulation.db"
	dbPath := filepath.Join(simDir, dbName)
	if _, err := os.Stat(dbPath); err != nil {
		return map[string]any{"count": 0, "comments": []any{}}
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		return map[string]any{"count": 0, "comments": []any{}, "error": err.Error()}
	}
	defer db.Close()

	var rows *sql.Rows
	if postID != "" {
		rows, err = db.Query(`
			SELECT * FROM comment
			WHERE post_id = ?
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?`, postID, limit, offset)
	} else {
		rows, err = db.Query(`
			SELECT * FROM comment
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return map[string]any{"count": 0, "comments": []any{}}
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	var comments []any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		row := map[string]any{}
		for i, c := range cols {
			row[c] = sqliteString(vals[i])
		}
		comments = append(comments, row)
	}
	return map[string]any{
		"count":    len(comments),
		"comments": comments,
	}
}
