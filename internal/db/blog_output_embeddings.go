package db

import (
	"context"
	"time"
)

func (s *Store) UpsertBlogOutputEmbedding(ctx context.Context, rec BlogOutputEmbedding) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO blog_output_embeddings (
	id, job_id, language, content_sha256,
	embedding, embedding_dimensions,
	embedding_model, embedding_model_base_url,
	created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id, language) DO UPDATE SET
	content_sha256 = excluded.content_sha256,
	embedding = excluded.embedding,
	embedding_dimensions = excluded.embedding_dimensions,
	embedding_model = excluded.embedding_model,
	embedding_model_base_url = excluded.embedding_model_base_url,
	updated_at = excluded.updated_at
`,
		rec.ID,
		rec.JobID,
		rec.Language,
		rec.ContentSHA256,
		rec.Embedding,
		rec.EmbeddingDimensions,
		rec.EmbeddingModel,
		rec.EmbeddingModelBaseURL,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}
