package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) CreateJobBatch(ctx context.Context, batch JobBatch, items []JobBatchItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
INSERT INTO job_batches (
	id, name, delay_seconds, status,
	current_item_index, current_job_id,
	processed_items, total_items,
	last_error, next_run_at, started_at, completed_at,
	created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		batch.ID,
		batch.Name,
		batch.DelaySeconds,
		batch.Status,
		batch.CurrentItemIndex,
		nullableString(batch.CurrentJobID),
		batch.ProcessedItems,
		batch.TotalItems,
		nullableString(batch.LastError),
		nullableString(batch.NextRunAt),
		nullableString(batch.StartedAt),
		nullableString(batch.CompletedAt),
		batch.CreatedAt.UTC().Format(time.RFC3339Nano),
		batch.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO job_batch_items (
	id, batch_id, item_index,
	source_type, source_url, source_path,
	title, section_id, main_model,
	job_id, status, error_message,
	created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		_, err := stmt.ExecContext(ctx,
			item.ID,
			item.BatchID,
			item.ItemIndex,
			item.SourceType,
			nullableString(item.SourceURL),
			nullableString(item.SourcePath),
			nullableString(item.Title),
			nullableString(item.SectionID),
			nullableString(item.MainModel),
			nullableString(item.JobID),
			item.Status,
			nullableString(item.ErrorMessage),
			item.CreatedAt.UTC().Format(time.RFC3339Nano),
			item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListJobBatches(ctx context.Context) ([]JobBatch, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, delay_seconds, status,
       current_item_index, current_job_id,
       processed_items, total_items,
       last_error, next_run_at, started_at, completed_at,
       created_at, updated_at
FROM job_batches
ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]JobBatch, 0)
	for rows.Next() {
		item, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetJobBatch(ctx context.Context, batchID string) (JobBatch, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, delay_seconds, status,
       current_item_index, current_job_id,
       processed_items, total_items,
       last_error, next_run_at, started_at, completed_at,
       created_at, updated_at
FROM job_batches
WHERE id = ?`, batchID)
	item, err := scanBatch(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return JobBatch{}, ErrNotFound
		}
		return JobBatch{}, err
	}
	return item, nil
}

func (s *Store) ListBatchItems(ctx context.Context, batchID string) ([]JobBatchItem, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, batch_id, item_index,
       source_type, source_url, source_path,
       title, section_id, main_model,
       job_id, status, error_message,
       created_at, updated_at
FROM job_batch_items
WHERE batch_id = ?
ORDER BY item_index ASC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]JobBatchItem, 0)
	for rows.Next() {
		item, err := scanBatchItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdateJobBatchState(ctx context.Context, batch JobBatch) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE job_batches
SET status = ?,
    current_item_index = ?,
    current_job_id = ?,
    processed_items = ?,
    total_items = ?,
    last_error = ?,
    next_run_at = ?,
    started_at = ?,
    completed_at = ?,
    updated_at = ?
WHERE id = ?`,
		batch.Status,
		batch.CurrentItemIndex,
		nullableString(batch.CurrentJobID),
		batch.ProcessedItems,
		batch.TotalItems,
		nullableString(batch.LastError),
		nullableString(batch.NextRunAt),
		nullableString(batch.StartedAt),
		nullableString(batch.CompletedAt),
		batch.UpdatedAt.UTC().Format(time.RFC3339Nano),
		batch.ID,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateBatchItemState(ctx context.Context, item JobBatchItem) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE job_batch_items
SET job_id = ?, status = ?, error_message = ?, updated_at = ?
WHERE id = ?`,
		nullableString(item.JobID),
		item.Status,
		nullableString(item.ErrorMessage),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		item.ID,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func scanBatch(s scanner) (JobBatch, error) {
	var out JobBatch
	var currentJobID, lastError, nextRunAt, startedAt, completedAt sql.NullString
	var createdAt, updatedAt string
	if err := s.Scan(
		&out.ID,
		&out.Name,
		&out.DelaySeconds,
		&out.Status,
		&out.CurrentItemIndex,
		&currentJobID,
		&out.ProcessedItems,
		&out.TotalItems,
		&lastError,
		&nextRunAt,
		&startedAt,
		&completedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return JobBatch{}, err
	}
	out.CurrentJobID = currentJobID.String
	out.LastError = lastError.String
	out.NextRunAt = nextRunAt.String
	out.StartedAt = startedAt.String
	out.CompletedAt = completedAt.String
	ct, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return JobBatch{}, err
	}
	ut, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return JobBatch{}, err
	}
	out.CreatedAt = ct
	out.UpdatedAt = ut
	return out, nil
}

func scanBatchItem(s scanner) (JobBatchItem, error) {
	var out JobBatchItem
	var sourceURL, sourcePath, title, sectionID, mainModel, jobID, errorMessage sql.NullString
	var createdAt, updatedAt string
	if err := s.Scan(
		&out.ID,
		&out.BatchID,
		&out.ItemIndex,
		&out.SourceType,
		&sourceURL,
		&sourcePath,
		&title,
		&sectionID,
		&mainModel,
		&jobID,
		&out.Status,
		&errorMessage,
		&createdAt,
		&updatedAt,
	); err != nil {
		return JobBatchItem{}, err
	}
	out.SourceURL = sourceURL.String
	out.SourcePath = sourcePath.String
	out.Title = title.String
	out.SectionID = sectionID.String
	out.MainModel = mainModel.String
	out.JobID = jobID.String
	out.ErrorMessage = errorMessage.String
	ct, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return JobBatchItem{}, err
	}
	ut, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return JobBatchItem{}, err
	}
	out.CreatedAt = ct
	out.UpdatedAt = ut
	return out, nil
}
