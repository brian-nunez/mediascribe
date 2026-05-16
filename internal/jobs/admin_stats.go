package jobs

import (
	"context"

	"github.com/brian-nunez/video-to-blog-page/internal/db"
)

func (s *Service) AdminStats(ctx context.Context) (db.AdminStats, error) {
	return s.Store.GetAdminStats(ctx)
}
