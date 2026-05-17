package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/brian-nunez/video-to-blog-page/internal/db"
	"github.com/brian-nunez/video-to-blog-page/internal/media"
)

type BatchQueueItemRequest struct {
	SourceType string `json:"source_type"`
	SourceURL  string `json:"source_url"`
	SourcePath string `json:"source_path"`
	Title      string `json:"title"`
	SectionID  string `json:"section_id"`
	MainModel  string `json:"main_model"`
}

type CreateBatchRequest struct {
	Name      string                  `json:"name"`
	Delay     string                  `json:"delay"`
	Items     []BatchQueueItemRequest `json:"items"`
	AutoStart bool                    `json:"auto_start"`
}

type BatchView struct {
	Batch db.JobBatch       `json:"batch"`
	Items []db.JobBatchItem `json:"items"`
}

type ChannelVideo = media.ChannelVideo

func parseBatchDelay(value string) (int, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "instant":
		return 0, nil
	case "30s":
		return 30, nil
	case "1m":
		return 60, nil
	case "2m":
		return 120, nil
	case "3m":
		return 180, nil
	case "5m":
		return 300, nil
	case "10m":
		return 600, nil
	case "15m":
		return 900, nil
	case "30m":
		return 1800, nil
	case "1h", "1 hour":
		return 3600, nil
	default:
		return 0, fmt.Errorf("invalid delay: %s", value)
	}
}

func (s *Service) CreateBatch(ctx context.Context, req CreateBatchRequest) (BatchView, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "New Batch"
	}
	if len(req.Items) == 0 {
		return BatchView{}, fmt.Errorf("at least one item is required")
	}
	delaySeconds, err := parseBatchDelay(req.Delay)
	if err != nil {
		return BatchView{}, err
	}

	now := time.Now().UTC()
	batch := db.JobBatch{
		ID:               uuid.NewString(),
		Name:             name,
		DelaySeconds:     delaySeconds,
		Status:           "queued",
		CurrentItemIndex: 0,
		ProcessedItems:   0,
		TotalItems:       len(req.Items),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	items := make([]db.JobBatchItem, 0, len(req.Items))
	for i, in := range req.Items {
		sourceType := strings.ToLower(strings.TrimSpace(in.SourceType))
		sourceURL := strings.TrimSpace(in.SourceURL)
		sourcePath := strings.TrimSpace(in.SourcePath)
		title := strings.TrimSpace(in.Title)
		sectionID := strings.TrimSpace(in.SectionID)
		mainModel := strings.TrimSpace(in.MainModel)
		if sourceType != "url" && sourceType != "path" {
			return BatchView{}, fmt.Errorf("item %d source_type must be url or path", i+1)
		}
		if sourceType == "url" && sourceURL == "" {
			return BatchView{}, fmt.Errorf("item %d missing source_url", i+1)
		}
		if sourceType == "path" && sourcePath == "" {
			return BatchView{}, fmt.Errorf("item %d missing source_path", i+1)
		}
		items = append(items, db.JobBatchItem{
			ID:         uuid.NewString(),
			BatchID:    batch.ID,
			ItemIndex:  i,
			SourceType: sourceType,
			SourceURL:  sourceURL,
			SourcePath: sourcePath,
			Title:      title,
			SectionID:  sectionID,
			MainModel:  mainModel,
			Status:     "queued",
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	if err := s.Store.CreateJobBatch(ctx, batch, items); err != nil {
		return BatchView{}, err
	}

	if req.AutoStart {
		s.startBatchProcessor(batch.ID)
	}
	return BatchView{Batch: batch, Items: items}, nil
}

func (s *Service) ListBatches(ctx context.Context) ([]db.JobBatch, error) {
	return s.Store.ListJobBatches(ctx)
}

func (s *Service) GetBatch(ctx context.Context, batchID string) (BatchView, error) {
	batch, err := s.Store.GetJobBatch(ctx, batchID)
	if err != nil {
		return BatchView{}, err
	}
	items, err := s.Store.ListBatchItems(ctx, batchID)
	if err != nil {
		return BatchView{}, err
	}
	return BatchView{Batch: batch, Items: items}, nil
}

func (s *Service) StartBatch(ctx context.Context, batchID string) error {
	batch, err := s.Store.GetJobBatch(ctx, batchID)
	if err != nil {
		return err
	}

	s.mu.Lock()
	_, isRunning := s.runningBatches[batchID]
	s.mu.Unlock()

	if isRunning {
		return nil
	}

	if batch.Status == "complete" {
		return fmt.Errorf("batch already complete")
	}
	now := time.Now().UTC()
	batch.Status = "queued"
	batch.StartedAt = now.Format(time.RFC3339Nano)
	batch.CompletedAt = ""
	batch.UpdatedAt = now
	if err := s.Store.UpdateJobBatchState(ctx, batch); err != nil {
		return err
	}
	s.startBatchProcessor(batch.ID)
	return nil
}

func (s *Service) ListChannelVideos(ctx context.Context, channelURL string, limit int) ([]ChannelVideo, error) {
	return media.ListChannelVideos(ctx, s.Runner.YTDLPBin, channelURL, limit)
}

func (s *Service) startBatchProcessor(batchID string) {
	s.mu.Lock()
	if _, ok := s.runningBatches[batchID]; ok {
		s.mu.Unlock()
		return
	}
	s.runningBatches[batchID] = struct{}{}
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.runningBatches, batchID)
			s.mu.Unlock()
		}()
		s.runBatch(context.Background(), batchID)
	}()
}

func (s *Service) runBatch(ctx context.Context, batchID string) {
	batch, err := s.Store.GetJobBatch(ctx, batchID)
	if err != nil {
		return
	}
	items, err := s.Store.ListBatchItems(ctx, batchID)
	if err != nil {
		return
	}

	for i := batch.CurrentItemIndex; i < len(items); i++ {
		item := items[i]
		if strings.TrimSpace(item.JobID) != "" {
			job, err := s.Store.GetJob(ctx, item.JobID)
			if err == nil {
				if job.Status == "failed" {
					now := time.Now().UTC()
					item.Status = "failed"
					item.ErrorMessage = job.ErrorMessage
					item.UpdatedAt = now
					_ = s.Store.UpdateBatchItemState(ctx, item)
					batch.Status = "failed"
					batch.LastError = fmt.Sprintf("item %d failed: %s", i+1, job.ErrorMessage)
					batch.CurrentItemIndex = i
					batch.CurrentJobID = item.JobID
					batch.UpdatedAt = now
					batch.CompletedAt = now.Format(time.RFC3339Nano)
					_ = s.Store.UpdateJobBatchState(ctx, batch)
					return
				}
				if job.Status == "complete" {
					continue
				}
			}
		}

		now := time.Now().UTC()
		batch.Status = "running"
		batch.CurrentItemIndex = i
		batch.CurrentJobID = ""
		batch.NextRunAt = ""
		batch.UpdatedAt = now
		if batch.StartedAt == "" {
			batch.StartedAt = now.Format(time.RFC3339Nano)
		}
		_ = s.Store.UpdateJobBatchState(ctx, batch)

		item.Status = "creating_job"
		item.UpdatedAt = now
		_ = s.Store.UpdateBatchItemState(ctx, item)

		job, err := s.createJob(ctx, CreateJobRequest{
			SourceType: item.SourceType,
			SourceURL:  item.SourceURL,
			SourcePath: item.SourcePath,
			Title:      item.Title,
			MainModel:  item.MainModel,
		}, true)
		if err != nil {
			now := time.Now().UTC()
			item.Status = "failed"
			item.ErrorMessage = err.Error()
			item.UpdatedAt = now
			_ = s.Store.UpdateBatchItemState(ctx, item)

			batch.Status = "failed"
			batch.LastError = fmt.Sprintf("item %d create failed: %v", i+1, err)
			batch.CurrentItemIndex = i
			batch.UpdatedAt = now
			batch.CompletedAt = now.Format(time.RFC3339Nano)
			_ = s.Store.UpdateJobBatchState(ctx, batch)
			return
		}

		item.JobID = job.ID
		item.Status = "running"
		item.ErrorMessage = ""
		item.UpdatedAt = time.Now().UTC()
		_ = s.Store.UpdateBatchItemState(ctx, item)

		batch.CurrentJobID = job.ID
		batch.UpdatedAt = time.Now().UTC()
		_ = s.Store.UpdateJobBatchState(ctx, batch)

		for {
			time.Sleep(2 * time.Second)
			current, err := s.Store.GetJob(ctx, job.ID)
			if err != nil {
				continue
			}
			if current.Status == "running" || current.Status == "queued" {
				continue
			}
			now = time.Now().UTC()
			if current.Status == "failed" {
				item.Status = "failed"
				item.ErrorMessage = current.ErrorMessage
				item.UpdatedAt = now
				_ = s.Store.UpdateBatchItemState(ctx, item)

				batch.Status = "failed"
				batch.LastError = fmt.Sprintf("item %d job failed: %s", i+1, current.ErrorMessage)
				batch.CurrentItemIndex = i
				batch.CurrentJobID = current.ID
				batch.UpdatedAt = now
				batch.CompletedAt = now.Format(time.RFC3339Nano)
				_ = s.Store.UpdateJobBatchState(ctx, batch)
				return
			}

			item.Status = "complete"
			item.ErrorMessage = ""
			item.UpdatedAt = now
			_ = s.Store.UpdateBatchItemState(ctx, item)

			if strings.TrimSpace(item.SectionID) != "" {
				if err := s.setSectionWithRetry(ctx, job.ID, item.SectionID); err != nil {
					item.Status = "failed"
					item.ErrorMessage = "section assignment failed: " + err.Error()
					item.UpdatedAt = time.Now().UTC()
					_ = s.Store.UpdateBatchItemState(ctx, item)

					batch.Status = "failed"
					batch.LastError = fmt.Sprintf("item %d section assignment failed: %v", i+1, err)
					batch.CurrentItemIndex = i
					batch.CurrentJobID = current.ID
					batch.UpdatedAt = item.UpdatedAt
					batch.CompletedAt = item.UpdatedAt.Format(time.RFC3339Nano)
					_ = s.Store.UpdateJobBatchState(ctx, batch)
					return
				}
			}

			batch.ProcessedItems = i + 1
			batch.CurrentItemIndex = i + 1
			batch.CurrentJobID = ""
			batch.LastError = ""
			batch.UpdatedAt = now
			if i == len(items)-1 {
				batch.Status = "complete"
				batch.CompletedAt = now.Format(time.RFC3339Nano)
				batch.NextRunAt = ""
				_ = s.Store.UpdateJobBatchState(ctx, batch)
				return
			}
			if batch.DelaySeconds > 0 {
				next := now.Add(time.Duration(batch.DelaySeconds) * time.Second)
				batch.Status = "waiting"
				batch.NextRunAt = next.Format(time.RFC3339Nano)
				_ = s.Store.UpdateJobBatchState(ctx, batch)
				time.Sleep(time.Duration(batch.DelaySeconds) * time.Second)
			} else {
				batch.Status = "queued"
				batch.NextRunAt = ""
				_ = s.Store.UpdateJobBatchState(ctx, batch)
			}
			break
		}
	}
}

func (s *Service) setSectionWithRetry(ctx context.Context, jobID, sectionID string) error {
	for attempt := 0; attempt < 20; attempt++ {
		if err := s.ensureCatalogForReadyJobs(ctx); err == nil {
			if blog, err := s.Store.GetBlogCatalogByJobID(ctx, jobID); err == nil {
				return s.Store.SetBlogCatalogSection(ctx, blog.ID, sectionID)
			}
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("catalog entry not available for job %s", jobID)
}
