package transcription

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type Transcript struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
}

type Chunk struct {
	Index      int     `json:"index"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Content    string  `json:"content"`
	TokenCount int     `json:"token_count"`
}

type Service struct {
	WhisperCPPBin          string
	WhisperModelPath       string
	TranscriptFallbackPath string
}

func (s Service) Transcribe(ctx context.Context, audioPath, outJSONPath string) (Transcript, error) {
	if s.TranscriptFallbackPath != "" {
		if txt, err := os.ReadFile(s.TranscriptFallbackPath); err == nil {
			transcript := buildTranscriptFromText(string(txt))
			if err := writeTranscript(outJSONPath, transcript); err != nil {
				return Transcript{}, err
			}
			return transcript, nil
		}
	}

	if s.WhisperCPPBin == "" {
		return Transcript{}, fmt.Errorf("WHISPER_CPP_BIN is empty and no fallback transcript provided")
	}
	if s.WhisperModelPath == "" {
		return Transcript{}, fmt.Errorf("WHISPER_MODEL_PATH is empty and no fallback transcript provided")
	}

	prefix := strings.TrimSuffix(outJSONPath, filepath.Ext(outJSONPath))
	cmd := exec.CommandContext(ctx, s.WhisperCPPBin,
		"-m", s.WhisperModelPath,
		"-f", audioPath,
		"-otxt",
		"-of", prefix,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Transcript{}, fmt.Errorf("whisper-cli failed: %w: %s", err, string(out))
	}

	rawTextPath := prefix + ".txt"
	text, err := os.ReadFile(rawTextPath)
	if err != nil {
		return Transcript{}, fmt.Errorf("read whisper output: %w", err)
	}

	transcript := buildTranscriptFromText(string(text))
	if err := writeTranscript(outJSONPath, transcript); err != nil {
		return Transcript{}, err
	}
	return transcript, nil
}

func LoadTranscript(path string) (Transcript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Transcript{}, err
	}
	var t Transcript
	if err := json.Unmarshal(data, &t); err != nil {
		return Transcript{}, err
	}
	return t, nil
}

func ChunkTranscript(transcript Transcript, maxChars int) []Chunk {
	if maxChars <= 0 {
		maxChars = 1200
	}

	segments := transcript.Segments
	if len(segments) == 0 {
		text := strings.TrimSpace(transcript.Text)
		if text == "" {
			return nil
		}
		segments = []Segment{{Start: 0, End: 0, Text: text}}
	}

	chunks := []Chunk{}
	buffer := strings.Builder{}
	chunkStart := segments[0].Start
	chunkEnd := segments[0].End

	flush := func() {
		content := strings.TrimSpace(buffer.String())
		if content == "" {
			buffer.Reset()
			return
		}
		chunks = append(chunks, Chunk{
			Index:      len(chunks),
			Start:      chunkStart,
			End:        chunkEnd,
			Content:    content,
			TokenCount: len(strings.Fields(content)),
		})
		buffer.Reset()
	}

	for i, seg := range segments {
		part := strings.TrimSpace(seg.Text)
		if part == "" {
			continue
		}
		if buffer.Len() == 0 {
			chunkStart = seg.Start
		}

		candidate := part
		if buffer.Len() > 0 {
			candidate = "\n" + part
		}
		if buffer.Len()+len(candidate) > maxChars {
			flush()
			chunkStart = seg.Start
		}
		if buffer.Len() > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(part)
		chunkEnd = seg.End

		if i == len(segments)-1 {
			flush()
		}
	}

	if buffer.Len() > 0 {
		flush()
	}

	return chunks
}

func buildTranscriptFromText(text string) Transcript {
	clean := strings.TrimSpace(text)
	return Transcript{
		Text: clean,
		Segments: []Segment{
			{Start: 0, End: 0, Text: clean},
		},
	}
}

func writeTranscript(path string, t Transcript) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
