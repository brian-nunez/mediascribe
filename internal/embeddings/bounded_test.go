package embeddings

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type fakeEmbedder struct {
	calls []string
	vecs  [][]float32
}

func (f *fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	f.calls = append(f.calls, text)
	if len(f.vecs) == 0 {
		return []float32{float32(len(f.calls)), float32(len(text))}, nil
	}
	if len(f.calls) > len(f.vecs) {
		return nil, fmt.Errorf("unexpected embed call")
	}
	return f.vecs[len(f.calls)-1], nil
}

func TestChunkTextForEmbeddingBoundsLargeText(t *testing.T) {
	text := ""
	for i := 0; i < 1200; i++ {
		text += fmt.Sprintf("word%d ", i)
	}

	chunks := ChunkTextForEmbedding(text, 128)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if got := EstimateTokenCount(chunk.Text); got > 128 {
			t.Fatalf("chunk %d estimated tokens = %d, want <= 128", i, got)
		}
		if chunk.Weight <= 0 {
			t.Fatalf("chunk %d weight = %f, want > 0", i, chunk.Weight)
		}
	}
}

func TestBoundedEmbedSplitsLargeText(t *testing.T) {
	embedder := &fakeEmbedder{}

	vector, err := BoundedEmbed(context.Background(), embedder, strings.Repeat("word ", 1200))
	if err != nil {
		t.Fatalf("BoundedEmbed() error = %v", err)
	}
	if len(embedder.calls) < 2 {
		t.Fatalf("embed calls = %d, want multiple calls", len(embedder.calls))
	}
	if len(vector) != 2 {
		t.Fatalf("vector dims = %d, want 2", len(vector))
	}
}

func TestAverageEmbeddingsWeightsChunks(t *testing.T) {
	vector, err := averageEmbeddings([]weightedEmbedding{
		{Vector: []float32{1, 3}, Weight: 2},
		{Vector: []float32{3, 5}, Weight: 2},
	})
	if err != nil {
		t.Fatalf("averageEmbeddings() error = %v", err)
	}
	if vector[0] != 2 || vector[1] != 4 {
		t.Fatalf("vector = %#v, want [2 4]", vector)
	}
}

func TestAverageEmbeddingsRejectsDimensionMismatch(t *testing.T) {
	_, err := averageEmbeddings([]weightedEmbedding{
		{Vector: []float32{1, 2}, Weight: 1},
		{Vector: []float32{3}, Weight: 1},
	})
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}
