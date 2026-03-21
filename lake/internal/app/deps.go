package app

import (
	"github.com/mirofish-offline/lake/internal/adapters/fileparser"
	"github.com/mirofish-offline/lake/internal/adapters/neo4j"
	"github.com/mirofish-offline/lake/internal/adapters/noop"
	"github.com/mirofish-offline/lake/internal/adapters/openai"
	"github.com/mirofish-offline/lake/internal/adapters/projectstore"
	"github.com/mirofish-offline/lake/internal/adapters/simrunner"
	"github.com/mirofish-offline/lake/internal/adapters/simulationfs"
	"github.com/mirofish-offline/lake/internal/adapters/simulationprep"
	"github.com/mirofish-offline/lake/internal/adapters/taskstore"
	"github.com/mirofish-offline/lake/internal/adapters/textproc"
	"github.com/mirofish-offline/lake/internal/config"
	"github.com/mirofish-offline/lake/internal/ports"
	"github.com/mirofish-offline/lake/internal/usecase/ontology"
	"github.com/mirofish-offline/lake/internal/usecase/simulation"
)

// Deps is the composition root's handle: inject into HTTP handlers / use cases.
type Deps struct {
	Config config.Config

	Neo4jHealth ports.Neo4jHealth
	NeoCloser   func() error
	GraphReady  bool

	Projects ports.ProjectRepository
	Tasks    ports.TaskRepository
	Text     ports.TextProcessor
	Files    ports.FileParser
	Ontology ports.OntologyGenerator
	Graph    ports.GraphBuilder
	Entity   ports.EntityReader
	Sim      *simulation.Service
	Reports  ports.ReportService
	Tools    ports.GraphTools
}

func NewDeps(cfg config.Config) (*Deps, error) {
	var n noop.Deps

	ps, err := projectstore.New(cfg.UploadFolder)
	if err != nil {
		return nil, err
	}
	tasks := taskstore.New()
	llm := openai.New(cfg)

	d := &Deps{
		Config:      cfg,
		Projects:    ps,
		Tasks:       tasks,
		Text:        textproc.New(),
		Files:       fileparser.New(),
		Ontology:    ontology.New(llm),
		Graph:       n,
		Entity:      n,
		Reports:     n,
		Tools:       n,
		Neo4jHealth: n,
	}

	gs, err := neo4j.NewGraphStore(cfg, llm)
	if err == nil {
		d.Graph = gs
		d.Neo4jHealth = gs
		d.NeoCloser = gs.Close
		d.GraphReady = true
		d.Entity = neo4j.NewEntityReader(gs)
	}

	simRepo, err := simulationfs.New(cfg.SimulationsDir())
	if err != nil {
		return nil, err
	}
	var rt ports.SimulationRuntime = simrunner.Disabled{}
	if r, err := simrunner.NewRuntime(cfg); err == nil {
		rt = r
	}
	d.Sim = &simulation.Service{
		Cfg:      cfg,
		Projects: ps,
		Tasks:    tasks,
		Repo:     simRepo,
		Entities: d.Entity,
		Profiles: simulationprep.NewProfileBuilder(cfg, llm),
		Config:   simulationprep.NewConfigBuilder(cfg, llm),
		Runtime:  rt,
		GraphOK:  d.GraphReady,
	}
	return d, nil
}
