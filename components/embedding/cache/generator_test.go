package cache

import (
	"crypto/md5"
	"crypto/sha256"
	"hash"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/stretchr/testify/assert"
)

func TestGenerator_UniquenessAndDifference(t *testing.T) {
	t.Run("Generate uniqueness", func(t *testing.T) {
		for _, tt := range []struct {
			callback func() string
		}{
			{func() string { return defaultGenerator.Generate("foo") }},
			{func() string { return defaultGenerator.Generate("foo", embedding.WithModel("bar")) }},
		} {
			assert.Equal(t, tt.callback(), tt.callback())
		}
	})

	t.Run("Generate different keys", func(t *testing.T) {
		assert.NotEqual(t, defaultGenerator.Generate("foo"), defaultGenerator.Generate("bar"))
		assert.NotEqual(t, defaultGenerator.Generate("foo"), defaultGenerator.Generate("foo", embedding.WithModel("bar")))
		assert.NotEqual(t, defaultGenerator.Generate("foo", embedding.WithModel("bar")),
			defaultGenerator.Generate("foo", embedding.WithModel("baz")))
	})
}

func TestGenerator_HashGenerator(t *testing.T) {
	text := "test text"
	model := "test-model"

	for _, tt := range []struct {
		name string
		hash hash.Hash
	}{
		{"sha256", sha256.New()},
		{"md5", md5.New()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewHashGenerator(tt.hash)
			assert.NotEmpty(t, generator.Generate(text, embedding.WithModel(model)))
		})
	}
}
