// Batasd serves the code execution API and runs background submission workers.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gedex/batasd/internal/config"
	"github.com/gedex/batasd/internal/execution"
	"github.com/gedex/batasd/internal/httpapi"
	"github.com/gedex/batasd/internal/language"
	"github.com/gedex/batasd/internal/migrations"
	"github.com/gedex/batasd/internal/queue/memory"
	postgresrepo "github.com/gedex/batasd/internal/repository/postgres"
	"github.com/gedex/batasd/internal/sandbox"
	"github.com/gedex/batasd/internal/sandbox/direct"
	"github.com/gedex/batasd/internal/submission"
	"github.com/gedex/batasd/internal/worker"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		logger.Error("ping database", "error", err)
		os.Exit(1)
	}

	if cfg.Database.RunMigrations {
		if err := migrations.Run(ctx, db); err != nil {
			logger.Error("run migrations", "error", err)
			os.Exit(1)
		}
	}

	languages, err := language.LoadCatalog()
	if err != nil {
		logger.Error("load language catalog", "error", err)
		os.Exit(1)
	}

	submissionRepo := postgresrepo.NewSubmissionRepository(db)
	queue := memory.NewQueue()

	var runner sandbox.Runner
	switch cfg.Sandbox.Driver {
	case "direct":
		runner = direct.NewRunner()
	default:
		logger.Error("unsupported sandbox driver", "driver", cfg.Sandbox.Driver)
		os.Exit(1)
	}

	executionEngine := execution.NewEngine(languages, runner, cfg.Sandbox.WorkDir)
	workerRunner := worker.New(logger, queue, submissionRepo, executionEngine)
	for i := 1; i <= cfg.Queue.Workers; i++ {
		go workerRunner.Run(ctx, i)
	}

	submissionService := submission.NewService(submission.ServiceConfig{
		Repository: submissionRepo,
		Queue:      queue,
		Languages:  languages,
		Limits:     cfg.Submissions.DefaultLimits,
	})

	router := httpapi.NewRouter(httpapi.Dependencies{
		Config:      cfg,
		Logger:      logger,
		Submissions: submissionService,
		Languages:   languages,
		Queue:       queue,
	})

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting server", "addr", cfg.HTTP.Addr, "env", cfg.AppEnv, "sandbox", cfg.Sandbox.Driver)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("serve http", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown server", "error", err)
		os.Exit(1)
	}
}
