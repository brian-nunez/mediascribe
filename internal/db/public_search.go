package db

import (
	"context"
	"strings"
	"time"
)

func (s *Store) ListPublishedBlogRowsByJobIDs(ctx context.Context, jobIDs []string) ([]PublicBlogRow, error) {
	if len(jobIDs) == 0 {
		return []PublicBlogRow{}, nil
	}
	placeholders := make([]string, 0, len(jobIDs))
	args := make([]any, 0, len(jobIDs))
	for _, id := range jobIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	if len(placeholders) == 0 {
		return []PublicBlogRow{}, nil
	}

	query := `
SELECT
  bc.id,
  bc.job_id,
  bc.title,
  COALESCE(bc.section_id, ''),
  COALESCE(sec.name, ''),
  COALESCE(bcd.preview_text, ''),
  COALESCE(bcd.languages_json, ''),
  COALESCE(j.source_url, ''),
  COALESCE(j.source_path, ''),
  COALESCE(j.status, ''),
  COALESCE(j.artifact_dir, ''),
  bc.updated_at,
  bc.created_at,
  COALESCE(j.updated_at, ''),
  COALESCE(j.created_at, '')
FROM blog_catalog bc
LEFT JOIN blog_publish_state bps ON bps.blog_id = bc.id
LEFT JOIN sections sec ON sec.id = bc.section_id
LEFT JOIN blog_catalog_derived bcd ON bcd.blog_id = bc.id
LEFT JOIN jobs j ON j.id = bc.job_id
WHERE bc.deleted = 0
  AND COALESCE(bps.published, 0) = 1
  AND bc.job_id IN (` + strings.Join(placeholders, ",") + `)
ORDER BY bc.updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PublicBlogRow, 0, len(args))
	for rows.Next() {
		var item PublicBlogRow
		var bcUpdated, bcCreated, jobUpdated, jobCreated string
		if err := rows.Scan(
			&item.BlogID,
			&item.JobID,
			&item.Title,
			&item.SectionID,
			&item.SectionName,
			&item.PreviewText,
			&item.LanguagesJSON,
			&item.SourceURL,
			&item.SourcePath,
			&item.Status,
			&item.ArtifactDir,
			&bcUpdated,
			&bcCreated,
			&jobUpdated,
			&jobCreated,
		); err != nil {
			return nil, err
		}
		item.BlogUpdated, _ = parseRFC3339OrZero(bcUpdated)
		item.BlogCreated, _ = parseRFC3339OrZero(bcCreated)
		item.JobUpdated, _ = parseRFC3339OrZero(jobUpdated)
		item.JobCreated, _ = parseRFC3339OrZero(jobCreated)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CountPublishedLanguages(ctx context.Context, sectionID string) ([]LanguageCount, error) {
	base := `
SELECT boe.language AS language, COUNT(DISTINCT bc.id) AS cnt
FROM blog_catalog bc
JOIN blog_output_embeddings boe ON boe.job_id = bc.job_id
LEFT JOIN blog_publish_state bps ON bps.blog_id = bc.id
WHERE bc.deleted = 0
  AND COALESCE(bps.published, 0) = 1`
	args := make([]any, 0, 1)
	if strings.TrimSpace(sectionID) != "" && sectionID != "all" && sectionID != "unsectioned" {
		base += " AND COALESCE(bc.section_id, '') = ?"
		args = append(args, sectionID)
	} else if sectionID == "unsectioned" {
		base += " AND COALESCE(bc.section_id, '') = ''"
	}
	base += " GROUP BY boe.language ORDER BY boe.language ASC"

	rows, err := s.db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LanguageCount, 0)
	for rows.Next() {
		var item LanguageCount
		if err := rows.Scan(&item.Language, &item.Count); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func parseRFC3339OrZero(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}
