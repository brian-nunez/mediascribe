package db

import "context"

type AdminStats struct {
	SectionsCount  int `json:"sections_count"`
	BlogsCount     int `json:"blogs_count"`
	PublishedCount int `json:"published_count"`
}

func (s *Store) GetAdminStats(ctx context.Context) (AdminStats, error) {
	var out AdminStats
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM sections`).Scan(&out.SectionsCount); err != nil {
		return AdminStats{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM blog_catalog WHERE deleted = 0`).Scan(&out.BlogsCount); err != nil {
		return AdminStats{}, err
	}
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM blog_catalog bc
LEFT JOIN blog_publish_state bps ON bps.blog_id = bc.id
WHERE bc.deleted = 0
  AND COALESCE(bps.published, 0) = 1`).Scan(&out.PublishedCount); err != nil {
		return AdminStats{}, err
	}
	return out, nil
}
