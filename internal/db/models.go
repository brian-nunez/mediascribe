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
