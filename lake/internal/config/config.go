package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config mirrors backend/app/config.py fields needed for Lake startup and adapters.
type Config struct {
	Host string
	Port int

	Debug bool

	LLMAPIKey    string
	LLMBaseURL   string
	LLMModelName string
	OllamaNumCtx int
	LLMTimeout   int // seconds

	Neo4jURI      string
	Neo4jUser     string
	Neo4jPassword string

	EmbeddingModel   string
	EmbeddingBaseURL string

	UploadFolder        string
	MaxUploadBytes      int64
	OasisSimulationsDir string
	// BackendRoot is the repo's backend/ directory (for OASIS scripts: backend/scripts).
	BackendRoot string
}

func Load() Config {
	port, _ := strconv.Atoi(getenv("LAKE_HTTP_PORT", getenv("FLASK_PORT", "5001")))
	maxUpload := int64(50 * 1024 * 1024)
	numCtx, _ := strconv.Atoi(getenv("OLLAMA_NUM_CTX", "8192"))
	llmTimeout, _ := strconv.Atoi(getenv("LLM_TIMEOUT_SECONDS", "300"))
	if llmTimeout <= 0 {
		llmTimeout = 300
	}

	uploadDefault := "uploads"
	backendRoot := ""
	if _, err := os.Stat("../../backend/uploads"); err == nil {
		uploadDefault = "../../backend/uploads"
		if _, err := os.Stat("../../backend/scripts"); err == nil {
			backendRoot, _ = filepath.Abs("../../backend")
		}
	}
	if backendRoot == "" {
		if p, err := filepath.Abs("backend"); err == nil {
			if _, err := os.Stat(filepath.Join(p, "scripts")); err == nil {
				backendRoot = p
			}
		}
	}

	return Config{
		Host: getenv("LAKE_HTTP_HOST", getenv("FLASK_HOST", "0.0.0.0")),
		Port: port,

		Debug: getenv("LAKE_DEBUG", getenv("FLASK_DEBUG", "true")) == "true",

		LLMAPIKey:    os.Getenv("LLM_API_KEY"),
		LLMBaseURL:   getenv("LLM_BASE_URL", "http://localhost:11434/v1"),
		LLMModelName: getenv("LLM_MODEL_NAME", "qwen2.5:32b"),
		OllamaNumCtx: numCtx,
		LLMTimeout:   llmTimeout,

		Neo4jURI:      getenv("NEO4J_URI", "bolt://localhost:7687"),
		Neo4jUser:     getenv("NEO4J_USER", "neo4j"),
		Neo4jPassword: getenv("NEO4J_PASSWORD", "mirofish"),

		EmbeddingModel:   getenv("EMBEDDING_MODEL", "nomic-embed-text"),
		EmbeddingBaseURL: getenv("EMBEDDING_BASE_URL", "http://localhost:11434"),

		UploadFolder:        getenv("LAKE_UPLOAD_FOLDER", uploadDefault),
		MaxUploadBytes:      maxUpload,
		OasisSimulationsDir: getenv("OASIS_SIMULATION_DATA_DIR", ""),
		BackendRoot:         getenv("LAKE_BACKEND_ROOT", backendRoot),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Validate returns issues that should block startup (aligned with Flask Config.validate).
func (c Config) Validate() []string {
	var errs []string
	if c.LLMAPIKey == "" {
		errs = append(errs, "LLM_API_KEY not configured (set to any non-empty value, e.g. ollama)")
	}
	if c.Neo4jURI == "" {
		errs = append(errs, "NEO4J_URI not configured")
	}
	if c.Neo4jPassword == "" {
		errs = append(errs, "NEO4J_PASSWORD not configured")
	}
	return errs
}

func (c Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// SimulationsDir returns the OASIS simulation workspace (state.json, profiles, run logs).
func (c Config) SimulationsDir() string {
	if c.OasisSimulationsDir != "" {
		return c.OasisSimulationsDir
	}
	return filepath.Join(c.UploadFolder, "simulations")
}

// ScriptsDir returns backend/scripts for run_parallel_simulation.py etc.
func (c Config) ScriptsDir() string {
	if v := os.Getenv("LAKE_SCRIPTS_DIR"); v != "" {
		return v
	}
	if c.BackendRoot != "" {
		return filepath.Join(c.BackendRoot, "scripts")
	}
	return ""
}

// ReportsDir is uploads/reports (history enrichment).
func (c Config) ReportsDir() string {
	return filepath.Join(c.UploadFolder, "reports")
}
