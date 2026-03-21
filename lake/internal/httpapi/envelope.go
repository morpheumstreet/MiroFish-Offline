package httpapi

import (
	"encoding/json"
	"net/http"
)

type envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Count   int    `json:"count,omitempty"`
	Error   string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{Success: true, Data: data})
}

func okCount(w http.ResponseWriter, data any, count int) {
	writeJSON(w, http.StatusOK, envelope{Success: true, Data: data, Count: count})
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, envelope{Success: false, Error: msg})
}
