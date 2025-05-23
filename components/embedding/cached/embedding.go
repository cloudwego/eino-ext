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
	embedder   embedding.Embedder
	cacher     Cacher
	expiration time.Duration
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

func WithExpiration(expiration time.Duration) Option {
	return optionFunc(func(e *Embedder) {
		e.expiration = expiration
	})
}

var _ embedding.Embedder = (*Embedder)(nil)

func NewEmbedder(embedder embedding.Embedder, opts ...Option) *Embedder {
	e := &Embedder{
		embedder:   embedder,
		cacher:     &noCacher{},
		expiration: time.Hour * 2,
	}
	for _, opt := range opts {
		opt.apply(e)
	}
	return e
}

func (e *Embedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	// // if texts is of length 1, use simpleEmbedString
	// if len(texts) == 1 {
	// 	return e.simpleEmbedStrings(ctx, texts[0], opts...)
	// }

	// otherwise, use the cached embedder
	var (
		embeddingsWithKey = make(map[int][]float64)
		notCached         []int
		uncachedTexts     []string
	)

	// Get cached embeddings and find uncached texts
	for idx, text := range texts {
		key := generateKey(text, opts...)
		emb, err := e.cacher.Get(ctx, key)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return nil, err
			}
			notCached = append(notCached, idx)
			uncachedTexts = append(uncachedTexts, text)
		} else {
			embeddingsWithKey[idx] = emb
		}
	}

	// Embed the uncached texts
	embeddings, err := e.embedder.EmbedStrings(ctx, uncachedTexts, opts...)
	if err != nil {
		return nil, err
	}

	// Cache the embeddings
	for i, idx := range notCached {
		key := generateKey(texts[idx], opts...)
		if err := e.cacher.Set(ctx, key, embeddings[i], e.expiration); err != nil {
			_ = err
			// skip caching if there's an error
		}
		embeddingsWithKey[idx] = embeddings[i]
	}

	// Convert the map to a slice
	result := make([][]float64, len(texts))
	for i := range texts {
		if emb, ok := embeddingsWithKey[i]; ok {
			result[i] = emb
		} else {
			result[i] = nil // it seems that such a case should not happen
		}
	}

	return result, nil
}

// func (e *Embedder) simpleEmbedStrings(ctx context.Context, text string, opts ...embedding.Option) ([][]float64, error) {
// 	key := generateKey(text, opts...)
// 	emb, err := e.cacher.Get(ctx, key)
// 	if err != nil {
// 		if !errors.Is(err, ErrNotFound) {
// 			return nil, err
// 		}
// 	}
//
// 	if emb != nil {
// 		return [][]float64{emb}, nil
// 	}
//
// 	embs, err := e.embedder.EmbedStrings(ctx, []string{text}, opts...)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(embs) != 1 {
// 		return nil, errors.New("embedding length mismatch")
// 	}
// 	return embs, nil
// }

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
