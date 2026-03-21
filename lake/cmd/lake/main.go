package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mirofish-offline/lake/internal/app"
	"github.com/mirofish-offline/lake/internal/config"
	"github.com/mirofish-offline/lake/internal/httpapi"
)

func main() {
	cfg := config.Load()
	if errs := cfg.Validate(); len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "Configuration errors:")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		os.Exit(1)
	}

	deps, err := app.NewDeps(cfg)
	if err != nil {
		log.Fatalf("deps: %v", err)
	}
	if deps.NeoCloser != nil {
		defer func() { _ = deps.NeoCloser() }()
	}

	srv := httpapi.NewServer(deps)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		if err := srv.App().Shutdown(); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("lake listening on http://%s (health: /health)", cfg.ListenAddr())
	if err := srv.App().Listen(cfg.ListenAddr()); err != nil {
		log.Printf("listen: %v", err)
	}
}
