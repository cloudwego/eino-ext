package cached

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

type Embedder struct {
	embedder embedding.Embedder
	cacher   Cacher
	expire   time.Duration
}

type Option interface {
	apply(*Embedder)
}

type optionFunc func(*Embedder)

func (f optionFunc) apply(e *Embedder) {
	f(e)
}

func WithCacher(cacher Cacher) Option {
	return optionFunc(func(e *Embedder) {
		e.cacher = cacher
	})
}

func WithExpire(expire time.Duration) Option {
	return optionFunc(func(e *Embedder) {
		e.expire = expire
	})
}

var _ embedding.Embedder = (*Embedder)(nil)

func NewEmbedder(embedder embedding.Embedder, opts ...Option) *Embedder {
	e := &Embedder{
		embedder: embedder,
		cacher:   &noCacher{},
		expire:   time.Hour * 2,
	}
	for _, opt := range opts {
		opt.apply(e)
	}
	return e
}

func (e *Embedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	var (
		embeddingsByKey = make(map[int][]float64)
		uncached        []int
		uncachedTexts   []string
	)

	// Get cached embeddings and find uncached texts
	for idx, text := range texts {
		key := generateKey(text, opts...)
		emb, err := e.cacher.Get(ctx, key)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return nil, err
			}
			uncached = append(uncached, idx)
			uncachedTexts = append(uncachedTexts, text)
		} else {
			embeddingsByKey[idx] = emb
		}
	}

	// Embed the uncached texts
	if len(uncachedTexts) > 0 {
		uncachedEmbeddings, err := e.embedder.EmbedStrings(ctx, uncachedTexts, opts...)
		if err != nil {
			return nil, err
		}

		// Cache the uncachedEmbeddings
		for i, idx := range uncached {
			key := generateKey(texts[idx], opts...)
			if err := e.cacher.Set(ctx, key, uncachedEmbeddings[i], e.expire); err != nil {
				_ = err
				// skip caching if there's an error
			}
			embeddingsByKey[idx] = uncachedEmbeddings[i]
		}
	}

	// Convert the map to a slice
	result := make([][]float64, len(texts))
	for i := range texts {
		if emb, ok := embeddingsByKey[i]; ok {
			result[i] = emb
		} else {
			result[i] = nil // it seems that such a case should not happen
		}
	}

	return result, nil
}

var hash = sha256.New()

func generateKey(text string, opts ...embedding.Option) string {
	options := embedding.GetCommonOptions(nil, opts...)
	model := ""
	if options.Model != nil {
		model = *options.Model
	}

	plainText := fmt.Sprintf("%s-%x", text, model)
	return fmt.Sprintf("%x", hash.Sum([]byte(plainText)))
}
