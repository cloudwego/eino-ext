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
	for _, tt := range []struct {
		name      string
		generator Generator
	}{
		{"SimpleGenerator", NewSimpleGenerator()},
		{"HashGenerator", NewHashGenerator(sha256.New())},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("Generate uniqueness", func(t *testing.T) {
				for _, tt := range []struct {
					callback func() string
				}{
					{func() string { return tt.generator.Generate("foo") }},
					{func() string { return tt.generator.Generate("foo", embedding.WithModel("bar")) }},
				} {
					assert.Equal(t, tt.callback(), tt.callback())
				}
			})

			t.Run("Generate different keys", func(t *testing.T) {
				assert.NotEqual(t, tt.generator.Generate("foo"), tt.generator.Generate("bar"))
				assert.NotEqual(t, tt.generator.Generate("foo"), tt.generator.Generate("foo", embedding.WithModel("bar")))
				assert.NotEqual(t, tt.generator.Generate("foo", embedding.WithModel("bar")),
					tt.generator.Generate("foo", embedding.WithModel("baz")))
			})
		})
	}
}

func TestGenerator_SimpleGenerator(t *testing.T) {
	text := "test text"
	model := "test-model"

	generator := NewSimpleGenerator()
	assert.Equal(t, generator.Generate(text, embedding.WithModel(model)), text+"-"+model)
	assert.Equal(t, generator.Generate(text), text+"-")
	assert.Equal(t, generator.Generate(""), "-")
	assert.Equal(t, generator.Generate(""), generator.Generate(""))
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
