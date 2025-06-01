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
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockEmbedder struct {
	embedding.Embedder
	mock.Mock
}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	args := m.Called(ctx, texts, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([][]float64), args.Error(1)
}

type mockCacher struct {
	Cacher
	mock.Mock
}

var _ Cacher = (*mockCacher)(nil)

func (m *mockCacher) Get(ctx context.Context, key string) ([]float64, bool, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).([]float64), args.Bool(1), args.Error(2)
}

func (m *mockCacher) Set(ctx context.Context, key string, value []float64, expire time.Duration) error {
	args := m.Called(ctx, key, value, expire)
	return args.Error(0)
}

func TestEmbedder_EmbedStrings(t *testing.T) {
	ctx := context.Background()
	texts := []string{"foo", "bar"}
	embeddings := [][]float64{{1.1, 2.2}, {3.3, 4.4}}
	expiration := time.Minute
	generatorOpt := GeneratorOption{}

	t.Run("embedder not set cacher", func(t *testing.T) {
		me := new(mockEmbedder)
		e, err := NewEmbedder(me, WithGenerator(NewSimpleGenerator()))
		require.Error(t, err)
		assert.Equal(t, ErrCacherRequired, err)
		assert.Nil(t, e)
		me.AssertExpectations(t)
	})

	t.Run("embedder not set generator", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e, err := NewEmbedder(me, WithCacher(mc))
		require.Error(t, err)
		assert.Equal(t, ErrGeneratorRequired, err)
		assert.Nil(t, e)
		mc.AssertExpectations(t)
		me.AssertExpectations(t)
	})

	t.Run("all cache hit", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e, err := NewEmbedder(me, WithCacher(mc), WithGenerator(NewSimpleGenerator()), WithExpiration(expiration))
		require.NoError(t, err)

		for i, text := range texts {
			key := e.generator.Generate(ctx, text, generatorOpt)
			mc.On("Get", mock.Anything, key).Return(embeddings[i], true, nil)
		}

		result, err := e.EmbedStrings(ctx, texts)
		assert.NoError(t, err)
		assert.Equal(t, embeddings, result)
		mc.AssertExpectations(t)
	})

	t.Run("partial cache hit", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e, err := NewEmbedder(me, WithCacher(mc), WithGenerator(NewSimpleGenerator()), WithExpiration(expiration))
		require.NoError(t, err)

		key0 := e.generator.Generate(ctx, texts[0], generatorOpt)
		key1 := e.generator.Generate(ctx, texts[1], generatorOpt)

		mc.On("Get", mock.Anything, key0).Return(nil, false, nil)
		mc.On("Get", mock.Anything, key1).Return(embeddings[1], true, nil)
		me.On("EmbedStrings", mock.Anything, []string{texts[0]}, mock.Anything).Return([][]float64{embeddings[0]}, nil)
		mc.On("Set", mock.Anything, key0, embeddings[0], expiration).Return(nil)

		result, err := e.EmbedStrings(ctx, texts)
		assert.NoError(t, err)
		assert.Equal(t, embeddings, result)

		mc.AssertExpectations(t)
		me.AssertExpectations(t)
	})

	t.Run("all cache miss", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e, err := NewEmbedder(me, WithCacher(mc), WithGenerator(NewSimpleGenerator()), WithExpiration(expiration))
		require.NoError(t, err)

		key0 := e.generator.Generate(ctx, texts[0], generatorOpt)
		key1 := e.generator.Generate(ctx, texts[1], generatorOpt)

		mc.On("Get", mock.Anything, key0).Return(nil, false, nil)
		mc.On("Get", mock.Anything, key1).Return(nil, false, nil)
		me.On("EmbedStrings", mock.Anything, texts, mock.Anything).Return(embeddings, nil)
		mc.On("Set", mock.Anything, key0, embeddings[0], expiration).Return(nil)
		mc.On("Set", mock.Anything, key1, embeddings[1], expiration).Return(nil)

		result, err := e.EmbedStrings(ctx, texts)
		assert.NoError(t, err)
		assert.Equal(t, embeddings, result)
		mc.AssertExpectations(t)
		me.AssertExpectations(t)
	})

	t.Run("cache get error", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e, err := NewEmbedder(me, WithCacher(mc), WithGenerator(NewSimpleGenerator()), WithExpiration(expiration))
		require.NoError(t, err)

		key := e.generator.Generate(ctx, texts[0], generatorOpt)
		mc.On("Get", mock.Anything, key).Return(nil, false, errors.New("cache error"))

		_, err = e.EmbedStrings(ctx, []string{texts[0]})
		assert.Error(t, err)
		mc.AssertExpectations(t)
		me.AssertExpectations(t)
	})

	t.Run("underlying embedder error", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e, err := NewEmbedder(me, WithCacher(mc), WithGenerator(NewSimpleGenerator()), WithExpiration(expiration))
		require.NoError(t, err)

		key := e.generator.Generate(ctx, texts[0], generatorOpt)
		mc.On("Get", mock.Anything, key).Return(nil, false, nil)
		me.On("EmbedStrings", mock.Anything, []string{texts[0]}, mock.Anything).Return(nil, errors.New("embed error"))

		_, err = e.EmbedStrings(ctx, []string{texts[0]})
		assert.Error(t, err)
		mc.AssertExpectations(t)
		me.AssertExpectations(t)
	})

	t.Run("cache set error, ignore", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e, err := NewEmbedder(me, WithCacher(mc), WithGenerator(NewSimpleGenerator()), WithExpiration(expiration))
		require.NoError(t, err)

		key0 := e.generator.Generate(ctx, texts[0], generatorOpt)

		mc.On("Get", mock.Anything, key0).Return(nil, false, nil)
		me.On("EmbedStrings", mock.Anything, []string{texts[0]}, mock.Anything).Return([][]float64{embeddings[0]}, nil)
		mc.On("Set", mock.Anything, key0, embeddings[0], expiration).Return(errors.New("set error"))

		result, err := e.EmbedStrings(ctx, []string{texts[0]})
		assert.NoError(t, err)
		assert.Equal(t, [][]float64{embeddings[0]}, result)
		mc.AssertExpectations(t)
		me.AssertExpectations(t)
	})
}
