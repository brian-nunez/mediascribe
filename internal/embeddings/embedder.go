package embeddings

import (
	"context"
	"encoding/binary"
	"math"

	"github.com/brian-nunez/video-to-blog-page/internal/ollama"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type OllamaEmbedder struct {
	Client *ollama.Client
	Model  string
}

func (o OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return o.Client.Embed(ctx, o.Model, text)
}

func Float32ToBytes(values []float32) []byte {
	buf := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(buf[i*4:(i+1)*4], math.Float32bits(v))
	}
	return buf
}
