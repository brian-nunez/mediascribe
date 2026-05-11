package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/brian-nunez/video-to-blog-page/internal/artifacts"
	"github.com/brian-nunez/video-to-blog-page/internal/db"
	"github.com/brian-nunez/video-to-blog-page/internal/embeddings"
	"github.com/brian-nunez/video-to-blog-page/internal/ollama"
	"github.com/brian-nunez/video-to-blog-page/internal/pipeline"
	"github.com/brian-nunez/video-to-blog-page/internal/translation"
)

var ErrNotReady = errors.New("artifact not ready")

type Service struct {
	Store     *db.Store
	Runner    pipeline.Runner
	Artifacts artifacts.Manager
	defaults  Defaults

	mu      sync.Mutex
	running map[string]struct{}
}

type Defaults struct {
	MainModel               string
	MainModelBaseURL        string
	EmbeddingModel          string
	EmbeddingModelBaseURL   string
	TranslationModel        string
	TranslationModelBaseURL string
	EnableTranslation       bool
}

type CreateJobRequest struct {
	SourceType string `json:"source_type"`
	SourceURL  string `json:"source_url"`
	SourcePath string `json:"source_path"`
	Title      string `json:"title"`

	MainModel               string `json:"main_model"`
	MainModelBaseURL        string `json:"main_model_base_url"`
	EmbeddingModel          string `json:"embedding_model"`
	EmbeddingModelBaseURL   string `json:"embedding_model_base_url"`
	TranslationModel        string `json:"translation_model"`
	TranslationModelBaseURL string `json:"translation_model_base_url"`

	TranslationEnabled  *bool  `json:"translation_enabled"`
	TranslationLanguage string `json:"translation_language"`
}

type SearchResult struct {
	JobID            string  `json:"job_id"`
	JobTitle         string  `json:"job_title"`
	JobStatus        string  `json:"job_status"`
	ChunkID          string  `json:"chunk_id"`
	ChunkIndex       int     `json:"chunk_index"`
	StartTimeSeconds float64 `json:"start_time_seconds"`
	EndTimeSeconds   float64 `json:"end_time_seconds"`
	Content          string  `json:"content"`
	Score            float64 `json:"score"`
}

type TranslationInfo struct {
	Language  string    `json:"language"`
	Filename  string    `json:"filename"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewService(store *db.Store, runner pipeline.Runner, mgr artifacts.Manager, defaults Defaults) *Service {
	return &Service{
		Store:     store,
		Runner:    runner,
		Artifacts: mgr,
		defaults:  defaults,
		running:   map[string]struct{}{},
	}
}

func (s *Service) CreateJob(ctx context.Context, req CreateJobRequest) (db.Job, error) {
	sourceType := strings.ToLower(strings.TrimSpace(req.SourceType))
	if sourceType != "url" && sourceType != "path" {
		return db.Job{}, fmt.Errorf("source_type must be 'url' or 'path'")
	}
	if sourceType == "url" && strings.TrimSpace(req.SourceURL) == "" {
		return db.Job{}, fmt.Errorf("source_url is required for source_type=url")
	}
	if sourceType == "path" && strings.TrimSpace(req.SourcePath) == "" {
		return db.Job{}, fmt.Errorf("source_path is required for source_type=path")
	}

	jobID := uuid.NewString()
	artifactDir, err := s.Artifacts.EnsureJobDir(jobID)
	if err != nil {
		return db.Job{}, err
	}

	translationEnabled := s.defaults.EnableTranslation
	if req.TranslationEnabled != nil {
		translationEnabled = *req.TranslationEnabled
	}

	now := time.Now().UTC()
	job := db.Job{
		ID:                      jobID,
		SourceType:              sourceType,
		SourceURL:               strings.TrimSpace(req.SourceURL),
		SourcePath:              strings.TrimSpace(req.SourcePath),
		Title:                   strings.TrimSpace(req.Title),
		Status:                  "queued",
		CurrentStage:            pipeline.StageCreateJob,
		ArtifactDir:             artifactDir,
		MainModel:               choose(req.MainModel, s.defaults.MainModel),
		MainModelBaseURL:        choose(req.MainModelBaseURL, s.defaults.MainModelBaseURL),
		EmbeddingModel:          choose(req.EmbeddingModel, s.defaults.EmbeddingModel),
		EmbeddingModelBaseURL:   choose(req.EmbeddingModelBaseURL, s.defaults.EmbeddingModelBaseURL),
		TranslationModel:        choose(req.TranslationModel, s.defaults.TranslationModel),
		TranslationModelBaseURL: choose(req.TranslationModelBaseURL, s.defaults.TranslationModelBaseURL),
		TranslationEnabled:      translationEnabled,
		TranslationLanguage:     strings.TrimSpace(req.TranslationLanguage),
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	if job.TranslationEnabled && job.TranslationLanguage == "" {
		job.TranslationLanguage = "es"
	}

	if err := s.Store.CreateJob(ctx, job); err != nil {
		return db.Job{}, err
	}
	s.start(job.ID)
	return job, nil
}

func (s *Service) RetryJob(ctx context.Context, jobID string) error {
	if _, err := s.Store.GetJob(ctx, jobID); err != nil {
		return err
	}
	if err := s.Store.ResetJobForRetry(ctx, jobID); err != nil {
		return err
	}
	s.start(jobID)
	return nil
}

func (s *Service) ListJobs(ctx context.Context) ([]db.Job, error) {
	return s.Store.ListJobs(ctx)
}

func (s *Service) GetJob(ctx context.Context, jobID string) (db.Job, error) {
	return s.Store.GetJob(ctx, jobID)
}

func (s *Service) GetTranscript(ctx context.Context, jobID string) (string, error) {
	job, err := s.Store.GetJob(ctx, jobID)
	if err != nil {
		return "", err
	}
	transcriptPath := filepath.Join(job.ArtifactDir, "transcript.json")
	if data, err := os.ReadFile(transcriptPath); err == nil {
		return string(data), nil
	}

	chunks, err := s.Store.ListTranscriptChunks(ctx, jobID)
	if err != nil {
		return "", err
	}
	if len(chunks) == 0 {
		return "", ErrNotReady
	}
	payload, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (s *Service) GetBlogMarkdown(ctx context.Context, jobID, language string) (string, string, error) {
	job, err := s.Store.GetJob(ctx, jobID)
	if err != nil {
		return "", "", err
	}

	lang := strings.TrimSpace(language)
	if lang != "" {
		translatedPath := filepath.Join(job.ArtifactDir, "final."+sanitizeLang(lang)+".md")
		raw, err := os.ReadFile(translatedPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", "", ErrNotReady
			}
			return "", "", err
		}
		return string(raw), filepath.Base(translatedPath), nil
	}

	if job.TranslationEnabled && strings.TrimSpace(job.TranslationLanguage) != "" {
		translatedPath := filepath.Join(job.ArtifactDir, "final."+sanitizeLang(job.TranslationLanguage)+".md")
		if raw, err := os.ReadFile(translatedPath); err == nil {
			return string(raw), filepath.Base(translatedPath), nil
		}
	}

	finalPath := filepath.Join(job.ArtifactDir, "final.md")
	raw, err := os.ReadFile(finalPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrNotReady
		}
		return "", "", err
	}
	return string(raw), filepath.Base(finalPath), nil
}

func (s *Service) TranslateCompletedBlog(ctx context.Context, jobID, language string) (TranslationInfo, error) {
	lang := strings.TrimSpace(language)
	if lang == "" {
		return TranslationInfo{}, fmt.Errorf("language is required")
	}

	job, err := s.Store.GetJob(ctx, jobID)
	if err != nil {
		return TranslationInfo{}, err
	}

	finalPath := filepath.Join(job.ArtifactDir, "final.md")
	raw, err := os.ReadFile(finalPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TranslationInfo{}, ErrNotReady
		}
		return TranslationInfo{}, err
	}

	client := ollama.NewClientWithRetry(
		job.TranslationModelBaseURL,
		s.Runner.TranslateModelTimeout,
		s.Runner.TranslateMaxRetries,
		s.Runner.ModelRetryBackoff,
	)
	translator := translation.OllamaTranslator{
		Client: client,
		Model:  job.TranslationModel,
	}
	translated, err := translator.TranslateMarkdown(ctx, string(raw), lang)
	if err != nil {
		return TranslationInfo{}, err
	}

	filename := fmt.Sprintf("final.%s.md", sanitizeLang(lang))
	path := filepath.Join(job.ArtifactDir, filename)
	if err := os.WriteFile(path, []byte(translated), 0o644); err != nil {
		return TranslationInfo{}, err
	}

	if out, err := s.Store.GetBlogOutputByJob(ctx, jobID); err == nil {
		out.TranslatedMarkdownPath = path
		out.TranslationLanguage = lang
		out.UpdatedAt = time.Now().UTC()
		_ = s.Store.UpsertBlogOutput(ctx, out)
	}

	info, err := os.Stat(path)
	if err != nil {
		return TranslationInfo{}, err
	}
	return TranslationInfo{
		Language:  sanitizeLang(lang),
		Filename:  filename,
		Path:      path,
		UpdatedAt: info.ModTime().UTC(),
	}, nil
}

func (s *Service) ListTranslations(ctx context.Context, jobID string) ([]TranslationInfo, error) {
	job, err := s.Store.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(job.ArtifactDir, "final.*.md"))
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`^final\.([^.]+)\.md$`)
	out := make([]TranslationInfo, 0, len(matches))
	for _, path := range matches {
		base := filepath.Base(path)
		if base == "final.md" {
			continue
		}
		m := re.FindStringSubmatch(base)
		if len(m) != 2 {
			continue
		}
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		out = append(out, TranslationInfo{
			Language:  m[1],
			Filename:  base,
			Path:      path,
			UpdatedAt: st.ModTime().UTC(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *Service) SearchChunks(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []SearchResult{}, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	records, err := s.Store.ListSearchChunkRecords(ctx, s.defaults.EmbeddingModel, s.defaults.EmbeddingModelBaseURL)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return []SearchResult{}, nil
	}

	queryLower := strings.ToLower(q)
	queryTerms := tokenizeQuery(queryLower)

	var queryVec []float32
	semanticEnabled := true
	client := ollama.NewClientWithRetry(
		s.defaults.EmbeddingModelBaseURL,
		s.Runner.EmbeddingModelTimeout,
		s.Runner.EmbeddingMaxRetries,
		s.Runner.ModelRetryBackoff,
	)
	queryVec, err = client.Embed(ctx, s.defaults.EmbeddingModel, q)
	if err != nil {
		// Fall back to lexical search when query embedding is unavailable.
		semanticEnabled = false
	}

	out := make([]SearchResult, 0, len(records))
	for _, rec := range records {
		contentLower := strings.ToLower(rec.Content)
		lexical := lexicalScore(queryLower, queryTerms, contentLower)
		semantic := 0.0

		if semanticEnabled {
			vec, err := embeddings.BytesToFloat32(rec.Embedding)
			if err == nil && len(vec) == len(queryVec) && len(vec) > 0 {
				semantic = cosineSimilarity(queryVec, vec)
			}
		}

		// Hybrid score: semantic ranking + lexical boost for exact-word retrieval.
		score := semantic + lexical
		if score <= 0 {
			continue
		}

		out = append(out, SearchResult{
			JobID:            rec.JobID,
			JobTitle:         rec.JobTitle,
			JobStatus:        rec.JobStatus,
			ChunkID:          rec.ChunkID,
			ChunkIndex:       rec.ChunkIndex,
			StartTimeSeconds: rec.StartTimeSeconds,
			EndTimeSeconds:   rec.EndTimeSeconds,
			Content:          rec.Content,
			Score:            score,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Service) start(jobID string) {
	s.mu.Lock()
	if _, exists := s.running[jobID]; exists {
		s.mu.Unlock()
		return
	}
	s.running[jobID] = struct{}{}
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, jobID)
			s.mu.Unlock()
		}()

		if err := s.Runner.Run(context.Background(), jobID); err != nil {
			log.Printf("job %s failed: %v", jobID, err)
		}
	}()
}

func choose(value, fallback string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return fallback
	}
	return v
}

func sanitizeLang(value string) string {
	clean := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "/", "", "\\", "", ".", "", "..", "")
	clean = replacer.Replace(clean)
	if clean == "" {
		return "translated"
	}
	return clean
}

func cosineSimilarity(a, b []float32) float64 {
	var dot float64
	var aNorm float64
	var bNorm float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		aNorm += av * av
		bNorm += bv * bv
	}
	if aNorm == 0 || bNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(aNorm) * math.Sqrt(bNorm))
}

func tokenizeQuery(q string) []string {
	parts := strings.Fields(q)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func lexicalScore(queryLower string, terms []string, contentLower string) float64 {
	score := 0.0
	if strings.Contains(contentLower, queryLower) {
		score += 2.0
	}
	for _, term := range terms {
		if term == "" {
			continue
		}
		hits := strings.Count(contentLower, term)
		if hits > 0 {
			// First hit gets strong boost; repeats get diminishing extra score.
			score += 1.0 + math.Min(0.75, float64(hits-1)*0.15)
		}
	}
	return score
}
