package jobs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brian-nunez/video-to-blog-page/internal/db"
)

type ArtifactSyncResult struct {
	BlogsScanned int `json:"blogs_scanned"`
	Updated      int `json:"updated"`
}

func (s *Service) SyncArtifactMetadata(ctx context.Context) (ArtifactSyncResult, error) {
	if err := s.ensureCatalogForReadyJobs(ctx); err != nil {
		return ArtifactSyncResult{}, err
	}
	catalog, err := s.Store.ListBlogCatalog(ctx)
	if err != nil {
		return ArtifactSyncResult{}, err
	}
	jobs, err := s.Store.ListJobs(ctx)
	if err != nil {
		return ArtifactSyncResult{}, err
	}
	jobByID := make(map[string]string, len(jobs))
	for _, j := range jobs {
		jobByID[j.ID] = j.ArtifactDir
	}

	result := ArtifactSyncResult{BlogsScanned: len(catalog)}
	for _, c := range catalog {
		dir := strings.TrimSpace(jobByID[c.JobID])
		if dir == "" {
			continue
		}
		preview, langs := blogPreviewAndLanguages(dir)
		if strings.TrimSpace(preview) == "" && len(langs) == 0 {
			continue
		}
		payload, _ := json.Marshal(langs)
		if err := s.Store.UpsertBlogCatalogDerived(ctx, c.ID, preview, string(payload)); err != nil {
			return result, err
		}
		if raw, err := os.ReadFile(filepath.Join(dir, "final.md")); err == nil {
			_ = s.Store.UpsertBlogContentCache(ctx, db.BlogContentCache{
				BlogID:    c.ID,
				Language:  OriginalLanguage,
				Markdown:  string(raw),
				UpdatedAt: time.Now().UTC(),
			})
		}
		matches, _ := filepath.Glob(filepath.Join(dir, "final.*.md"))
		for _, path := range matches {
			base := filepath.Base(path)
			if base == "final.md" {
				continue
			}
			lang := strings.TrimSuffix(strings.TrimPrefix(base, "final."), ".md")
			lang = normalizeLanguage(lang)
			if lang == "" {
				continue
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			_ = s.Store.UpsertBlogContentCache(ctx, db.BlogContentCache{
				BlogID:    c.ID,
				Language:  lang,
				Markdown:  string(raw),
				UpdatedAt: time.Now().UTC(),
			})
		}
		result.Updated++
	}
	return result, nil
}
