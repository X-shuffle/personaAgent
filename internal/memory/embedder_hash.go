package memory

import (
	"context"
	"crypto/sha256"
	"math"
)

// HashEmbedder is a deterministic local embedder for dev/test.
type HashEmbedder struct {
	dim int
}

func NewHashEmbedder(dim int) HashEmbedder {
	if dim <= 0 {
		dim = 256
	}
	return HashEmbedder{dim: dim}
}

func (e HashEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, 0, len(texts))
	for _, text := range texts {
		hash := sha256.Sum256([]byte(text))
		v := make([]float64, e.dim)
		for i, b := range hash {
			v[i%e.dim] += float64(b)
		}
		norm := l2Norm(v)
		if norm > 0 {
			for i := range v {
				v[i] = v[i] / norm
			}
		}
		out = append(out, v)
	}
	return out, nil
}

func l2Norm(v []float64) float64 {
	var sum float64
	for _, n := range v {
		sum += n * n
	}
	return math.Sqrt(sum)
}
