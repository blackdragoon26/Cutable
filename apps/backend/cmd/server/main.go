package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/blackdragoon26/cutable/apps/backend/internal/agent"
	"github.com/blackdragoon26/cutable/apps/backend/internal/config"
	"github.com/blackdragoon26/cutable/apps/backend/internal/httpapi"
	"github.com/blackdragoon26/cutable/apps/backend/internal/provider"
	"github.com/blackdragoon26/cutable/apps/backend/internal/store"
)

func main() {
	loadEnvFiles("/run/secrets/cutable.env", "../../.env", ".env")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	database, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database startup failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	if err := database.Migrate(ctx); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	openRouter := provider.NewOpenRouter(cfg.OpenRouterAPIKey, cfg.OpenRouterModel)
	e2b := provider.NewE2B(cfg.E2BAPIKey, cfg.E2BTemplateAlias, cfg.SandboxTimeout)
	runner := agent.NewRunner(database, openRouter, e2b, cfg.AgentMaxSteps, logger)
	api := httpapi.New(cfg, database, runner, e2b, logger)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      15 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		logger.Info("Cutable API listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			cancel()
		}
	}()
	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}
}

// loadEnvFiles loads each candidate .env file independently. godotenv.Load
// stops at the first file it can't open, so passing all candidates to a
// single call silently skips every later path whenever an earlier one (e.g.
// the production-only /run/secrets/cutable.env) doesn't exist — which meant
// local development's repository-root .env was never actually loaded.
func loadEnvFiles(paths ...string) {
	for _, path := range paths {
		_ = godotenv.Load(path)
	}
}
