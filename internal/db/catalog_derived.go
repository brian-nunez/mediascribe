package db

import "context"

func (s *Store) UpsertBlogCatalogDerived(ctx context.Context, blogID, previewText, languagesJSON string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO blog_catalog_derived (blog_id, preview_text, languages_json, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(blog_id) DO UPDATE SET
  preview_text = excluded.preview_text,
  languages_json = excluded.languages_json,
  updated_at = excluded.updated_at
`, blogID, previewText, languagesJSON, nowRFC3339())
	return err
}
