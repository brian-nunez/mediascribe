package db

import "time"

type Job struct {
	ID                      string     `json:"id"`
	SourceType              string     `json:"source_type"`
	SourceURL               string     `json:"source_url,omitempty"`
	SourcePath              string     `json:"source_path,omitempty"`
	Title                   string     `json:"title,omitempty"`
	Status                  string     `json:"status"`
	CurrentStage            string     `json:"current_stage,omitempty"`
	ErrorMessage            string     `json:"error_message,omitempty"`
	ArtifactDir             string     `json:"artifact_dir"`
	MainModel               string     `json:"main_model"`
	MainModelBaseURL        string     `json:"main_model_base_url"`
	EmbeddingModel          string     `json:"embedding_model"`
	EmbeddingModelBaseURL   string     `json:"embedding_model_base_url"`
	TranslationModel        string     `json:"translation_model"`
	TranslationModelBaseURL string     `json:"translation_model_base_url"`
	TranslationEnabled      bool       `json:"translation_enabled"`
	TranslationLanguage     string     `json:"translation_language,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	CompletedAt             *time.Time `json:"completed_at,omitempty"`
}

type TranscriptChunk struct {
	ID               string    `json:"id"`
	JobID            string    `json:"job_id"`
	ChunkIndex       int       `json:"chunk_index"`
	StartTimeSeconds float64   `json:"start_time_seconds"`
	EndTimeSeconds   float64   `json:"end_time_seconds"`
	Content          string    `json:"content"`
	TokenCount       int       `json:"token_count"`
	CreatedAt        time.Time `json:"created_at"`
}

type ChunkEmbedding struct {
	ID                    string    `json:"id"`
	JobID                 string    `json:"job_id"`
	ChunkID               string    `json:"chunk_id"`
	Embedding             []byte    `json:"embedding"`
	EmbeddingDimensions   int       `json:"embedding_dimensions"`
	EmbeddingModel        string    `json:"embedding_model"`
	EmbeddingModelBaseURL string    `json:"embedding_model_base_url"`
	CreatedAt             time.Time `json:"created_at"`
}

type BlogOutputEmbedding struct {
	ID                    string    `json:"id"`
	JobID                 string    `json:"job_id"`
	Language              string    `json:"language"`
	ContentSHA256         string    `json:"content_sha256"`
	Embedding             []byte    `json:"embedding"`
	EmbeddingDimensions   int       `json:"embedding_dimensions"`
	EmbeddingModel        string    `json:"embedding_model"`
	EmbeddingModelBaseURL string    `json:"embedding_model_base_url"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type ChunkAnalysis struct {
	ID           string    `json:"id"`
	JobID        string    `json:"job_id"`
	ChunkID      string    `json:"chunk_id"`
	AnalysisJSON string    `json:"analysis_json"`
	CreatedAt    time.Time `json:"created_at"`
}

type BlogOutput struct {
	ID                     string    `json:"id"`
	JobID                  string    `json:"job_id"`
	Title                  string    `json:"title"`
	Slug                   string    `json:"slug"`
	OutlinePath            string    `json:"outline_path,omitempty"`
	DraftPath              string    `json:"draft_path,omitempty"`
	FinalMarkdownPath      string    `json:"final_markdown_path,omitempty"`
	TranslatedMarkdownPath string    `json:"translated_markdown_path,omitempty"`
	TranslationLanguage    string    `json:"translation_language,omitempty"`
	Status                 string    `json:"status"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type SearchChunkRecord struct {
	JobID            string  `json:"job_id"`
	JobTitle         string  `json:"job_title"`
	JobStatus        string  `json:"job_status"`
	ChunkID          string  `json:"chunk_id"`
	ChunkIndex       int     `json:"chunk_index"`
	StartTimeSeconds float64 `json:"start_time_seconds"`
	EndTimeSeconds   float64 `json:"end_time_seconds"`
	Content          string  `json:"content"`
	Embedding        []byte  `json:"embedding"`
	EmbeddingDims    int     `json:"embedding_dimensions"`
}

type Section struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BlogCatalog struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	SectionID string    `json:"section_id,omitempty"`
	Title     string    `json:"title"`
	Published bool      `json:"published"`
	Deleted   bool      `json:"deleted"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BlogContentOverride struct {
	ID        string    `json:"id"`
	BlogID    string    `json:"blog_id"`
	Language  string    `json:"language"`
	Markdown  string    `json:"markdown"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PublicBlogRow struct {
	BlogID      string
	JobID       string
	Title       string
	SectionID   string
	SectionName string
	SourceURL   string
	SourcePath  string
	Status      string
	ArtifactDir string
	BlogUpdated time.Time
	BlogCreated time.Time
	JobUpdated  time.Time
	JobCreated  time.Time
}

type AdminUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type AdminSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	RevokedAt string    `json:"revoked_at,omitempty"`
}

type JobBatch struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	DelaySeconds     int       `json:"delay_seconds"`
	Status           string    `json:"status"`
	CurrentItemIndex int       `json:"current_item_index"`
	CurrentJobID     string    `json:"current_job_id,omitempty"`
	ProcessedItems   int       `json:"processed_items"`
	TotalItems       int       `json:"total_items"`
	LastError        string    `json:"last_error,omitempty"`
	NextRunAt        string    `json:"next_run_at,omitempty"`
	StartedAt        string    `json:"started_at,omitempty"`
	CompletedAt      string    `json:"completed_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type JobBatchItem struct {
	ID           string    `json:"id"`
	BatchID      string    `json:"batch_id"`
	ItemIndex    int       `json:"item_index"`
	SourceType   string    `json:"source_type"`
	SourceURL    string    `json:"source_url,omitempty"`
	SourcePath   string    `json:"source_path,omitempty"`
	Title        string    `json:"title,omitempty"`
	SectionID    string    `json:"section_id,omitempty"`
	MainModel    string    `json:"main_model,omitempty"`
	JobID        string    `json:"job_id,omitempty"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
