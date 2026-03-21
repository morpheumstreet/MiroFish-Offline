package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/mirofish-offline/lake/internal/app"
)

type Server struct {
	deps *app.Deps
	app  *fiber.App
}

func NewServer(deps *app.Deps) *Server {
	bodyLimit := int(deps.Config.MaxUploadBytes)
	if bodyLimit <= 0 {
		bodyLimit = 50 << 20
	}
	app := fiber.New(fiber.Config{
		BodyLimit:   bodyLimit,
		ReadTimeout: 300 * time.Second,
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Content-Type, Authorization",
	}))
	s := &Server{deps: deps, app: app}
	s.routes()
	return s
}

func (s *Server) App() *fiber.App {
	return s.app
}

func (s *Server) reqCtx(c *fiber.Ctx) context.Context {
	if cx := c.UserContext(); cx != nil {
		return cx
	}
	return context.Background()
}

func (s *Server) routes() {
	s.app.Get("/health", s.handleHealth)

	api := s.app.Group("/api")
	s.mountGraph(api.Group("/graph"))
	s.mountSimulation(api.Group("/simulation"))
	s.mountReport(api.Group("/report"))

	d := strings.TrimSpace(os.Getenv("LAKE_FRONTEND_DIST"))
	if d != "" {
		if _, err := os.Stat(filepath.Join(d, "index.html")); err == nil {
			s.mountSPA(d)
		}
	}
}

func (s *Server) mountGraph(g fiber.Router) {
	g.Get("/project/:id", s.handleGetProject)
	g.Get("/project/list", s.handleListProjects)
	g.Delete("/project/:id", s.handleDeleteProject)
	g.Post("/project/:id/reset", s.handleResetProject)
	g.Post("/ontology/generate", s.handleOntologyGenerate)
	g.Post("/build", s.handleGraphBuild)
	g.Get("/task/:id", s.handleGetTask)
	g.Get("/tasks", s.handleListTasks)
	g.Get("/data/:graphId", s.handleGetGraphData)
	g.Delete("/delete/:graphId", s.handleDeleteGraph)
	g.Get("/entities/:graphId/by-type/:entityType", s.handleSimEntitiesByType)
	g.Get("/entities/:graphId/:entityUUID", s.handleSimEntityDetail)
	g.Get("/entities/:graphId", s.handleSimEntities)
}

func (s *Server) mountSimulation(sim fiber.Router) {
	sim.Post("/create", s.handleSimCreate)
	sim.Post("/prepare", s.handleSimPrepare)
	sim.Post("/prepare/status", s.handleSimPrepareStatus)
	sim.Post("/generate-profiles", s.handleSimGenerateProfiles)
	sim.Post("/start", s.handleSimStart)
	sim.Post("/stop", s.handleSimStop)
	sim.Post("/env-status", s.handleSimEnvStatus)
	sim.Post("/close-env", s.handleSimCloseEnv)
	sim.Post("/interview/batch", s.handleSimInterviewBatch)

	ent := sim.Group("/entities")
	ent.Get("/:graphId/by-type/:entityType", s.handleSimEntitiesByType)
	ent.Get("/:graphId/:entityUUID", s.handleSimEntityDetail)
	ent.Get("/:graphId", s.handleSimEntities)

	sim.Get("/list", s.handleSimList)
	sim.Get("/history", s.handleSimHistory)
	sim.Get("/download/script/:scriptName", s.handleSimScriptDownload)
	sim.Get("/:simulationId/profiles/realtime", s.handleSimProfilesRealtime)
	sim.Get("/:simulationId/profiles", s.handleSimProfiles)
	sim.Get("/:simulationId/config/realtime", s.handleSimConfigRealtime)
	sim.Get("/:simulationId/config/download", s.handleSimConfigDownload)
	sim.Get("/:simulationId/config", s.handleSimConfig)
	sim.Get("/:simulationId/run-status/detail", s.handleSimRunStatusDetail)
	sim.Get("/:simulationId/run-status", s.handleSimRunStatus)
	sim.Get("/:simulationId/actions", s.handleSimActions)
	sim.Get("/:simulationId/timeline", s.handleSimTimeline)
	sim.Get("/:simulationId/agent-stats", s.handleSimAgentStats)
	sim.Get("/:simulationId/posts", s.handleSimPosts)
	sim.Get("/:simulationId/comments", s.handleSimComments)
	sim.Get("/:simulationId", s.handleSimGet)
}

func (s *Server) mountReport(r fiber.Router) {
	r.Get("/generate/status", s.handleReportGenerateStatusGET)
	r.Post("/generate/status", s.handleReportGenerateStatusPOST)
	r.Get("/check", s.handleReportCheck)
	r.Get("/by-simulation", s.handleReportBySimulation)
	r.Get("/list", s.handleReportList)
	r.Post("/tools/search", s.handleReportToolsSearch)
	r.Post("/tools/statistics", s.handleReportToolsStatistics)
	r.Post("/generate", s.handleReportGenerate)
	r.Post("/chat", s.handleReportChat)
	r.Get("/:reportId/agent-log/stream", s.handleReportAgentLogStream)
	r.Get("/:reportId/console-log/stream", s.handleReportConsoleLogStream)
	r.Get("/:reportId/agent-log", s.handleReportAgentLog)
	r.Get("/:reportId/console-log", s.handleReportConsoleLog)
	r.Get("/:reportId/download", s.handleReportDownload)
	r.Get("/:reportId/progress", s.handleReportProgress)
	r.Get("/:reportId/sections", s.handleReportSections)
	r.Get("/:reportId/section/:sectionIndex", s.handleReportSection)
	r.Delete("/:reportId", s.handleReportDelete)
	r.Get("/:reportId", s.handleReportGet)
}

func (s *Server) mountSPA(dist string) {
	root := filepath.Clean(dist)
	h := func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return c.SendStatus(fiber.StatusMethodNotAllowed)
		}
		wild := strings.TrimSpace(c.Params("*"))
		rel := strings.TrimPrefix(filepath.Clean("/"+wild), "/")
		p := filepath.Join(root, rel)
		if !strings.HasPrefix(p, root) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			return c.SendFile(filepath.Join(root, "index.html"))
		}
		return c.SendFile(p)
	}
	s.app.Get("/*", h)
	s.app.Head("/*", h)
}

func (s *Server) handleHealth(c *fiber.Ctx) error {
	data := map[string]any{
		"status":  "ok",
		"service": "MiroFish-Offline Lake",
	}
	if s.deps.NeoCloser != nil {
		err := s.deps.Neo4jHealth.Ping(s.reqCtx(c))
		data["neo4j_ok"] = err == nil
		if err != nil {
			data["neo4j_error"] = err.Error()
		}
	}
	return okResp(c, data)
}
