package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/brian-nunez/video-to-blog-page/internal/db"
)

const OriginalLanguage = "en"

type BlogLanguageContent struct {
	Language  string    `json:"language"`
	Markdown  string    `json:"markdown"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BlogView struct {
	ID          string                `json:"id"`
	JobID       string                `json:"job_id"`
	Title       string                `json:"title"`
	SourceURL   string                `json:"source_url,omitempty"`
	SourcePath  string                `json:"source_path,omitempty"`
	Status      string                `json:"status"`
	Transcript  string                `json:"transcript"`
	Languages   []BlogLanguageContent `json:"languages"`
	SectionID   string                `json:"section_id,omitempty"`
	SectionName string                `json:"section_name,omitempty"`
	Published   bool                  `json:"published"`
	Deleted     bool                  `json:"deleted"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

type SectionWithBlogs struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	SortOrder int        `json:"sort_order"`
	Blogs     []BlogView `json:"blogs"`
}

type PublicCatalog struct {
	Sections    []SectionWithBlogs `json:"sections"`
	Unsectioned []BlogView         `json:"unsectioned"`
}

type PublicSectionSummary struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type PublicBlogSummary struct {
	ID          string    `json:"id"`
	JobID       string    `json:"job_id"`
	Title       string    `json:"title"`
	SectionID   string    `json:"section_id,omitempty"`
	SectionName string    `json:"section_name,omitempty"`
	SourceURL   string    `json:"source_url,omitempty"`
	SourcePath  string    `json:"source_path,omitempty"`
	Status      string    `json:"status"`
	Preview     string    `json:"preview"`
	Languages   []string  `json:"languages"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PublicFeedPage struct {
	Items          []PublicBlogSummary    `json:"items"`
	Sections       []PublicSectionSummary `json:"sections"`
	LanguageCounts map[string]int         `json:"language_counts"`
	Offset         int                    `json:"offset"`
	Limit          int                    `json:"limit"`
	Total          int                    `json:"total"`
	HasMore        bool                   `json:"has_more"`
	NextOffset     int                    `json:"next_offset"`
}

func (s *Service) ListPublicCatalog(ctx context.Context) (PublicCatalog, error) {
	sections, blogs, err := s.listCatalogViews(ctx, false, false)
	if err != nil {
		return PublicCatalog{}, err
	}
	return PublicCatalog{
		Sections:    sections,
		Unsectioned: blogs,
	}, nil
}

func (s *Service) ListPublicFeedPage(ctx context.Context, sectionID string, limit, offset int) (PublicFeedPage, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	mode := strings.TrimSpace(sectionID)
	var (
		rows  []db.PublicBlogRow
		total int
		err   error
	)
	switch mode {
	case "", "all":
		rows, err = s.Store.ListAllPublicBlogPage(ctx, limit, offset)
		if err != nil {
			return PublicFeedPage{}, err
		}
		total, err = s.Store.CountAllPublicBlogs(ctx)
	case "unsectioned":
		rows, err = s.Store.ListPublicBlogPage(ctx, "", limit, offset)
		if err != nil {
			return PublicFeedPage{}, err
		}
		total, err = s.Store.CountPublicBlogsInSection(ctx, "")
	default:
		rows, err = s.Store.ListPublicBlogPage(ctx, mode, limit, offset)
		if err != nil {
			return PublicFeedPage{}, err
		}
		total, err = s.Store.CountPublicBlogsInSection(ctx, mode)
	}
	if err != nil {
		return PublicFeedPage{}, err
	}
	sections, err := s.publicSectionsWithCounts(ctx)
	if err != nil {
		return PublicFeedPage{}, err
	}

	items := make([]PublicBlogSummary, 0, len(rows))
	for _, row := range rows {
		preview, langs := row.PreviewText, parseLanguagesJSON(row.LanguagesJSON)
		sectionName := row.SectionName
		if strings.TrimSpace(sectionName) == "" {
			sectionName = "Unsectioned"
		}
		items = append(items, PublicBlogSummary{
			ID:          row.BlogID,
			JobID:       row.JobID,
			Title:       row.Title,
			SectionID:   row.SectionID,
			SectionName: sectionName,
			SourceURL:   row.SourceURL,
			SourcePath:  row.SourcePath,
			Status:      row.Status,
			Preview:     preview,
			Languages:   langs,
			UpdatedAt:   row.BlogUpdated,
		})
	}

	nextOffset := offset + len(items)
	langCountsList, err := s.Store.CountPublishedLanguages(ctx, mode)
	if err != nil {
		return PublicFeedPage{}, err
	}
	langCounts := make(map[string]int, len(langCountsList))
	for _, lc := range langCountsList {
		lang := strings.TrimSpace(strings.ToLower(lc.Language))
		if lang == "" {
			continue
		}
		langCounts[lang] = lc.Count
	}
	return PublicFeedPage{
		Items:          items,
		Sections:       sections,
		LanguageCounts: langCounts,
		Offset:         offset,
		Limit:          limit,
		Total:          total,
		HasMore:        nextOffset < total,
		NextOffset:     nextOffset,
	}, nil
}

func (s *Service) GetPublicBlogByID(ctx context.Context, blogID string) (BlogView, error) {
	item, err := s.GetBlogByCatalogID(ctx, blogID, false)
	if err != nil {
		return BlogView{}, err
	}
	if !item.Published || item.Deleted {
		return BlogView{}, db.ErrNotFound
	}
	return item, nil
}

func (s *Service) ListAdminCatalog(ctx context.Context) (PublicCatalog, []BlogView, error) {
	if err := s.ensureCatalogForReadyJobs(ctx); err != nil {
		return PublicCatalog{}, nil, err
	}
	sections, unsectioned, err := s.listCatalogViews(ctx, true, true)
	if err != nil {
		return PublicCatalog{}, nil, err
	}
	all, err := s.listAllBlogViews(ctx)
	if err != nil {
		return PublicCatalog{}, nil, err
	}
	return PublicCatalog{
		Sections:    sections,
		Unsectioned: unsectioned,
	}, all, nil
}

func (s *Service) ListSections(ctx context.Context) ([]db.Section, error) {
	return s.Store.ListSections(ctx)
}

func (s *Service) CreateSection(ctx context.Context, name string) (db.Section, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return db.Section{}, fmt.Errorf("name is required")
	}
	items, err := s.Store.ListSections(ctx)
	if err != nil {
		return db.Section{}, err
	}
	maxSort := 0
	for _, item := range items {
		if item.SortOrder > maxSort {
			maxSort = item.SortOrder
		}
	}
	now := time.Now().UTC()
	out := db.Section{
		ID:        uuid.NewString(),
		Name:      n,
		SortOrder: maxSort + 10,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.Store.CreateSection(ctx, out); err != nil {
		return db.Section{}, err
	}
	return out, nil
}

func (s *Service) UpdateSection(ctx context.Context, id, name string, sortOrder int) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return fmt.Errorf("name is required")
	}
	return s.Store.UpdateSection(ctx, db.Section{
		ID:        id,
		Name:      n,
		SortOrder: sortOrder,
		UpdatedAt: time.Now().UTC(),
	})
}

func (s *Service) DeleteSection(ctx context.Context, id string) error {
	return s.Store.DeleteSection(ctx, id)
}

func (s *Service) UpdateBlogMetadata(ctx context.Context, blogID, title, sectionID string) error {
	t := strings.TrimSpace(title)
	if t == "" {
		return fmt.Errorf("title is required")
	}
	return s.Store.UpdateBlogCatalogMetadata(ctx, blogID, t, strings.TrimSpace(sectionID))
}

func (s *Service) MoveBlogToSection(ctx context.Context, blogID, sectionID string) error {
	return s.Store.SetBlogCatalogSection(ctx, blogID, strings.TrimSpace(sectionID))
}

func (s *Service) DeleteBlog(ctx context.Context, blogID string) error {
	return s.Store.SetBlogCatalogDeleted(ctx, blogID, true)
}

func (s *Service) RestoreBlog(ctx context.Context, blogID string) error {
	return s.Store.SetBlogCatalogDeleted(ctx, blogID, false)
}

func (s *Service) SetBlogPublished(ctx context.Context, blogID string, published bool) error {
	return s.Store.SetBlogCatalogPublished(ctx, blogID, published)
}

func (s *Service) UpdateBlogLanguageContent(ctx context.Context, blogID, language, markdown string) error {
	lang := normalizeLanguage(language)
	if lang == "" {
		return fmt.Errorf("language is required")
	}
	if strings.TrimSpace(markdown) == "" {
		return s.Store.DeleteBlogContentOverride(ctx, blogID, lang)
	}
	return s.Store.UpsertBlogContentOverride(ctx, db.BlogContentOverride{
		ID:        uuid.NewString(),
		BlogID:    blogID,
		Language:  lang,
		Markdown:  markdown,
		UpdatedAt: time.Now().UTC(),
	})
}

func (s *Service) GetBlogByCatalogID(ctx context.Context, blogID string, includeDeleted bool) (BlogView, error) {
	blogs, err := s.listAllBlogViews(ctx)
	if err != nil {
		return BlogView{}, err
	}
	for _, item := range blogs {
		if item.ID != blogID {
			continue
		}
		if item.Deleted && !includeDeleted {
			return BlogView{}, db.ErrNotFound
		}
		return item, nil
	}
	return BlogView{}, db.ErrNotFound
}

func (s *Service) ensureCatalogForReadyJobs(ctx context.Context) error {
	jobs, err := s.Store.ListJobs(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job.Status != "complete" {
			continue
		}
		finalPath := filepath.Join(job.ArtifactDir, "final.md")
		if _, err := os.Stat(finalPath); err != nil {
			continue
		}

		title := strings.TrimSpace(job.Title)
		if title == "" {
			title = "Tech Blog " + job.ID[:8]
		}

		_, err = s.Store.GetBlogCatalogByJobID(ctx, job.ID)
		if errors.Is(err, db.ErrNotFound) {
			now := time.Now().UTC()
			if err := s.Store.UpsertBlogCatalog(ctx, db.BlogCatalog{
				ID:        uuid.NewString(),
				JobID:     job.ID,
				Title:     title,
				Deleted:   false,
				CreatedAt: now,
				UpdatedAt: now,
			}); err != nil {
				return err
			}
			if err := s.syncDerivedForBlog(ctx, job.ArtifactDir, job.ID); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := s.syncDerivedForBlog(ctx, job.ArtifactDir, job.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) syncDerivedForBlog(ctx context.Context, artifactDir, jobID string) error {
	blog, err := s.Store.GetBlogCatalogByJobID(ctx, jobID)
	if err != nil {
		return err
	}
	preview, langs := blogPreviewAndLanguages(artifactDir)
	payload, _ := json.Marshal(langs)
	if err := s.Store.UpsertBlogCatalogDerived(ctx, blog.ID, preview, string(payload)); err != nil {
		return err
	}
	_ = s.syncContentCacheForBlog(ctx, blog.ID, artifactDir)
	return nil
}

func (s *Service) syncContentCacheForBlog(ctx context.Context, blogID, artifactDir string) error {
	if raw, err := os.ReadFile(filepath.Join(artifactDir, "final.md")); err == nil {
		_ = s.Store.UpsertBlogContentCache(ctx, db.BlogContentCache{
			BlogID:    blogID,
			Language:  OriginalLanguage,
			Markdown:  string(raw),
			UpdatedAt: time.Now().UTC(),
		})
	}
	matches, _ := filepath.Glob(filepath.Join(artifactDir, "final.*.md"))
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
			BlogID:    blogID,
			Language:  lang,
			Markdown:  string(raw),
			UpdatedAt: time.Now().UTC(),
		})
	}
	return nil
}

func (s *Service) listCatalogViews(ctx context.Context, includeDeleted, includeUnpublished bool) ([]SectionWithBlogs, []BlogView, error) {
	sections, err := s.Store.ListSections(ctx)
	if err != nil {
		return nil, nil, err
	}
	allBlogs, err := s.listAllBlogViews(ctx)
	if err != nil {
		return nil, nil, err
	}

	bySection := make(map[string][]BlogView)
	unsectioned := make([]BlogView, 0)
	for _, blog := range allBlogs {
		if blog.Deleted && !includeDeleted {
			continue
		}
		if !blog.Published && !includeUnpublished {
			continue
		}
		if strings.TrimSpace(blog.SectionID) == "" {
			unsectioned = append(unsectioned, blog)
			continue
		}
		bySection[blog.SectionID] = append(bySection[blog.SectionID], blog)
	}

	outSections := make([]SectionWithBlogs, 0, len(sections))
	for _, section := range sections {
		outSections = append(outSections, SectionWithBlogs{
			ID:        section.ID,
			Name:      section.Name,
			SortOrder: section.SortOrder,
			Blogs:     sortedBlogs(bySection[section.ID]),
		})
	}

	sort.Slice(outSections, func(i, j int) bool {
		if outSections[i].SortOrder == outSections[j].SortOrder {
			return strings.ToLower(outSections[i].Name) < strings.ToLower(outSections[j].Name)
		}
		return outSections[i].SortOrder < outSections[j].SortOrder
	})
	return outSections, sortedBlogs(unsectioned), nil
}

func (s *Service) listAllBlogViews(ctx context.Context) ([]BlogView, error) {
	catalog, err := s.Store.ListBlogCatalog(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := s.Store.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	sections, err := s.Store.ListSections(ctx)
	if err != nil {
		return nil, err
	}
	overrides, err := s.Store.ListBlogContentOverrides(ctx)
	if err != nil {
		return nil, err
	}
	cachedContent, err := s.Store.ListBlogContentCache(ctx)
	if err != nil {
		return nil, err
	}

	jobByID := make(map[string]db.Job, len(jobs))
	for _, job := range jobs {
		jobByID[job.ID] = job
	}
	sectionNameByID := make(map[string]string, len(sections))
	for _, sec := range sections {
		sectionNameByID[sec.ID] = sec.Name
	}
	overrideByKey := make(map[string]db.BlogContentOverride, len(overrides))
	for _, item := range overrides {
		key := item.BlogID + "::" + normalizeLanguage(item.Language)
		overrideByKey[key] = item
	}
	cacheByKey := make(map[string]db.BlogContentCache, len(cachedContent))
	for _, item := range cachedContent {
		key := item.BlogID + "::" + normalizeLanguage(item.Language)
		cacheByKey[key] = item
	}

	out := make([]BlogView, 0, len(catalog))
	for _, item := range catalog {
		job, ok := jobByID[item.JobID]
		if !ok {
			continue
		}
		languages, err := s.loadLanguageContents(job, item.ID, overrideByKey, cacheByKey)
		if err != nil {
			return nil, err
		}
		transcript := ""
		if payload, err := s.GetTranscript(ctx, job.ID); err == nil {
			transcript = normalizeTranscriptForDisplay(payload)
		}
		out = append(out, BlogView{
			ID:          item.ID,
			JobID:       job.ID,
			Title:       item.Title,
			SourceURL:   job.SourceURL,
			SourcePath:  job.SourcePath,
			Status:      job.Status,
			Transcript:  transcript,
			Languages:   languages,
			SectionID:   item.SectionID,
			SectionName: sectionNameByID[item.SectionID],
			Published:   item.Published,
			Deleted:     item.Deleted,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) loadLanguageContents(job db.Job, blogID string, overrideByKey map[string]db.BlogContentOverride, cacheByKey map[string]db.BlogContentCache) ([]BlogLanguageContent, error) {
	entries := make(map[string]BlogLanguageContent)

	originalPath := filepath.Join(job.ArtifactDir, "final.md")
	if raw, err := os.ReadFile(originalPath); err == nil {
		st, _ := os.Stat(originalPath)
		entries[OriginalLanguage] = BlogLanguageContent{
			Language:  OriginalLanguage,
			Markdown:  string(raw),
			UpdatedAt: statTimeOrNow(st),
		}
	}

	matches, err := filepath.Glob(filepath.Join(job.ArtifactDir, "final.*.md"))
	if err != nil {
		return nil, err
	}
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
		st, _ := os.Stat(path)
		entries[lang] = BlogLanguageContent{
			Language:  lang,
			Markdown:  string(raw),
			UpdatedAt: statTimeOrNow(st),
		}
	}

	for key, ov := range overrideByKey {
		if !strings.HasPrefix(key, blogID+"::") {
			continue
		}
		lang := normalizeLanguage(ov.Language)
		entries[lang] = BlogLanguageContent{
			Language:  lang,
			Markdown:  ov.Markdown,
			UpdatedAt: ov.UpdatedAt,
		}
	}
	for key, cv := range cacheByKey {
		if !strings.HasPrefix(key, blogID+"::") {
			continue
		}
		lang := normalizeLanguage(cv.Language)
		entries[lang] = BlogLanguageContent{
			Language:  lang,
			Markdown:  cv.Markdown,
			UpdatedAt: cv.UpdatedAt,
		}
	}

	out := make([]BlogLanguageContent, 0, len(entries))
	for _, item := range entries {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Language == OriginalLanguage {
			return true
		}
		if out[j].Language == OriginalLanguage {
			return false
		}
		return out[i].Language < out[j].Language
	})
	return out, nil
}

func statTimeOrNow(info os.FileInfo) time.Time {
	if info == nil {
		return time.Now().UTC()
	}
	return info.ModTime().UTC()
}

func normalizeLanguage(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" {
		return ""
	}
	if v == "en" {
		return OriginalLanguage
	}
	return v
}

func normalizeTranscriptForDisplay(raw string) string {
	var payload struct {
		Segments []struct {
			Start float64 `json:"start"`
			End   float64 `json:"end"`
			Text  string  `json:"text"`
		} `json:"segments"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || len(payload.Segments) == 0 {
		return raw
	}
	lines := make([]string, 0, len(payload.Segments))
	for _, seg := range payload.Segments {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("[%0.1fs-%0.1fs] %s", seg.Start, seg.End, text))
	}
	return strings.Join(lines, "\n")
}

func (s *Service) publicSectionsWithCounts(ctx context.Context) ([]PublicSectionSummary, error) {
	sections, err := s.Store.ListSections(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PublicSectionSummary, 0, len(sections)+1)
	for _, sec := range sections {
		n, err := s.Store.CountPublicBlogsInSection(ctx, sec.ID)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		out = append(out, PublicSectionSummary{ID: sec.ID, Name: sec.Name, Count: n})
	}
	unsectionedCount, err := s.Store.CountPublicBlogsInSection(ctx, "")
	if err != nil {
		return nil, err
	}
	if unsectionedCount > 0 {
		out = append(out, PublicSectionSummary{ID: "unsectioned", Name: "Unsectioned", Count: unsectionedCount})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func blogPreviewAndLanguages(artifactDir string) (string, []string) {
	finalPath := filepath.Join(artifactDir, "final.md")
	raw, _ := os.ReadFile(finalPath)
	preview := strings.TrimSpace(string(raw))
	if preview != "" {
		preview = strings.ReplaceAll(preview, "\n", " ")
		preview = strings.Join(strings.Fields(preview), " ")
		if len(preview) > 220 {
			preview = preview[:220] + "..."
		}
	}
	if preview == "" {
		preview = "No preview available."
	}

	langs := make([]string, 0, 4)
	if len(raw) > 0 {
		langs = append(langs, "en")
	}
	matches, _ := filepath.Glob(filepath.Join(artifactDir, "final.*.md"))
	for _, path := range matches {
		base := filepath.Base(path)
		if base == "final.md" {
			continue
		}
		lang := strings.TrimSuffix(strings.TrimPrefix(base, "final."), ".md")
		lang = normalizeLanguage(lang)
		if lang == "" || lang == "en" {
			continue
		}
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return preview, langs
}

func parseLanguagesJSON(raw string) []string {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		return nil
	}
	var langs []string
	if err := json.Unmarshal([]byte(payload), &langs); err != nil {
		return nil
	}
	out := make([]string, 0, len(langs))
	seen := map[string]struct{}{}
	for _, l := range langs {
		k := normalizeLanguage(l)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedBlogs(in []BlogView) []BlogView {
	out := append([]BlogView(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}
