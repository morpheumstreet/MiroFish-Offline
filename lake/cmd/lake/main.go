package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	srv := &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           httpapi.NewServer(deps).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("lake listening on http://%s (health: /health)", cfg.ListenAddr())
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
