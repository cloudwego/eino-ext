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
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerator_UniquenessAndDifference(t *testing.T) {
	ctx := context.Background()
	opt := GeneratorOption{}

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
					{func() string { return tt.generator.Generate(ctx, "foo", opt) }},
					{func() string { return tt.generator.Generate(ctx, "foo", GeneratorOption{Model: "bar"}) }},
				} {
					assert.Equal(t, tt.callback(), tt.callback())
				}
			})

			t.Run("Generate different keys", func(t *testing.T) {
				assert.NotEqual(t, tt.generator.Generate(ctx, "foo", opt), tt.generator.Generate(ctx, "bar", opt))
				assert.NotEqual(t, tt.generator.Generate(ctx, "foo", opt), tt.generator.Generate(ctx, "foo", GeneratorOption{Model: "bar"}))
				assert.NotEqual(t, tt.generator.Generate(ctx, "foo", GeneratorOption{Model: "bar"}),
					tt.generator.Generate(ctx, "foo", GeneratorOption{Model: "baz"}))
			})
		})
	}
}

func TestGenerator_SimpleGenerator(t *testing.T) {
	text := "test text"
	model := "test-model"
	ctx := context.Background()
	opt := GeneratorOption{}

	generator := NewSimpleGenerator()
	assert.Equal(t, generator.Generate(ctx, text, GeneratorOption{Model: model}), text+"-"+model)
	assert.Equal(t, generator.Generate(ctx, text, opt), text+"-")
	assert.Equal(t, generator.Generate(ctx, "", opt), "-")
}

func TestGenerator_HashGenerator(t *testing.T) {
	text := "test text"
	model := "test-model"
	ctx := context.Background()

	for _, tt := range []struct {
		name string
		hash hash.Hash
	}{
		{"sha256", sha256.New()},
		{"md5", md5.New()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewHashGenerator(tt.hash)
			assert.NotEmpty(t, generator.Generate(ctx, text, GeneratorOption{Model: model}))
		})
	}
}

// TestGenerator_HashGeneratorProducesDigest pins the actual hashing contract.
// Generate used to call hasher.Sum(plaintext), which does not hash anything: Sum appends
// the current digest to its argument, so the "hash" key was the hex encoded plaintext
// followed by the digest of an empty input. Keys therefore grew with the input and the
// cached text was recoverable from them.
func TestGenerator_HashGeneratorProducesDigest(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name    string
		text    string
		model   string
		digest  func(string) string
		wantLen int
	}{
		{
			name:    "sha256",
			text:    "test text",
			model:   "test-model",
			wantLen: 64,
			digest: func(plain string) string {
				sum := sha256.Sum256([]byte(plain))
				return hex.EncodeToString(sum[:])
			},
		},
		{
			name:    "md5",
			text:    "test text",
			model:   "test-model",
			wantLen: 32,
			digest: func(plain string) string {
				sum := md5.Sum([]byte(plain))
				return hex.EncodeToString(sum[:])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			generator := NewHashGenerator(sha256.New())
			if tc.name == "md5" {
				generator = NewHashGenerator(md5.New())
			}

			plainText := tc.text + "-" + tc.model
			key := generator.Generate(ctx, tc.text, GeneratorOption{Model: tc.model})

			assert.Equal(t, tc.digest(plainText), key, "key must be the digest of the wrapped generator output")
			assert.Len(t, key, tc.wantLen, "hash keys must have the digest's fixed length")

			// The cached text must not be recoverable from the key: a key that embeds the
			// plaintext leaks prompt and document content into the cache key space.
			assert.NotContains(t, key, hex.EncodeToString([]byte(tc.text)))
			assert.NotContains(t, key, tc.text)
		})
	}
}

// TestGenerator_HashGeneratorKeyLengthIsIndependentOfInput makes the "no plaintext in the
// key" property explicit: the old implementation produced 2*(len(plaintext)+digestSize)
// hex characters, so a large document produced a correspondingly large cache key.
func TestGenerator_HashGeneratorKeyLengthIsIndependentOfInput(t *testing.T) {
	ctx := context.Background()
	generator := NewHashGenerator(sha256.New())

	shortKey := generator.Generate(ctx, "a", GeneratorOption{})
	longKey := generator.Generate(ctx, strings.Repeat("a", 4096), GeneratorOption{})

	assert.Len(t, shortKey, 64)
	assert.Len(t, longKey, 64, "key length must not grow with the input")
	assert.NotEqual(t, shortKey, longKey)
}

// TestGenerator_HashGeneratorRepeatedCalls covers the reused hasher: without a Reset the
// second call would digest the concatenation of both inputs.
func TestGenerator_HashGeneratorRepeatedCalls(t *testing.T) {
	ctx := context.Background()
	generator := NewHashGenerator(sha256.New())

	first := generator.Generate(ctx, "first", GeneratorOption{})
	second := generator.Generate(ctx, "second", GeneratorOption{})
	firstAgain := generator.Generate(ctx, "first", GeneratorOption{})

	assert.NotEqual(t, first, second)
	assert.Equal(t, first, firstAgain, "hashing the same input again must produce the same key")

	sum := sha256.Sum256([]byte("first-"))
	assert.Equal(t, hex.EncodeToString(sum[:]), first)
}

// TestGenerator_HashGeneratorConcurrent guards the shared hash.Hash. Generate is reachable
// from concurrent EmbedStrings calls, and after the fix it mutates the hasher state, so an
// unsynchronised implementation interleaves Write and Sum and returns wrong keys.
func TestGenerator_HashGeneratorConcurrent(t *testing.T) {
	ctx := context.Background()
	generator := NewHashGenerator(sha256.New())

	inputs := make([]string, 0, 64)
	want := make(map[string]string, 64)
	for i := 0; i < 64; i++ {
		text := fmt.Sprintf("text-%d", i)
		inputs = append(inputs, text)

		sum := sha256.Sum256([]byte(text + "-"))
		want[text] = hex.EncodeToString(sum[:])
	}

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan string, workers*len(inputs))

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, text := range inputs {
				if got := generator.Generate(ctx, text, GeneratorOption{}); got != want[text] {
					errs <- fmt.Sprintf("text %q: got %s, want %s", text, got, want[text])

					return
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}
