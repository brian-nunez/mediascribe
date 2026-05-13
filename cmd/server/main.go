package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/brian-nunez/video-to-blog-page/internal/artifacts"
	"github.com/brian-nunez/video-to-blog-page/internal/auth"
	"github.com/brian-nunez/video-to-blog-page/internal/config"
	"github.com/brian-nunez/video-to-blog-page/internal/db"
	api "github.com/brian-nunez/video-to-blog-page/internal/http"
	"github.com/brian-nunez/video-to-blog-page/internal/jobs"
	"github.com/brian-nunez/video-to-blog-page/internal/pipeline"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := artifacts.EnsureDir(cfg.ArtifactRoot); err != nil {
		log.Fatalf("ensure artifact root: %v", err)
	}

	store, err := db.Open(ctx, cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	if err := store.RunDefaultMigrations(ctx); err != nil {
		log.Fatalf("migrations: %v", err)
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

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("resolve working directory: %v", err)
	}

	h := api.Handler{
		Jobs: jobSvc,
		Auth: auth.Service{
			Store:      store,
			SessionTTL: cfg.AdminSessionTTL,
			CookieName: cfg.AdminCookieName,
		},
		UIRootDir: filepath.Join(cwd, "ui"),
	}

	mux := http.NewServeMux()
	mux.Handle("/", h.Routes())
	mux.Handle("/artifacts/", http.StripPrefix("/artifacts/", http.FileServer(http.Dir(filepath.Join(cwd, "artifacts")))))

	log.Printf("server listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
