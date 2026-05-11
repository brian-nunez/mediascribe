package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/brian-nunez/video-to-blog-page/internal/artifacts"
	"github.com/brian-nunez/video-to-blog-page/internal/db"
	"github.com/brian-nunez/video-to-blog-page/internal/pipeline"
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

func (s *Service) GetBlogMarkdown(ctx context.Context, jobID string) (string, string, error) {
	job, err := s.Store.GetJob(ctx, jobID)
	if err != nil {
		return "", "", err
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
