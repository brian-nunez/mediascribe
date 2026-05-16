package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/brian-nunez/video-to-blog-page/internal/artifacts"
	"github.com/brian-nunez/video-to-blog-page/internal/auth"
	"github.com/brian-nunez/video-to-blog-page/internal/config"
	"github.com/brian-nunez/video-to-blog-page/internal/db"
	api "github.com/brian-nunez/video-to-blog-page/internal/http"
	"github.com/brian-nunez/video-to-blog-page/internal/httpserver"
	"github.com/brian-nunez/video-to-blog-page/internal/jobs"
	"github.com/brian-nunez/video-to-blog-page/internal/observability"
	"github.com/brian-nunez/video-to-blog-page/internal/pipeline"
)

func Run() error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := artifacts.EnsureDir(cfg.ArtifactRoot); err != nil {
		return err
	}

	store, err := db.Open(ctx, cfg.SQLitePath)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.RunDefaultMigrations(ctx); err != nil {
		return err
	}

	runner := pipeline.Runner{
		Store:                  store,
		FFmpegBin:              cfg.FFmpegBin,
		WhisperCPPBin:          cfg.WhisperCPPBin,
		WhisperModelPath:       cfg.WhisperModelPath,
		YTDLPBin:               cfg.YTDLPBin,
		TranscriptFallbackPath: cfg.TranscriptFallbackPath,
		MainModelTimeout:       cfg.DefaultMainModelTimeout,
		MainModelMaxRetries:    cfg.DefaultMainModelMaxRetries,
		EmbeddingModelTimeout:  cfg.DefaultEmbeddingModelTimeout,
		EmbeddingMaxRetries:    cfg.DefaultEmbeddingMaxRetries,
		TranslateModelTimeout:  cfg.DefaultTranslateModelTimeout,
		TranslateMaxRetries:    cfg.DefaultTranslateMaxRetries,
		ModelRetryBackoff:      cfg.ModelRetryBackoff,
	}

	jobSvc := jobs.NewService(store, runner, artifacts.Manager{Root: cfg.ArtifactRoot}, jobs.Defaults{
		MainModel:               cfg.DefaultMainModel,
		MainModelBaseURL:        cfg.DefaultMainModelBaseURL,
		EmbeddingModel:          cfg.DefaultEmbeddingModel,
		EmbeddingModelBaseURL:   cfg.DefaultEmbeddingModelBaseURL,
		TranslationModel:        cfg.DefaultTranslateModel,
		TranslationModelBaseURL: cfg.DefaultTranslateModelBaseURL,
		EnableTranslation:       cfg.EnableTranslation,
	})
	go func() {
		if err := jobSvc.BackfillOutputEmbeddings(context.Background()); err != nil {
			log.Printf("output embedding backfill failed: %v", err)
		}
	}()

	apiHandler := api.Handler{
		Jobs: jobSvc,
		Auth: auth.Service{
			Store:      store,
			SessionTTL: cfg.AdminSessionTTL,
			CookieName: cfg.AdminCookieName,
		},
	}

	otelConfig := observability.LoadConfigFromEnv()
	telemetry, err := observability.Init(ctx, otelConfig, log.Default())
	if err != nil {
		return err
	}

	artifactStaticDir := filepath.Dir(filepath.Clean(cfg.ArtifactRoot))
	if artifactStaticDir == "." || strings.TrimSpace(artifactStaticDir) == "" {
		artifactStaticDir = "./artifacts"
	}

	server := httpserver.Bootstrap(httpserver.BootstrapConfig{
		StaticDirectories: map[string]string{
			"/assets":    "./assets",
			"/icons":     "./assets/icons",
			"/artifacts": artifactStaticDir,
		},
		APIHandler: apiHandler.Routes(),
		Jobs:       jobSvc,
		Observability: httpserver.ObservabilityConfig{
			ServiceName:    otelConfig.ServiceName,
			TracingEnabled: otelConfig.Enabled,
		},
	})

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("server listening on %s", cfg.HTTPAddr)
		serverErr <- server.Start(cfg.HTTPAddr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("shutdown signal received: %s", sig)
	case err := <-serverErr:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := telemetry.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Println("server exited cleanly")
	return nil
}
