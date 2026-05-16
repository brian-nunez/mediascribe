package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *Store) CountAllPublicBlogs(ctx context.Context, language string) (int, error) {
	query := `
SELECT COUNT(1)
FROM blog_catalog bc
LEFT JOIN blog_publish_state bps ON bps.blog_id = bc.id
WHERE bc.deleted = 0
  AND COALESCE(bps.published, 0) = 1`
	args := make([]any, 0, 1)
	if strings.TrimSpace(language) != "" && strings.TrimSpace(strings.ToLower(language)) != "all" {
		query += ` AND EXISTS (SELECT 1 FROM blog_output_embeddings boe WHERE boe.job_id = bc.job_id AND LOWER(boe.language) = LOWER(?))`
		args = append(args, language)
	}
	row := s.db.QueryRowContext(ctx, query, args...)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) CountPublicBlogsInSection(ctx context.Context, sectionID, language string) (int, error) {
	query := `
SELECT COUNT(1)
FROM blog_catalog bc
LEFT JOIN blog_publish_state bps ON bps.blog_id = bc.id
WHERE bc.deleted = 0
  AND COALESCE(bps.published, 0) = 1
  AND COALESCE(bc.section_id, '') = ?`
	args := []any{sectionID}
	if strings.TrimSpace(language) != "" && strings.TrimSpace(strings.ToLower(language)) != "all" {
		query += ` AND EXISTS (SELECT 1 FROM blog_output_embeddings boe WHERE boe.job_id = bc.job_id AND LOWER(boe.language) = LOWER(?))`
		args = append(args, language)
	}
	row := s.db.QueryRowContext(ctx, query, args...)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) ListPublicBlogPage(ctx context.Context, sectionID, language string, limit, offset int) ([]PublicBlogRow, error) {
	query := `
SELECT
  bc.id,
  bc.job_id,
  bc.title,
  COALESCE(bc.section_id, ''),
  COALESCE(sec.name, ''),
  COALESCE(
    NULLIF(NULLIF(TRIM(bcd.preview_text), ''), 'No preview available.'),
    (
      SELECT SUBSTR(TRIM(REPLACE(REPLACE(bcc.markdown, CHAR(10), ' '), CHAR(13), ' ')), 1, 220)
      FROM blog_content_cache bcc
      WHERE bcc.blog_id = bc.id AND LOWER(bcc.language) = 'en'
      LIMIT 1
    ),
    (
      SELECT SUBSTR(TRIM(REPLACE(REPLACE(bcc.markdown, CHAR(10), ' '), CHAR(13), ' ')), 1, 220)
      FROM blog_content_cache bcc
      WHERE bcc.blog_id = bc.id
      ORDER BY CASE WHEN LOWER(bcc.language) = 'en' THEN 0 ELSE 1 END, bcc.updated_at DESC
      LIMIT 1
    ),
    ''
  ),
  COALESCE(
    bcd.languages_json,
    (
      SELECT json_group_array(lang.language)
      FROM (
        SELECT DISTINCT boe.language AS language
        FROM blog_output_embeddings boe
        WHERE boe.job_id = bc.job_id
        ORDER BY boe.language
      ) lang
    ),
    '[]'
  ),
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
  AND COALESCE(bc.section_id, '') = ?`
	args := []any{sectionID}
	if strings.TrimSpace(language) != "" && strings.TrimSpace(strings.ToLower(language)) != "all" {
		query += ` AND EXISTS (SELECT 1 FROM blog_output_embeddings boe WHERE boe.job_id = bc.job_id AND LOWER(boe.language) = LOWER(?))`
		args = append(args, language)
	}
	query += `
ORDER BY bc.updated_at DESC
LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PublicBlogRow, 0, limit)
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
		var err error
		item.BlogUpdated, err = time.Parse(time.RFC3339Nano, bcUpdated)
		if err != nil {
			return nil, err
		}
		item.BlogCreated, err = time.Parse(time.RFC3339Nano, bcCreated)
		if err != nil {
			return nil, err
		}
		if jobUpdated != "" {
			item.JobUpdated, _ = time.Parse(time.RFC3339Nano, jobUpdated)
		}
		if jobCreated != "" {
			item.JobCreated, _ = time.Parse(time.RFC3339Nano, jobCreated)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListAllPublicBlogPage(ctx context.Context, language string, limit, offset int) ([]PublicBlogRow, error) {
	query := `
SELECT
  bc.id,
  bc.job_id,
  bc.title,
  COALESCE(bc.section_id, ''),
  COALESCE(sec.name, ''),
  COALESCE(
    NULLIF(NULLIF(TRIM(bcd.preview_text), ''), 'No preview available.'),
    (
      SELECT SUBSTR(TRIM(REPLACE(REPLACE(bcc.markdown, CHAR(10), ' '), CHAR(13), ' ')), 1, 220)
      FROM blog_content_cache bcc
      WHERE bcc.blog_id = bc.id AND LOWER(bcc.language) = 'en'
      LIMIT 1
    ),
    (
      SELECT SUBSTR(TRIM(REPLACE(REPLACE(bcc.markdown, CHAR(10), ' '), CHAR(13), ' ')), 1, 220)
      FROM blog_content_cache bcc
      WHERE bcc.blog_id = bc.id
      ORDER BY CASE WHEN LOWER(bcc.language) = 'en' THEN 0 ELSE 1 END, bcc.updated_at DESC
      LIMIT 1
    ),
    ''
  ),
  COALESCE(
    bcd.languages_json,
    (
      SELECT json_group_array(lang.language)
      FROM (
        SELECT DISTINCT boe.language AS language
        FROM blog_output_embeddings boe
        WHERE boe.job_id = bc.job_id
        ORDER BY boe.language
      ) lang
    ),
    '[]'
  ),
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
  AND COALESCE(bps.published, 0) = 1`
	args := make([]any, 0, 3)
	if strings.TrimSpace(language) != "" && strings.TrimSpace(strings.ToLower(language)) != "all" {
		query += ` AND EXISTS (SELECT 1 FROM blog_output_embeddings boe WHERE boe.job_id = bc.job_id AND LOWER(boe.language) = LOWER(?))`
		args = append(args, language)
	}
	query += `
ORDER BY bc.updated_at DESC
LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PublicBlogRow, 0, limit)
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
		var err error
		item.BlogUpdated, err = time.Parse(time.RFC3339Nano, bcUpdated)
		if err != nil {
			return nil, err
		}
		item.BlogCreated, err = time.Parse(time.RFC3339Nano, bcCreated)
		if err != nil {
			return nil, err
		}
		if jobUpdated != "" {
			item.JobUpdated, _ = time.Parse(time.RFC3339Nano, jobUpdated)
		}
		if jobCreated != "" {
			item.JobCreated, _ = time.Parse(time.RFC3339Nano, jobCreated)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListSections(ctx context.Context) ([]Section, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, sort_order, created_at, updated_at
FROM sections
ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Section, 0)
	for rows.Next() {
		var item Section
		var createdAt, updatedAt string
		if err := rows.Scan(&item.ID, &item.Name, &item.SortOrder, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		ct, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		ut, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, err
		}
		item.CreatedAt = ct
		item.UpdatedAt = ut
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateSection(ctx context.Context, item Section) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sections (id, name, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)`,
		item.ID,
		item.Name,
		item.SortOrder,
		item.CreatedAt.UTC().Format(time.RFC3339Nano),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) UpdateSection(ctx context.Context, item Section) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE sections
SET name = ?, sort_order = ?, updated_at = ?
WHERE id = ?`,
		item.Name,
		item.SortOrder,
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

func (s *Store) DeleteSection(ctx context.Context, sectionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE blog_catalog SET section_id = NULL, updated_at = ? WHERE section_id = ?`, nowRFC3339(), sectionID); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM sections WHERE id = ?`, sectionID)
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
	return tx.Commit()
}

func (s *Store) UpsertBlogCatalog(ctx context.Context, item BlogCatalog) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO blog_catalog (id, job_id, section_id, title, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id) DO UPDATE SET
	title = excluded.title,
	updated_at = excluded.updated_at`,
		item.ID,
		item.JobID,
		nullableString(item.SectionID),
		item.Title,
		boolToInt(item.Deleted),
		item.CreatedAt.UTC().Format(time.RFC3339Nano),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) ListBlogCatalog(ctx context.Context) ([]BlogCatalog, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT
	bc.id, bc.job_id, bc.section_id, bc.title,
	COALESCE(bps.published, 0) AS published,
	bc.deleted, bc.created_at, bc.updated_at
FROM blog_catalog bc
LEFT JOIN blog_publish_state bps
	ON bps.blog_id = bc.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BlogCatalog, 0)
	for rows.Next() {
		var item BlogCatalog
		var sectionID sql.NullString
		var published, deleted int
		var createdAt, updatedAt string
		if err := rows.Scan(&item.ID, &item.JobID, &sectionID, &item.Title, &published, &deleted, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		item.SectionID = sectionID.String
		item.Published = published == 1
		item.Deleted = deleted == 1
		ct, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		ut, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, err
		}
		item.CreatedAt = ct
		item.UpdatedAt = ut
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetBlogCatalogByJobID(ctx context.Context, jobID string) (BlogCatalog, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
	bc.id, bc.job_id, bc.section_id, bc.title,
	COALESCE(bps.published, 0) AS published,
	bc.deleted, bc.created_at, bc.updated_at
FROM blog_catalog bc
LEFT JOIN blog_publish_state bps
	ON bps.blog_id = bc.id
WHERE bc.job_id = ?`, jobID)

	var item BlogCatalog
	var sectionID sql.NullString
	var published, deleted int
	var createdAt, updatedAt string
	if err := row.Scan(&item.ID, &item.JobID, &sectionID, &item.Title, &published, &deleted, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BlogCatalog{}, ErrNotFound
		}
		return BlogCatalog{}, err
	}
	item.SectionID = sectionID.String
	item.Published = published == 1
	item.Deleted = deleted == 1
	ct, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return BlogCatalog{}, err
	}
	ut, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return BlogCatalog{}, err
	}
	item.CreatedAt = ct
	item.UpdatedAt = ut
	return item, nil
}

func (s *Store) UpdateBlogCatalogMetadata(ctx context.Context, blogID, title, sectionID string) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE blog_catalog
SET title = ?, section_id = ?, updated_at = ?
WHERE id = ?`,
		title,
		nullableString(sectionID),
		nowRFC3339(),
		blogID,
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

func (s *Store) SetBlogCatalogSection(ctx context.Context, blogID, sectionID string) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE blog_catalog
SET section_id = ?, updated_at = ?
WHERE id = ?`,
		nullableString(sectionID),
		nowRFC3339(),
		blogID,
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

func (s *Store) SetBlogCatalogDeleted(ctx context.Context, blogID string, deleted bool) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE blog_catalog
SET deleted = ?, updated_at = ?
WHERE id = ?`,
		boolToInt(deleted),
		nowRFC3339(),
		blogID,
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

func (s *Store) SetBlogCatalogPublished(ctx context.Context, blogID string, published bool) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO blog_publish_state (blog_id, published, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(blog_id) DO UPDATE SET
	published = excluded.published,
	updated_at = excluded.updated_at`,
		blogID,
		boolToInt(published),
		nowRFC3339(),
	)
	return err
}

func (s *Store) ListBlogContentOverrides(ctx context.Context) ([]BlogContentOverride, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, blog_id, language, markdown, updated_at
FROM blog_content_overrides`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BlogContentOverride, 0)
	for rows.Next() {
		var item BlogContentOverride
		var updatedAt string
		if err := rows.Scan(&item.ID, &item.BlogID, &item.Language, &item.Markdown, &updatedAt); err != nil {
			return nil, err
		}
		ut, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt = ut
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpsertBlogContentOverride(ctx context.Context, item BlogContentOverride) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO blog_content_overrides (id, blog_id, language, markdown, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(blog_id, language) DO UPDATE SET
	markdown = excluded.markdown,
	updated_at = excluded.updated_at`,
		item.ID,
		item.BlogID,
		item.Language,
		item.Markdown,
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) DeleteBlogContentOverride(ctx context.Context, blogID, language string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM blog_content_overrides
WHERE blog_id = ? AND language = ?`, blogID, language)
	return err
}

func (s *Store) CreateAdminUser(ctx context.Context, user AdminUser) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO admin_users (id, username, password_hash, created_at)
VALUES (?, ?, ?, ?)`,
		user.ID,
		user.Username,
		user.PasswordHash,
		user.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) GetAdminUserByUsername(ctx context.Context, username string) (AdminUser, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, created_at
FROM admin_users
WHERE username = ?`, username)

	var user AdminUser
	var createdAt string
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AdminUser{}, ErrNotFound
		}
		return AdminUser{}, err
	}
	ct, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return AdminUser{}, err
	}
	user.CreatedAt = ct
	return user, nil
}

func (s *Store) GetAdminUserByID(ctx context.Context, id string) (AdminUser, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, created_at
FROM admin_users
WHERE id = ?`, id)

	var user AdminUser
	var createdAt string
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AdminUser{}, ErrNotFound
		}
		return AdminUser{}, err
	}
	ct, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return AdminUser{}, err
	}
	user.CreatedAt = ct
	return user, nil
}

func (s *Store) CreateAdminSession(ctx context.Context, session AdminSession) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO admin_sessions (id, user_id, token_hash, expires_at, created_at, revoked_at)
VALUES (?, ?, ?, ?, ?, NULL)`,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt.UTC().Format(time.RFC3339Nano),
		session.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) GetAdminSessionByTokenHash(ctx context.Context, tokenHash string) (AdminSession, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
FROM admin_sessions
WHERE token_hash = ?`, tokenHash)

	var session AdminSession
	var expiresAt, createdAt string
	var revokedAt sql.NullString
	if err := row.Scan(&session.ID, &session.UserID, &session.TokenHash, &expiresAt, &createdAt, &revokedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AdminSession{}, ErrNotFound
		}
		return AdminSession{}, err
	}
	et, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return AdminSession{}, err
	}
	ct, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return AdminSession{}, err
	}
	session.ExpiresAt = et
	session.CreatedAt = ct
	session.RevokedAt = revokedAt.String
	return session, nil
}

func (s *Store) RevokeAdminSessionByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE admin_sessions
SET revoked_at = ?
WHERE token_hash = ? AND revoked_at IS NULL`,
		nowRFC3339(),
		tokenHash,
	)
	return err
}
