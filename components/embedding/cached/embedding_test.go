package cached

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockEmbedder struct {
	embedding.Embedder
	mock.Mock
}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	args := m.Called(ctx, texts, opts)
	return args.Get(0).([][]float64), args.Error(1)
}

type mockCacher struct {
	Cacher
	mock.Mock
}

func (m *mockCacher) Get(ctx context.Context, key string) ([]float64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]float64), args.Error(1)
}

func (m *mockCacher) Set(ctx context.Context, key string, value []float64, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

func TestEmbedder_EmbedStrings(t *testing.T) {
	ctx := context.Background()
	texts := []string{"foo", "bar"}
	embeddings := [][]float64{{1.1, 2.2}, {3.3, 4.4}}
	expiration := time.Minute

	t.Run("embedder", func(t *testing.T) {
		me := new(mockEmbedder)
		e := NewEmbedder(me)

		me.On("EmbedStrings", mock.Anything, texts, mock.Anything).Return(embeddings, nil)

		result, err := e.EmbedStrings(ctx, texts)
		assert.NoError(t, err)
		assert.Equal(t, embeddings, result)
	})

	t.Run("all cache hit", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e := NewEmbedder(me, WithCacher(mc), WithExpiration(expiration))

		me.On("EmbedStrings", mock.Anything, texts, mock.Anything).Return(embeddings, nil)
		for i, text := range texts {
			key := generateKey(text)
			mc.On("Get", mock.Anything, key).Return(embeddings[i], nil)
		}

		result, err := e.EmbedStrings(ctx, texts)
		assert.NoError(t, err)
		assert.Equal(t, embeddings, result)
		mc.AssertExpectations(t)
	})

	t.Run("partial cache hit", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e := NewEmbedder(me, WithCacher(mc), WithExpiration(expiration))

		key0 := generateKey(texts[0])
		key1 := generateKey(texts[1])
		mc.On("Get", mock.Anything, key0).Return(nil, ErrNotFound)
		mc.On("Get", mock.Anything, key1).Return(embeddings[1], nil)
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
		e := NewEmbedder(me, WithCacher(mc), WithExpiration(expiration))

		key0 := generateKey(texts[0])
		key1 := generateKey(texts[1])
		mc.On("Get", mock.Anything, key0).Return(nil, ErrNotFound)
		mc.On("Get", mock.Anything, key1).Return(nil, ErrNotFound)
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
		e := NewEmbedder(me, WithCacher(mc), WithExpiration(expiration))

		key := generateKey(texts[0])
		mc.On("Get", mock.Anything, key).Return(nil, errors.New("cache error"))

		_, err := e.EmbedStrings(ctx, []string{texts[0]})
		assert.Error(t, err)
		mc.AssertExpectations(t)
	})

	t.Run("underlying embedder error", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e := NewEmbedder(me, WithCacher(mc), WithExpiration(expiration))

		key := generateKey(texts[0])
		mc.On("Get", mock.Anything, key).Return(nil, ErrNotFound)
		me.On("EmbedStrings", mock.Anything, []string{texts[0]}, mock.Anything).Return(nil, errors.New("embed error"))

		_, err := e.EmbedStrings(ctx, []string{texts[0]})
		assert.Error(t, err)
		mc.AssertExpectations(t)
		me.AssertExpectations(t)
	})

	t.Run("cache set error, ignore", func(t *testing.T) {
		mc := new(mockCacher)
		me := new(mockEmbedder)
		e := NewEmbedder(me, WithCacher(mc), WithExpiration(expiration))

		key := generateKey(texts[0])
		mc.On("Get", mock.Anything, key).Return(nil, ErrNotFound)
		me.On("EmbedStrings", mock.Anything, []string{texts[0]}, mock.Anything).Return([][]float64{embeddings[0]}, nil)
		mc.On("Set", mock.Anything, key, embeddings[0], expiration).Return(errors.New("set error"))

		result, err := e.EmbedStrings(ctx, []string{texts[0]})
		assert.NoError(t, err)
		assert.Equal(t, [][]float64{embeddings[0]}, result)
		mc.AssertExpectations(t)
		me.AssertExpectations(t)
	})
}
