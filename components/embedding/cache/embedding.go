/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cache

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

var (
	ErrCacherRequired    = errors.New("embedding/cache: cacher is required")
	ErrGeneratorRequired = errors.New("embedding/cache: generator is required")
)

type Embedder struct {
	embedder   embedding.Embedder
	cacher     Cacher
	generator  Generator
	expiration time.Duration
}

type Option interface {
	apply(*Embedder)
}

type optionFunc func(*Embedder)

func (f optionFunc) apply(e *Embedder) {
	f(e)
}

// WithCacher returns an [Option] that sets the [Cacher] for the [Embedder].
func WithCacher(cacher Cacher) Option {
	return optionFunc(func(e *Embedder) {
		e.cacher = cacher
	})
}

// WithGenerator returns an [Option] that sets the [Generator] for the [Embedder].
func WithGenerator(generator Generator) Option {
	return optionFunc(func(e *Embedder) {
		e.generator = generator
	})
}

// WithExpiration returns an [Option] that sets the expiration duration for cached embeddings in the [Embedder].
func WithExpiration(expiration time.Duration) Option {
	return optionFunc(func(e *Embedder) {
		e.expiration = expiration
	})
}

var _ embedding.Embedder = (*Embedder)(nil)

// NewEmbedder creates a new [Embedder] instance with cache support.
func NewEmbedder(embedder embedding.Embedder, opts ...Option) (*Embedder, error) {
	e := &Embedder{
		embedder:   embedder,
		expiration: time.Hour * 2,
	}
	for _, opt := range opts {
		opt.apply(e)
	}

	if e.cacher == nil {
		return nil, ErrCacherRequired
	}

	if e.generator == nil {
		return nil, ErrGeneratorRequired
	}

	return e, nil
}

func (e *Embedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	var (
		embeddingsByKey = make(map[int][]float64)
		embeddingOpts   = embedding.GetCommonOptions(nil, opts...)
		uncached        []int
		uncachedTexts   []string
	)

	// generate options for the generator
	var generatorOpt GeneratorOption
	if embeddingOpts.Model != nil {
		generatorOpt.Model = *embeddingOpts.Model
	}

	// Get cached embeddings and find uncached texts
	for idx, text := range texts {
		key := e.generator.Generate(ctx, text, generatorOpt)
		emb, ok, err := e.cacher.Get(ctx, key)
		if err != nil {
			return nil, err
		} else if ok {
			embeddingsByKey[idx] = emb
		} else {
			// If the key is not found, we consider it as uncached
			uncached = append(uncached, idx)
			uncachedTexts = append(uncachedTexts, text)
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
			key := e.generator.Generate(ctx, texts[idx], generatorOpt)
			if err := e.cacher.Set(ctx, key, uncachedEmbeddings[i], e.expiration); err != nil {
				_ = err // skip caching if there's an error
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
