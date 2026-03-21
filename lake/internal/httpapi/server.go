package httpapi

import (
	"net/http"

	"github.com/mirofish-offline/lake/internal/app"
)

type Server struct {
	deps *app.Deps
	mux  *http.ServeMux
}

func NewServer(deps *app.Deps) *Server {
	s := &Server{deps: deps, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.cors(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)

	api := http.NewServeMux()
	s.mountGraph(api)
	s.mountSimulation(api)
	s.mountReport(api)

	s.mux.Handle("/api/", http.StripPrefix("/api", api))
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"status":  "ok",
		"service": "MiroFish-Offline Lake",
	}
	if s.deps.NeoCloser != nil {
		err := s.deps.Neo4jHealth.Ping(r.Context())
		data["neo4j_ok"] = err == nil
		if err != nil {
			data["neo4j_error"] = err.Error()
		}
	}
	ok(w, data)
}

func (s *Server) mountGraph(m *http.ServeMux) {
	prefix := "graph"
	m.HandleFunc("GET /"+prefix+"/project/{id}", s.handleGetProject)
	m.HandleFunc("GET /"+prefix+"/project/list", s.handleListProjects)
	m.HandleFunc("DELETE /"+prefix+"/project/{id}", s.handleDeleteProject)
	m.HandleFunc("POST /"+prefix+"/project/{id}/reset", s.handleResetProject)
	m.HandleFunc("POST /"+prefix+"/ontology/generate", s.handleOntologyGenerate)
	m.HandleFunc("POST /"+prefix+"/build", s.handleGraphBuild)
	m.HandleFunc("GET /"+prefix+"/task/{id}", s.handleGetTask)
	m.HandleFunc("GET /"+prefix+"/tasks", s.handleListTasks)
	m.HandleFunc("GET /"+prefix+"/data/{graphId}", s.handleGetGraphData)
	m.HandleFunc("DELETE /"+prefix+"/delete/{graphId}", s.handleDeleteGraph)
}

func (s *Server) mountSimulation(m *http.ServeMux) {
	p := "simulation"
	// Literal paths before /{simulationId} wildcard.
	m.HandleFunc("GET /"+p+"/entities/{graphId}/by-type/{entityType}", s.handleSimEntitiesByType)
	m.HandleFunc("GET /"+p+"/entities/{graphId}/{entityUUID}", s.handleSimEntityDetail)
	m.HandleFunc("GET /"+p+"/entities/{graphId}", s.handleSimEntities)
	m.HandleFunc("POST /"+p+"/create", s.handleSimCreate)
	m.HandleFunc("POST /"+p+"/prepare", s.handleSimPrepare)
	m.HandleFunc("POST /"+p+"/prepare/status", s.handleSimPrepareStatus)
	m.HandleFunc("GET /"+p+"/list", s.handleSimList)
	m.HandleFunc("GET /"+p+"/history", s.handleSimHistory)
	m.HandleFunc("GET /"+p+"/script/{scriptName}/download", s.handleSimScriptDownload)
	m.HandleFunc("POST /"+p+"/generate-profiles", s.handleSimGenerateProfiles)
	m.HandleFunc("POST /"+p+"/start", s.handleSimStart)
	m.HandleFunc("POST /"+p+"/stop", s.handleSimStop)
	m.HandleFunc("POST /"+p+"/env-status", s.handleSimEnvStatus)
	m.HandleFunc("POST /"+p+"/close-env", s.handleSimCloseEnv)
	m.HandleFunc("POST /"+p+"/interview/batch", s.handleSimInterviewBatch)
	m.HandleFunc("GET /"+p+"/{simulationId}/profiles/realtime", s.handleSimProfilesRealtime)
	m.HandleFunc("GET /"+p+"/{simulationId}/profiles", s.handleSimProfiles)
	m.HandleFunc("GET /"+p+"/{simulationId}/config/realtime", s.handleSimConfigRealtime)
	m.HandleFunc("GET /"+p+"/{simulationId}/config/download", s.handleSimConfigDownload)
	m.HandleFunc("GET /"+p+"/{simulationId}/config", s.handleSimConfig)
	m.HandleFunc("GET /"+p+"/{simulationId}/run-status/detail", s.handleSimRunStatusDetail)
	m.HandleFunc("GET /"+p+"/{simulationId}/run-status", s.handleSimRunStatus)
	m.HandleFunc("GET /"+p+"/{simulationId}/actions", s.handleSimActions)
	m.HandleFunc("GET /"+p+"/{simulationId}/timeline", s.handleSimTimeline)
	m.HandleFunc("GET /"+p+"/{simulationId}/agent-stats", s.handleSimAgentStats)
	m.HandleFunc("GET /"+p+"/{simulationId}/posts", s.handleSimPosts)
	m.HandleFunc("GET /"+p+"/{simulationId}/comments", s.handleSimComments)
	m.HandleFunc("GET /"+p+"/{simulationId}", s.handleSimGet)
}

func (s *Server) mountReport(m *http.ServeMux) {
	p := "report"
	m.HandleFunc("GET /"+p+"/generate/status", s.handleReportGenerateStatusGET)
	m.HandleFunc("POST /"+p+"/generate/status", s.handleReportGenerateStatusPOST)
	m.HandleFunc("GET /"+p+"/check/{simulationId}", s.handleReportCheck)
	m.HandleFunc("GET /"+p+"/by-simulation/{simulationId}", s.handleReportBySimulation)
	m.HandleFunc("POST /"+p+"/generate", s.handleReportGenerate)
	m.HandleFunc("POST /"+p+"/chat", s.handleReportChat)
	m.HandleFunc("GET /"+p+"/{reportId}/agent-log", s.handleReportAgentLog)
	m.HandleFunc("GET /"+p+"/{reportId}/console-log", s.handleReportConsoleLog)
	m.HandleFunc("GET /"+p+"/{reportId}", s.handleReportGet)
}

func (s *Server) stubGraph(label string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		fail(w, http.StatusNotImplemented, "lake skeleton: "+label+" — port use case + adapter")
	}
}
