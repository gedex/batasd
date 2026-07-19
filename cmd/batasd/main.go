// Batasd serves the code execution API and runs background submission workers.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gedex/batasd/internal/callback"
	"github.com/gedex/batasd/internal/config"
	"github.com/gedex/batasd/internal/execution"
	"github.com/gedex/batasd/internal/httpapi"
	"github.com/gedex/batasd/internal/language"
	"github.com/gedex/batasd/internal/migrations"
	"github.com/gedex/batasd/internal/queue/memory"
	postgresrepo "github.com/gedex/batasd/internal/repository/postgres"
	"github.com/gedex/batasd/internal/sandbox"
	"github.com/gedex/batasd/internal/sandbox/direct"
	dockersandbox "github.com/gedex/batasd/internal/sandbox/docker"
	isolatesandbox "github.com/gedex/batasd/internal/sandbox/isolate"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	go func() {
		sig := <-signals
		logger.Info("shutdown signal received", "signal", sig.String())
		cancel()
	}()

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
	case "docker":
		runner = dockersandbox.NewRunner(cfg.Sandbox.DockerBinary, cfg.Sandbox.DockerImage)
	case "isolate":
		runner = isolatesandbox.NewRunner(
			cfg.Sandbox.IsolateBinary,
			cfg.Sandbox.IsolateBoxIDStart,
			cfg.Sandbox.IsolateBoxIDCount,
			cfg.Sandbox.IsolateControlGroup,
		)
	default:
		logger.Error("unsupported sandbox driver", "driver", cfg.Sandbox.Driver)
		os.Exit(1)
	}

	executionEngine := execution.NewEngine(languages, runner, cfg.Sandbox.WorkDir)
	callbackDeliverer := callback.NewDeliverer(submissionRepo, cfg.Callback.Timeout)
	workerMonitor := worker.NewMonitor(cfg.Queue.Workers)
	workerRunner := worker.New(logger, queue, submissionRepo, executionEngine, callbackDeliverer, workerMonitor)
	var workerWG sync.WaitGroup
	for i := 1; i <= cfg.Queue.Workers; i++ {
		workerWG.Add(1)
		go func(id int) {
			defer workerWG.Done()
			workerRunner.Run(ctx, id)
		}(i)
	}
	if err := recoverUnfinishedSubmissions(ctx, logger, submissionRepo, queue); err != nil {
		logger.Error("recover unfinished submissions", "error", err)
		os.Exit(1)
	}

	submissionService := submission.NewService(submission.ServiceConfig{
		Repository: submissionRepo,
		Queue:      queue,
		Languages:  languages,
		Limits:     cfg.Submissions.DefaultLimits,
		MaxLimits:  cfg.Submissions.MaxLimits,
	})

	router := httpapi.NewRouter(httpapi.Dependencies{
		Config:      cfg,
		Logger:      logger,
		Submissions: submissionService,
		Languages:   languages,
		Queue:       queue,
		Workers:     workerMonitor,
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
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown started")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	logger.Info("stopping server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown server", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")

	logger.Info("waiting for workers", "workers", cfg.Queue.Workers)
	workersStopped := make(chan struct{})
	go func() {
		workerWG.Wait()
		close(workersStopped)
	}()

	select {
	case <-workersStopped:
		logger.Info("workers stopped")
	case <-shutdownCtx.Done():
		logger.Error("wait for workers", "error", shutdownCtx.Err())
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}

type unfinishedRecoverer interface {
	RecoverUnfinished(ctx context.Context, recoveredAt time.Time) ([]string, error)
}

type submissionEnqueuer interface {
	Enqueue(ctx context.Context, token string) error
}

func recoverUnfinishedSubmissions(ctx context.Context, logger *slog.Logger, repo unfinishedRecoverer, queue submissionEnqueuer) error {
	tokens, err := repo.RecoverUnfinished(ctx, time.Now().UTC())
	if err != nil {
		return err
	}
	for _, token := range tokens {
		if err := queue.Enqueue(ctx, token); err != nil {
			return fmt.Errorf("enqueue recovered submission %s: %w", token, err)
		}
	}
	logger.Info("recovered unfinished submissions", "count", len(tokens))
	return nil
}
