package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/brian-nunez/video-to-blog-page/internal/artifacts"
	"github.com/brian-nunez/video-to-blog-page/internal/db"
	"github.com/brian-nunez/video-to-blog-page/internal/embeddings"
	"github.com/brian-nunez/video-to-blog-page/internal/markdown"
	"github.com/brian-nunez/video-to-blog-page/internal/media"
	"github.com/brian-nunez/video-to-blog-page/internal/ollama"
	"github.com/brian-nunez/video-to-blog-page/internal/transcription"
	"github.com/brian-nunez/video-to-blog-page/internal/translation"
)

const (
	StageCreateJob           = "create_job"
	StageDownloadOrCopyVideo = "download_or_copy_video"
	StageExtractAudio        = "extract_audio"
	StageTranscribeAudio     = "transcribe_audio"
	StageChunkTranscript     = "chunk_transcript"
	StageGenerateEmbeddings  = "generate_embeddings"
	StageAnalyzeChunks       = "analyze_chunks"
	StageCreateOutline       = "create_outline"
	StageWriteDraft          = "write_draft"
	StageRefineMarkdown      = "refine_markdown"
	StageOptionalTranslate   = "optional_translate_markdown"
	StageMarkComplete        = "mark_complete"
)

type StageSpec struct {
	Name            string
	RequiredInputs  []string
	ExpectedOutputs []string
}

var OrderedStages = []StageSpec{
	{Name: StageDownloadOrCopyVideo, RequiredInputs: []string{"source_url or source_path"}, ExpectedOutputs: []string{"source.mp4"}},
	{Name: StageExtractAudio, RequiredInputs: []string{"source.mp4"}, ExpectedOutputs: []string{"audio.wav"}},
	{Name: StageTranscribeAudio, RequiredInputs: []string{"audio.wav"}, ExpectedOutputs: []string{"transcript.json"}},
	{Name: StageChunkTranscript, RequiredInputs: []string{"transcript.json"}, ExpectedOutputs: []string{"chunks.json"}},
	{Name: StageGenerateEmbeddings, RequiredInputs: []string{"chunks.json"}, ExpectedOutputs: []string{"chunk_embeddings"}},
	{Name: StageAnalyzeChunks, RequiredInputs: []string{"chunks.json"}, ExpectedOutputs: []string{"analysis.json"}},
	{Name: StageCreateOutline, RequiredInputs: []string{"analysis.json", "transcript.json"}, ExpectedOutputs: []string{"outline.md"}},
	{Name: StageWriteDraft, RequiredInputs: []string{"outline.md", "transcript.json"}, ExpectedOutputs: []string{"draft.md"}},
	{Name: StageRefineMarkdown, RequiredInputs: []string{"draft.md"}, ExpectedOutputs: []string{"final.md"}},
	{Name: StageOptionalTranslate, RequiredInputs: []string{"final.md"}, ExpectedOutputs: []string{"final.<lang>.md (optional)"}},
	{Name: StageMarkComplete, RequiredInputs: []string{"all pipeline artifacts"}, ExpectedOutputs: []string{"job status complete"}},
}

type Runner struct {
	Store *db.Store

	FFmpegBin              string
	WhisperCPPBin          string
	WhisperModelPath       string
	YTDLPBin               string
	TranscriptFallbackPath string

	MainModelTimeout      time.Duration
	MainModelMaxRetries   int
	EmbeddingModelTimeout time.Duration
	EmbeddingMaxRetries   int
	TranslateModelTimeout time.Duration
	TranslateMaxRetries   int
	ModelRetryBackoff     time.Duration
}

func (r Runner) Run(ctx context.Context, jobID string) error {
	job, err := r.Store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	if err := r.Store.SetJobRunningStage(ctx, job.ID, StageCreateJob); err != nil {
		return err
	}

	mainClient := ollama.NewClientWithRetry(job.MainModelBaseURL, r.MainModelTimeout, r.MainModelMaxRetries, r.ModelRetryBackoff)
	embeddingClient := ollama.NewClientWithRetry(job.EmbeddingModelBaseURL, r.EmbeddingModelTimeout, r.EmbeddingMaxRetries, r.ModelRetryBackoff)
	translateClient := ollama.NewClientWithRetry(job.TranslationModelBaseURL, r.TranslateModelTimeout, r.TranslateMaxRetries, r.ModelRetryBackoff)

	mdWriter := markdown.Writer{Client: mainClient, Model: job.MainModel}
	embedder := embeddings.OllamaEmbedder{Client: embeddingClient, Model: job.EmbeddingModel}
	translator := translation.OllamaTranslator{Client: translateClient, Model: job.TranslationModel}
	transcriber := transcription.Service{
		WhisperCPPBin:          r.WhisperCPPBin,
		WhisperModelPath:       r.WhisperModelPath,
		TranscriptFallbackPath: r.TranscriptFallbackPath,
	}

	for _, stage := range OrderedStages {
		if err := r.Store.SetJobRunningStage(ctx, job.ID, stage.Name); err != nil {
			return err
		}

		var stageErr error
		switch stage.Name {
		case StageDownloadOrCopyVideo:
			stageErr = runDownloadOrCopy(ctx, job, r.YTDLPBin)
		case StageExtractAudio:
			stageErr = runExtractAudio(ctx, job, r.FFmpegBin)
		case StageTranscribeAudio:
			stageErr = runTranscription(ctx, job, transcriber)
		case StageChunkTranscript:
			stageErr = runChunkTranscript(ctx, r.Store, job)
		case StageGenerateEmbeddings:
			stageErr = runGenerateEmbeddings(ctx, r.Store, job, embedder)
		case StageAnalyzeChunks:
			stageErr = runAnalyzeChunks(ctx, r.Store, job, mainClient)
		case StageCreateOutline:
			stageErr = runCreateOutline(ctx, r.Store, job, mdWriter)
		case StageWriteDraft:
			stageErr = runWriteDraft(ctx, job, mdWriter)
		case StageRefineMarkdown:
			stageErr = runRefineMarkdown(ctx, job, mdWriter)
		case StageOptionalTranslate:
			stageErr = runOptionalTranslate(ctx, job, translator)
		case StageMarkComplete:
			stageErr = runMarkComplete(ctx, r.Store, job, embedder)
		default:
			stageErr = fmt.Errorf("unknown stage: %s", stage.Name)
		}

		if stageErr != nil {
			return r.failJob(ctx, job.ID, stage.Name, stageErr)
		}
	}

	return nil
}

func runDownloadOrCopy(ctx context.Context, job db.Job, ytdlpBin string) error {
	videoPath := filepath.Join(job.ArtifactDir, "source.mp4")
	return media.DownloadOrCopyVideo(ctx, job.SourceType, job.SourceURL, job.SourcePath, videoPath, ytdlpBin)
}

func runExtractAudio(ctx context.Context, job db.Job, ffmpegBin string) error {
	videoPath := filepath.Join(job.ArtifactDir, "source.mp4")
	audioPath := filepath.Join(job.ArtifactDir, "audio.wav")
	return media.ExtractAudio(ctx, ffmpegBin, videoPath, audioPath)
}

func runTranscription(ctx context.Context, job db.Job, transcriber transcription.Service) error {
	audioPath := filepath.Join(job.ArtifactDir, "audio.wav")
	transcriptPath := filepath.Join(job.ArtifactDir, "transcript.json")
	_, err := transcriber.Transcribe(ctx, audioPath, transcriptPath)
	return err
}

func runChunkTranscript(ctx context.Context, store *db.Store, job db.Job) error {
	transcriptPath := filepath.Join(job.ArtifactDir, "transcript.json")
	t, err := transcription.LoadTranscript(transcriptPath)
	if err != nil {
		return err
	}

	chunks := transcription.ChunkTranscript(t, 1200)
	if err := artifacts.WriteJSON(filepath.Join(job.ArtifactDir, "chunks.json"), chunks); err != nil {
		return err
	}

	dbChunks := make([]db.TranscriptChunk, 0, len(chunks))
	now := time.Now().UTC()
	for _, chunk := range chunks {
		dbChunks = append(dbChunks, db.TranscriptChunk{
			ID:               uuid.NewString(),
			JobID:            job.ID,
			ChunkIndex:       chunk.Index,
			StartTimeSeconds: chunk.Start,
			EndTimeSeconds:   chunk.End,
			Content:          chunk.Content,
			TokenCount:       chunk.TokenCount,
			CreatedAt:        now,
		})
	}

	return store.ReplaceTranscriptChunks(ctx, job.ID, dbChunks)
}

func runGenerateEmbeddings(ctx context.Context, store *db.Store, job db.Job, embedder embeddings.Embedder) error {
	chunks, err := store.ListTranscriptChunks(ctx, job.ID)
	if err != nil {
		return err
	}

	records := make([]db.ChunkEmbedding, 0, len(chunks))
	now := time.Now().UTC()
	for _, chunk := range chunks {
		vector, err := embedder.Embed(ctx, chunk.Content)
		if err != nil {
			return fmt.Errorf("embed chunk %d: %w", chunk.ChunkIndex, err)
		}
		records = append(records, db.ChunkEmbedding{
			ID:                    uuid.NewString(),
			JobID:                 job.ID,
			ChunkID:               chunk.ID,
			Embedding:             embeddings.Float32ToBytes(vector),
			EmbeddingDimensions:   len(vector),
			EmbeddingModel:        job.EmbeddingModel,
			EmbeddingModelBaseURL: job.EmbeddingModelBaseURL,
			CreatedAt:             now,
		})
	}
	return store.ReplaceChunkEmbeddings(ctx, job.ID, records)
}

func runAnalyzeChunks(ctx context.Context, store *db.Store, job db.Job, mainClient *ollama.Client) error {
	chunks, err := store.ListTranscriptChunks(ctx, job.ID)
	if err != nil {
		return err
	}

	type analysisItem struct {
		ChunkIndex int    `json:"chunk_index"`
		ChunkID    string `json:"chunk_id"`
		Analysis   string `json:"analysis"`
	}

	analysisPayload := make([]analysisItem, 0, len(chunks))
	records := make([]db.ChunkAnalysis, 0, len(chunks))
	now := time.Now().UTC()

	for _, chunk := range chunks {
		prompt := fmt.Sprintf(`Analyze this transcript chunk for technical content.

Return JSON with keys:
- key_points (array)
- implementation_details (array)
- caveats (array)
- notable_terms (array)

Chunk:\n%s`, chunk.Content)

		result, err := mainClient.Generate(ctx, job.MainModel, prompt)
		if err != nil {
			return fmt.Errorf("analyze chunk %d: %w", chunk.ChunkIndex, err)
		}

		records = append(records, db.ChunkAnalysis{
			ID:           uuid.NewString(),
			JobID:        job.ID,
			ChunkID:      chunk.ID,
			AnalysisJSON: result,
			CreatedAt:    now,
		})
		analysisPayload = append(analysisPayload, analysisItem{
			ChunkIndex: chunk.ChunkIndex,
			ChunkID:    chunk.ID,
			Analysis:   result,
		})
	}

	if err := store.ReplaceChunkAnalyses(ctx, job.ID, records); err != nil {
		return err
	}
	return artifacts.WriteJSON(filepath.Join(job.ArtifactDir, "analysis.json"), analysisPayload)
}

func runCreateOutline(ctx context.Context, store *db.Store, job db.Job, writer markdown.Writer) error {
	transcriptRaw, err := os.ReadFile(filepath.Join(job.ArtifactDir, "transcript.json"))
	if err != nil {
		return err
	}
	analysisRaw, err := os.ReadFile(filepath.Join(job.ArtifactDir, "analysis.json"))
	if err != nil {
		return err
	}

	outline, err := writer.CreateOutline(ctx, string(transcriptRaw), string(analysisRaw))
	if err != nil {
		return err
	}
	outlinePath := filepath.Join(job.ArtifactDir, "outline.md")
	if err := artifacts.WriteString(outlinePath, outline); err != nil {
		return err
	}

	blog, err := defaultBlogOutput(store, ctx, job.ID)
	if err != nil {
		return err
	}
	blog.OutlinePath = outlinePath
	blog.UpdatedAt = time.Now().UTC()
	blog.Status = "outline_ready"
	return store.UpsertBlogOutput(ctx, blog)
}

func runWriteDraft(ctx context.Context, job db.Job, writer markdown.Writer) error {
	outlinePath := filepath.Join(job.ArtifactDir, "outline.md")
	transcriptPath := filepath.Join(job.ArtifactDir, "transcript.json")

	outline, err := os.ReadFile(outlinePath)
	if err != nil {
		return err
	}
	transcriptRaw, err := os.ReadFile(transcriptPath)
	if err != nil {
		return err
	}

	draft, err := writer.CreateDraft(ctx, string(outline), string(transcriptRaw))
	if err != nil {
		return err
	}
	return artifacts.WriteString(filepath.Join(job.ArtifactDir, "draft.md"), draft)
}

func runRefineMarkdown(ctx context.Context, job db.Job, writer markdown.Writer) error {
	draftPath := filepath.Join(job.ArtifactDir, "draft.md")
	raw, err := os.ReadFile(draftPath)
	if err != nil {
		return err
	}
	final, err := writer.RefineMarkdown(ctx, string(raw))
	if err != nil {
		return err
	}
	return artifacts.WriteString(filepath.Join(job.ArtifactDir, "final.md"), final)
}

func runOptionalTranslate(ctx context.Context, job db.Job, translator translation.Translator) error {
	if !job.TranslationEnabled {
		return nil
	}
	if strings.TrimSpace(job.TranslationLanguage) == "" {
		return fmt.Errorf("translation enabled but translation_language is empty")
	}

	finalPath := filepath.Join(job.ArtifactDir, "final.md")
	raw, err := os.ReadFile(finalPath)
	if err != nil {
		return err
	}

	translated, err := translator.TranslateMarkdown(ctx, string(raw), job.TranslationLanguage)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("final.%s.md", sanitizeLang(job.TranslationLanguage))
	return artifacts.WriteString(filepath.Join(job.ArtifactDir, filename), translated)
}

func runMarkComplete(ctx context.Context, store *db.Store, job db.Job, embedder embeddings.Embedder) error {
	finalPath := filepath.Join(job.ArtifactDir, "final.md")
	if _, err := os.Stat(finalPath); err != nil {
		return fmt.Errorf("missing final markdown: %w", err)
	}
	if err := GenerateBlogOutputEmbeddings(ctx, store, job, embedder); err != nil {
		return err
	}

	blog, err := defaultBlogOutput(store, ctx, job.ID)
	if err != nil {
		return err
	}
	blog.DraftPath = filepath.Join(job.ArtifactDir, "draft.md")
	blog.FinalMarkdownPath = finalPath
	blog.Status = "complete"
	blog.UpdatedAt = time.Now().UTC()
	if job.TranslationEnabled {
		blog.TranslationLanguage = job.TranslationLanguage
		blog.TranslatedMarkdownPath = filepath.Join(job.ArtifactDir, fmt.Sprintf("final.%s.md", sanitizeLang(job.TranslationLanguage)))
	}
	if err := store.UpsertBlogOutput(ctx, blog); err != nil {
		return err
	}
	// Cleanup large source video artifact after successful completion to save disk.
	_ = cleanupSourceVideo(job.ArtifactDir)

	return store.MarkJobComplete(ctx, job.ID)
}

func cleanupSourceVideo(artifactDir string) error {
	videoPath := filepath.Join(artifactDir, "source.mp4")
	if err := os.Remove(videoPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (r Runner) failJob(ctx context.Context, jobID, stage string, stageErr error) error {
	msg := stageErr.Error()
	if len(msg) > 4000 {
		msg = msg[:4000] + "...(truncated)"
	}
	_ = r.Store.MarkJobFailed(ctx, jobID, stage, msg)
	return stageErr
}

func sanitizeLang(value string) string {
	clean := strings.ToLower(strings.TrimSpace(value))
	if clean == "" {
		return "translated"
	}
	re := regexp.MustCompile(`[^a-z0-9_-]+`)
	clean = re.ReplaceAllString(clean, "")
	if clean == "" {
		return "translated"
	}
	return clean
}

func defaultBlogOutput(store *db.Store, ctx context.Context, jobID string) (db.BlogOutput, error) {
	now := time.Now().UTC()
	blog, err := store.GetBlogOutputByJob(ctx, jobID)
	if err == nil {
		return blog, nil
	}
	if err != db.ErrNotFound {
		return db.BlogOutput{}, err
	}

	job, err := store.GetJob(ctx, jobID)
	if err != nil {
		return db.BlogOutput{}, err
	}
	title := job.Title
	if strings.TrimSpace(title) == "" {
		title = "Technical Video Blog"
	}
	return db.BlogOutput{
		ID:        uuid.NewString(),
		JobID:     jobID,
		Title:     title,
		Slug:      slugify(title),
		Status:    "processing",
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func slugify(value string) string {
	s := strings.ToLower(strings.TrimSpace(value))
	s = strings.ReplaceAll(s, "_", "-")
	re := regexp.MustCompile(`[^a-z0-9\- ]+`)
	s = re.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, " ", "-")
	reDash := regexp.MustCompile(`-+`)
	s = reDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "technical-video-blog"
	}
	return s
}

func BuildTranscriptPreview(chunks []db.TranscriptChunk) string {
	parts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		parts = append(parts, c.Content)
	}
	return strings.Join(parts, "\n\n")
}

func BuildAnalysisString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var pretty any
	if err := json.Unmarshal(data, &pretty); err != nil {
		return string(data), nil
	}
	formatted, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return string(data), nil
	}
	return string(formatted), nil
}
