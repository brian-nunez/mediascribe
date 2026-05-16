package db

import (
	"context"
	"time"
)

type BlogContentCache struct {
	BlogID    string
	Language  string
	Markdown  string
	UpdatedAt time.Time
}

func (s *Store) UpsertBlogContentCache(ctx context.Context, item BlogContentCache) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO blog_content_cache (blog_id, language, markdown, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(blog_id, language) DO UPDATE SET
  markdown = excluded.markdown,
  updated_at = excluded.updated_at
`, item.BlogID, item.Language, item.Markdown, item.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListBlogContentCache(ctx context.Context) ([]BlogContentCache, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT blog_id, language, markdown, updated_at FROM blog_content_cache`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BlogContentCache, 0)
	for rows.Next() {
		var item BlogContentCache
		var updatedAt string
		if err := rows.Scan(&item.BlogID, &item.Language, &item.Markdown, &updatedAt); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt = t
		out = append(out, item)
	}
	return out, rows.Err()
}
