package main

import (
	"context"
	"fmt"
	"log"

	"github.com/brian-nunez/video-to-blog-page/internal/artifacts"
	"github.com/brian-nunez/video-to-blog-page/internal/config"
	"github.com/brian-nunez/video-to-blog-page/internal/db"
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
		Store:                 store,
		EmbeddingModelTimeout: cfg.DefaultEmbeddingModelTimeout,
		EmbeddingMaxRetries:   cfg.DefaultEmbeddingMaxRetries,
		ModelRetryBackoff:     cfg.ModelRetryBackoff,
	}

	svc := jobs.NewService(store, runner, artifacts.Manager{Root: cfg.ArtifactRoot}, jobs.Defaults{
		EmbeddingModel:        cfg.DefaultEmbeddingModel,
		EmbeddingModelBaseURL: cfg.DefaultEmbeddingModelBaseURL,
	})

	result, err := svc.RebuildAllEmbeddingsNow(ctx)
	if err != nil {
		log.Fatalf("rebuild embeddings failed: %v", err)
	}

	fmt.Printf("Rebuild complete. Jobs processed: %d/%d\n", result.ProcessedJobs, result.TotalJobs)
	fmt.Printf("Chunk embeddings rebuilt: %d\n", result.ChunkEmbeddingsRebuilt)
	fmt.Printf("Output embeddings rebuilt: %d\n", result.OutputEmbeddingsRebuilt)
}
