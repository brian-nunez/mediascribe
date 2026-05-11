package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr string

	SQLitePath   string
	ArtifactRoot string

	DefaultMainModel             string
	DefaultMainModelBaseURL      string
	DefaultMainModelTimeout      time.Duration
	DefaultMainModelMaxRetries   int
	DefaultEmbeddingModel        string
	DefaultEmbeddingModelBaseURL string
	DefaultEmbeddingModelTimeout time.Duration
	DefaultEmbeddingMaxRetries   int
	DefaultTranslateModel        string
	DefaultTranslateModelBaseURL string
	DefaultTranslateModelTimeout time.Duration
	DefaultTranslateMaxRetries   int
	ModelRetryBackoff            time.Duration

	FFmpegBin              string
	WhisperCPPBin          string
	WhisperModelPath       string
	YTDLPBin               string
	TranscriptFallbackPath string
	EnableTranslation      bool
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:                     envOrDefault("HTTP_ADDR", ":8080"),
		SQLitePath:                   envOrDefault("SQLITE_PATH", "./data/app.db"),
		ArtifactRoot:                 envOrDefault("ARTIFACT_ROOT", "./artifacts/jobs"),
		DefaultMainModel:             envOrDefault("MAIN_MODEL", "gpt-oss"),
		DefaultMainModelBaseURL:      envOrDefault("MAIN_MODEL_BASE_URL", "http://localhost:11434"),
		DefaultEmbeddingModel:        envOrDefault("EMBEDDING_MODEL", "embeddinggemma"),
		DefaultEmbeddingModelBaseURL: envOrDefault("EMBEDDING_MODEL_BASE_URL", "http://localhost:11435"),
		DefaultTranslateModel:        envOrDefault("TRANSLATE_MODEL", "translategemma"),
		DefaultTranslateModelBaseURL: envOrDefault("TRANSLATE_MODEL_BASE_URL", "http://localhost:11436"),
		FFmpegBin:                    envOrDefault("FFMPEG_BIN", "ffmpeg"),
		WhisperCPPBin:                envOrDefault("WHISPER_CPP_BIN", "./deps/whisper.cpp/build/bin/whisper-cli"),
		WhisperModelPath:             envOrDefault("WHISPER_MODEL_PATH", "./deps/whisper.cpp/models/ggml-base.bin"),
		YTDLPBin:                     envOrDefault("YTDLP_BIN", "yt-dlp"),
		TranscriptFallbackPath:       os.Getenv("TRANSCRIPT_FALLBACK_PATH"),
	}

	mainTimeout, err := durationFromEnv("MAIN_MODEL_TIMEOUT", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	embedTimeout, err := durationFromEnv("EMBEDDING_MODEL_TIMEOUT", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	translateTimeout, err := durationFromEnv("TRANSLATE_MODEL_TIMEOUT", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	mainRetries, err := intFromEnv("MAIN_MODEL_MAX_RETRIES", 2)
	if err != nil {
		return Config{}, err
	}
	embedRetries, err := intFromEnv("EMBEDDING_MODEL_MAX_RETRIES", 2)
	if err != nil {
		return Config{}, err
	}
	translateRetries, err := intFromEnv("TRANSLATE_MODEL_MAX_RETRIES", 2)
	if err != nil {
		return Config{}, err
	}
	retryBackoff, err := durationFromEnv("MODEL_RETRY_BACKOFF", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	enableTranslation, err := boolFromEnv("ENABLE_TRANSLATION", false)
	if err != nil {
		return Config{}, err
	}

	cfg.DefaultMainModelTimeout = mainTimeout
	cfg.DefaultMainModelMaxRetries = mainRetries
	cfg.DefaultEmbeddingModelTimeout = embedTimeout
	cfg.DefaultEmbeddingMaxRetries = embedRetries
	cfg.DefaultTranslateModelTimeout = translateTimeout
	cfg.DefaultTranslateMaxRetries = translateRetries
	cfg.ModelRetryBackoff = retryBackoff
	cfg.EnableTranslation = enableTranslation

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err == nil {
		return d, nil
	}
	seconds, serr := strconv.Atoi(v)
	if serr != nil {
		return 0, fmt.Errorf("invalid duration for %s: %w", key, err)
	}
	return time.Duration(seconds) * time.Second, nil
}

func boolFromEnv(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid bool for %s: %w", key, err)
	}
	return parsed, nil
}

func intFromEnv(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid int for %s: %w", key, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("invalid int for %s: must be >= 0", key)
	}
	return parsed, nil
}
