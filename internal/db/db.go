package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, sqlitePath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) RunMigrations(ctx context.Context, migrationSQL string) error {
	_, err := s.db.ExecContext(ctx, migrationSQL)
	return err
}

func (s *Store) CreateJob(ctx context.Context, job Job) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO jobs (
	id, source_type, source_url, source_path, title,
	status, current_stage, error_message,
	artifact_dir,
	main_model, main_model_base_url,
	embedding_model, embedding_model_base_url,
	translation_model, translation_model_base_url,
	translation_enabled, translation_language,
	created_at, updated_at, completed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		job.ID,
		job.SourceType,
		nullableString(job.SourceURL),
		nullableString(job.SourcePath),
		nullableString(job.Title),
		job.Status,
		nullableString(job.CurrentStage),
		nullableString(job.ErrorMessage),
		job.ArtifactDir,
		job.MainModel,
		job.MainModelBaseURL,
		job.EmbeddingModel,
		job.EmbeddingModelBaseURL,
		job.TranslationModel,
		job.TranslationModelBaseURL,
		boolToInt(job.TranslationEnabled),
		nullableString(job.TranslationLanguage),
		job.CreatedAt.UTC().Format(time.RFC3339Nano),
		job.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTime(job.CompletedAt),
	)
	return err
}

func (s *Store) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, source_type, source_url, source_path, title,
       status, current_stage, error_message,
       artifact_dir,
       main_model, main_model_base_url,
       embedding_model, embedding_model_base_url,
       translation_model, translation_model_base_url,
       translation_enabled, translation_language,
       created_at, updated_at, completed_at
FROM jobs
ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Job, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, source_type, source_url, source_path, title,
       status, current_stage, error_message,
       artifact_dir,
       main_model, main_model_base_url,
       embedding_model, embedding_model_base_url,
       translation_model, translation_model_base_url,
       translation_enabled, translation_language,
       created_at, updated_at, completed_at
FROM jobs
WHERE id = ?`, id)

	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	return job, nil
}

func (s *Store) SetJobRunningStage(ctx context.Context, id, stage string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs
SET status = 'running', current_stage = ?, error_message = NULL, updated_at = ?
WHERE id = ?`, stage, nowRFC3339(), id)
	return err
}

func (s *Store) MarkJobFailed(ctx context.Context, id, stage, errMessage string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs
SET status = 'failed', current_stage = ?, error_message = ?, updated_at = ?
WHERE id = ?`, stage, errMessage, nowRFC3339(), id)
	return err
}

func (s *Store) MarkJobComplete(ctx context.Context, id string) error {
	now := nowRFC3339()
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs
SET status = 'complete', current_stage = 'mark_complete', error_message = NULL,
    updated_at = ?, completed_at = ?
WHERE id = ?`, now, now, id)
	return err
}

func (s *Store) ResetJobForRetry(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs
SET status = 'queued', current_stage = 'create_job', error_message = NULL,
    updated_at = ?, completed_at = NULL
WHERE id = ?`, nowRFC3339(), id)
	return err
}

func (s *Store) ReplaceTranscriptChunks(ctx context.Context, jobID string, chunks []TranscriptChunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM transcript_chunks WHERE job_id = ?`, jobID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO transcript_chunks (
	id, job_id, chunk_index,
	start_time_seconds, end_time_seconds,
	content, token_count, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		_, err := stmt.ExecContext(ctx,
			chunk.ID,
			chunk.JobID,
			chunk.ChunkIndex,
			chunk.StartTimeSeconds,
			chunk.EndTimeSeconds,
			chunk.Content,
			chunk.TokenCount,
			chunk.CreatedAt.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListTranscriptChunks(ctx context.Context, jobID string) ([]TranscriptChunk, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, job_id, chunk_index,
       start_time_seconds, end_time_seconds,
       content, token_count, created_at
FROM transcript_chunks
WHERE job_id = ?
ORDER BY chunk_index ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chunks := make([]TranscriptChunk, 0)
	for rows.Next() {
		var chunk TranscriptChunk
		var createdAt string
		if err := rows.Scan(
			&chunk.ID,
			&chunk.JobID,
			&chunk.ChunkIndex,
			&chunk.StartTimeSeconds,
			&chunk.EndTimeSeconds,
			&chunk.Content,
			&chunk.TokenCount,
			&createdAt,
		); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		chunk.CreatedAt = t
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func (s *Store) ReplaceChunkEmbeddings(ctx context.Context, jobID string, records []ChunkEmbedding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM chunk_embeddings WHERE job_id = ?`, jobID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO chunk_embeddings (
	id, job_id, chunk_id,
	embedding, embedding_dimensions,
	embedding_model, embedding_model_base_url,
	created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, rec := range records {
		_, err := stmt.ExecContext(ctx,
			rec.ID,
			rec.JobID,
			rec.ChunkID,
			rec.Embedding,
			rec.EmbeddingDimensions,
			rec.EmbeddingModel,
			rec.EmbeddingModelBaseURL,
			rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ReplaceChunkAnalyses(ctx context.Context, jobID string, records []ChunkAnalysis) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM chunk_analysis WHERE job_id = ?`, jobID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO chunk_analysis (id, job_id, chunk_id, analysis_json, created_at)
VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, rec := range records {
		_, err := stmt.ExecContext(ctx,
			rec.ID,
			rec.JobID,
			rec.ChunkID,
			rec.AnalysisJSON,
			rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpsertBlogOutput(ctx context.Context, out BlogOutput) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO blog_outputs (
	id, job_id, title, slug,
	outline_path, draft_path, final_markdown_path,
	translated_markdown_path, translation_language,
	status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id) DO UPDATE SET
	title = excluded.title,
	slug = excluded.slug,
	outline_path = excluded.outline_path,
	draft_path = excluded.draft_path,
	final_markdown_path = excluded.final_markdown_path,
	translated_markdown_path = excluded.translated_markdown_path,
	translation_language = excluded.translation_language,
	status = excluded.status,
	updated_at = excluded.updated_at
`,
		out.ID,
		out.JobID,
		out.Title,
		out.Slug,
		nullableString(out.OutlinePath),
		nullableString(out.DraftPath),
		nullableString(out.FinalMarkdownPath),
		nullableString(out.TranslatedMarkdownPath),
		nullableString(out.TranslationLanguage),
		out.Status,
		out.CreatedAt.UTC().Format(time.RFC3339Nano),
		out.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) GetBlogOutputByJob(ctx context.Context, jobID string) (BlogOutput, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, job_id, title, slug,
       outline_path, draft_path, final_markdown_path,
       translated_markdown_path, translation_language,
       status, created_at, updated_at
FROM blog_outputs WHERE job_id = ?`, jobID)

	var out BlogOutput
	var outlinePath, draftPath, finalPath, translatedPath, translationLanguage sql.NullString
	var createdAt, updatedAt string
	if err := row.Scan(
		&out.ID,
		&out.JobID,
		&out.Title,
		&out.Slug,
		&outlinePath,
		&draftPath,
		&finalPath,
		&translatedPath,
		&translationLanguage,
		&out.Status,
		&createdAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BlogOutput{}, ErrNotFound
		}
		return BlogOutput{}, err
	}
	out.OutlinePath = outlinePath.String
	out.DraftPath = draftPath.String
	out.FinalMarkdownPath = finalPath.String
	out.TranslatedMarkdownPath = translatedPath.String
	out.TranslationLanguage = translationLanguage.String

	ct, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return BlogOutput{}, err
	}
	ut, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return BlogOutput{}, err
	}
	out.CreatedAt = ct
	out.UpdatedAt = ut
	return out, nil
}

var ErrNotFound = errors.New("not found")

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(s scanner) (Job, error) {
	var job Job
	var sourceURL, sourcePath, title, currentStage, errorMessage, translationLanguage sql.NullString
	var completedAt sql.NullString
	var translationEnabled int
	var createdAt, updatedAt string

	err := s.Scan(
		&job.ID,
		&job.SourceType,
		&sourceURL,
		&sourcePath,
		&title,
		&job.Status,
		&currentStage,
		&errorMessage,
		&job.ArtifactDir,
		&job.MainModel,
		&job.MainModelBaseURL,
		&job.EmbeddingModel,
		&job.EmbeddingModelBaseURL,
		&job.TranslationModel,
		&job.TranslationModelBaseURL,
		&translationEnabled,
		&translationLanguage,
		&createdAt,
		&updatedAt,
		&completedAt,
	)
	if err != nil {
		return Job{}, err
	}

	job.SourceURL = sourceURL.String
	job.SourcePath = sourcePath.String
	job.Title = title.String
	job.CurrentStage = currentStage.String
	job.ErrorMessage = errorMessage.String
	job.TranslationEnabled = translationEnabled == 1
	job.TranslationLanguage = translationLanguage.String

	ct, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Job{}, fmt.Errorf("parse created_at: %w", err)
	}
	ut, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return Job{}, fmt.Errorf("parse updated_at: %w", err)
	}
	job.CreatedAt = ct
	job.UpdatedAt = ut

	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return Job{}, fmt.Errorf("parse completed_at: %w", err)
		}
		job.CompletedAt = &t
	}

	return job, nil
}
