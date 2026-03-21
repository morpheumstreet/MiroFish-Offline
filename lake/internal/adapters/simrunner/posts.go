package simrunner

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// QueryPosts reads OASIS sqlite post table (best-effort).
func QueryPosts(simDir, platform string, limit, offset int) map[string]any {
	dbName := platform + "_simulation.db"
	dbPath := filepath.Join(simDir, dbName)
	if _, err := os.Stat(dbPath); err != nil {
		return map[string]any{
			"platform": platform,
			"count":    0,
			"posts":    []any{},
			"message":  "Database does not exist，SimulationMay not have run yet",
		}
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		return map[string]any{"platform": platform, "count": 0, "posts": []any{}, "error": err.Error()}
	}
	defer db.Close()
	rows, err := db.Query(`SELECT * FROM post ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return map[string]any{"platform": platform, "count": 0, "posts": []any{}, "message": err.Error()}
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var posts []any
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
		posts = append(posts, row)
	}
	var total int
	_ = db.QueryRow(`SELECT COUNT(*) FROM post`).Scan(&total)
	return map[string]any{
		"platform": platform,
		"count":    len(posts),
		"total":    total,
		"posts":    posts,
	}
}

func sqliteString(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}
