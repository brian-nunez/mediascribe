package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/brian-nunez/video-to-blog-page/internal/db"
	"github.com/brian-nunez/video-to-blog-page/internal/embeddings"
)

// GenerateBlogOutputEmbeddings creates/upserts embeddings for final blog outputs:
// - final.md as language "en"
// - final.<lang>.md files as their language code.
func GenerateBlogOutputEmbeddings(ctx context.Context, store *db.Store, job db.Job, embedder embeddings.Embedder) error {
	outputs, err := discoverBlogOutputs(job.ArtifactDir)
	if err != nil {
		return err
	}
	if len(outputs) == 0 {
		return fmt.Errorf("no blog output markdown files found for embedding")
	}

	for _, item := range outputs {
		vector, err := embedder.Embed(ctx, item.Markdown)
		if err != nil {
			return fmt.Errorf("embed blog output %s: %w", item.Language, err)
		}

		now := time.Now().UTC()
		rec := db.BlogOutputEmbedding{
			ID:                    uuid.NewString(),
			JobID:                 job.ID,
			Language:              item.Language,
			ContentSHA256:         sha256Hex(item.Markdown),
			Embedding:             embeddings.Float32ToBytes(vector),
			EmbeddingDimensions:   len(vector),
			EmbeddingModel:        job.EmbeddingModel,
			EmbeddingModelBaseURL: job.EmbeddingModelBaseURL,
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		if err := store.UpsertBlogOutputEmbedding(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

type blogOutputFile struct {
	Language string
	Markdown string
}

func discoverBlogOutputs(artifactDir string) ([]blogOutputFile, error) {
	out := make([]blogOutputFile, 0, 4)

	finalPath := filepath.Join(artifactDir, "final.md")
	if raw, err := os.ReadFile(finalPath); err == nil {
		out = append(out, blogOutputFile{
			Language: "en",
			Markdown: string(raw),
		})
	}

	files, err := filepath.Glob(filepath.Join(artifactDir, "final.*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, file := range files {
		base := filepath.Base(file)
		if base == "final.md" {
			continue
		}
		lang := strings.TrimSuffix(strings.TrimPrefix(base, "final."), ".md")
		lang = sanitizeLang(lang)
		if lang == "" || lang == "en" {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		out = append(out, blogOutputFile{
			Language: lang,
			Markdown: string(raw),
		})
	}
	return out, nil
}

func sha256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
