package jobs

import (
	"context"
	"strings"
)

type PublicSearchHit struct {
	BlogID      string   `json:"blog_id"`
	JobID       string   `json:"job_id"`
	Title       string   `json:"title"`
	SectionID   string   `json:"section_id,omitempty"`
	SectionName string   `json:"section_name,omitempty"`
	SourceURL   string   `json:"source_url,omitempty"`
	SourcePath  string   `json:"source_path,omitempty"`
	Preview     string   `json:"preview"`
	Languages   []string `json:"languages"`
	Score       float64  `json:"score"`
	MatchStart  float64  `json:"match_start_seconds"`
	MatchEnd    float64  `json:"match_end_seconds"`
}

func (s *Service) SearchPublicBlogs(ctx context.Context, query string, limit int) ([]PublicSearchHit, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	chunkResults, err := s.SearchChunks(ctx, query, limit*8)
	if err != nil {
		return nil, err
	}
	if len(chunkResults) == 0 {
		return []PublicSearchHit{}, nil
	}

	jobIDs := make([]string, 0, len(chunkResults))
	seenJob := map[string]struct{}{}
	for _, r := range chunkResults {
		jid := strings.TrimSpace(r.JobID)
		if jid == "" {
			continue
		}
		if _, ok := seenJob[jid]; ok {
			continue
		}
		seenJob[jid] = struct{}{}
		jobIDs = append(jobIDs, jid)
	}
	rows, err := s.Store.ListPublishedBlogRowsByJobIDs(ctx, jobIDs)
	if err != nil {
		return nil, err
	}
	rowByJob := make(map[string]PublicBlogSummary, len(rows))
	for _, row := range rows {
		preview, langs := row.PreviewText, parseLanguagesJSON(row.LanguagesJSON)
		if strings.TrimSpace(preview) == "" || len(langs) == 0 {
			preview, langs = blogPreviewAndLanguages(row.ArtifactDir)
		}
		name := row.SectionName
		if strings.TrimSpace(name) == "" {
			name = "Unsectioned"
		}
		rowByJob[row.JobID] = PublicBlogSummary{
			ID:          row.BlogID,
			JobID:       row.JobID,
			Title:       row.Title,
			SectionID:   row.SectionID,
			SectionName: name,
			SourceURL:   row.SourceURL,
			SourcePath:  row.SourcePath,
			Preview:     preview,
			Languages:   langs,
		}
	}

	out := make([]PublicSearchHit, 0, limit)
	usedBlog := map[string]struct{}{}
	for _, r := range chunkResults {
		s, ok := rowByJob[r.JobID]
		if !ok || strings.TrimSpace(s.ID) == "" {
			continue
		}
		if _, exists := usedBlog[s.ID]; exists {
			continue
		}
		usedBlog[s.ID] = struct{}{}
		out = append(out, PublicSearchHit{
			BlogID:      s.ID,
			JobID:       s.JobID,
			Title:       s.Title,
			SectionID:   s.SectionID,
			SectionName: s.SectionName,
			SourceURL:   s.SourceURL,
			SourcePath:  s.SourcePath,
			Preview:     s.Preview,
			Languages:   s.Languages,
			Score:       r.Score,
			MatchStart:  r.StartTimeSeconds,
			MatchEnd:    r.EndTimeSeconds,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
