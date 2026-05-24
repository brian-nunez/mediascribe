package embeddings

import (
	"context"
	"fmt"
	"math"
	"strings"
)

const DefaultMaxEstimatedTokens = 384

type TextChunk struct {
	Text   string
	Weight float64
}

type weightedEmbedding struct {
	Vector []float32
	Weight float64
}

func BoundedEmbed(ctx context.Context, embedder Embedder, text string) ([]float32, error) {
	chunks := ChunkTextForEmbedding(text, DefaultMaxEstimatedTokens)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no text to embed")
	}
	if len(chunks) == 1 {
		return embedder.Embed(ctx, chunks[0].Text)
	}

	vectors := make([]weightedEmbedding, 0, len(chunks))
	for i, chunk := range chunks {
		vector, err := embedder.Embed(ctx, chunk.Text)
		if err != nil {
			return nil, fmt.Errorf("chunk %d/%d: %w", i+1, len(chunks), err)
		}
		vectors = append(vectors, weightedEmbedding{
			Vector: vector,
			Weight: chunk.Weight,
		})
	}
	return averageEmbeddings(vectors)
}

func ChunkTextForEmbedding(text string, maxTokens int) []TextChunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxTokens <= 0 {
		maxTokens = DefaultMaxEstimatedTokens
	}
	if EstimateTokenCount(text) <= maxTokens {
		return []TextChunk{{Text: text, Weight: float64(EstimateTokenCount(text))}}
	}

	paragraphs := strings.Split(text, "\n\n")
	chunks := make([]TextChunk, 0)
	var current strings.Builder
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if EstimateTokenCount(paragraph) > maxTokens {
			chunks = appendCurrentChunk(chunks, &current)
			chunks = append(chunks, splitLongParagraph(paragraph, maxTokens)...)
			continue
		}

		next := paragraph
		if current.Len() > 0 {
			next = current.String() + "\n\n" + paragraph
		}
		if EstimateTokenCount(next) > maxTokens {
			chunks = appendCurrentChunk(chunks, &current)
			current.WriteString(paragraph)
		} else {
			if current.Len() > 0 {
				current.WriteString("\n\n")
			}
			current.WriteString(paragraph)
		}
	}
	chunks = appendCurrentChunk(chunks, &current)
	return chunks
}

func appendCurrentChunk(chunks []TextChunk, current *strings.Builder) []TextChunk {
	text := strings.TrimSpace(current.String())
	if text != "" {
		chunks = append(chunks, TextChunk{
			Text:   text,
			Weight: float64(EstimateTokenCount(text)),
		})
	}
	current.Reset()
	return chunks
}

func splitLongParagraph(paragraph string, maxTokens int) []TextChunk {
	words := strings.Fields(paragraph)
	if len(words) == 0 {
		return nil
	}

	chunks := make([]TextChunk, 0)
	var current strings.Builder
	for _, word := range words {
		next := word
		if current.Len() > 0 {
			next = current.String() + " " + word
		}
		if current.Len() > 0 && EstimateTokenCount(next) > maxTokens {
			chunks = appendCurrentChunk(chunks, &current)
		}
		if EstimateTokenCount(word) > maxTokens {
			chunks = append(chunks, splitLongWord(word, maxTokens)...)
			continue
		}
		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(word)
	}
	chunks = appendCurrentChunk(chunks, &current)
	return chunks
}

func splitLongWord(word string, maxTokens int) []TextChunk {
	runes := []rune(word)
	maxRunes := maxTokens * 3
	if maxRunes <= 0 {
		maxRunes = DefaultMaxEstimatedTokens * 3
	}
	out := make([]TextChunk, 0, int(math.Ceil(float64(len(runes))/float64(maxRunes))))
	for start := 0; start < len(runes); start += maxRunes {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		text := string(runes[start:end])
		out = append(out, TextChunk{
			Text:   text,
			Weight: float64(EstimateTokenCount(text)),
		})
	}
	return out
}

func EstimateTokenCount(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	byWords := len(strings.Fields(text))
	byChars := int(math.Ceil(float64(len([]rune(text))) / 4.0))
	if byChars > byWords {
		return byChars
	}
	return byWords
}

func averageEmbeddings(vectors []weightedEmbedding) ([]float32, error) {
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no embedding vectors")
	}
	dims := len(vectors[0].Vector)
	if dims == 0 {
		return nil, fmt.Errorf("empty embedding vector")
	}
	avg := make([]float64, dims)
	totalWeight := 0.0
	for i, item := range vectors {
		if len(item.Vector) != dims {
			return nil, fmt.Errorf("embedding dimension mismatch at chunk %d: got %d, want %d", i+1, len(item.Vector), dims)
		}
		weight := item.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
		for j, value := range item.Vector {
			avg[j] += float64(value) * weight
		}
	}
	if totalWeight <= 0 {
		return nil, fmt.Errorf("invalid embedding weights")
	}
	out := make([]float32, dims)
	for i, value := range avg {
		out[i] = float32(value / totalWeight)
	}
	return out, nil
}
