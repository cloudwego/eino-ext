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
	"fmt"
	"hash"

	"github.com/cloudwego/eino/components/embedding"
)

// Generator is an interface for generating unique keys based on text and optional embedding options.
// It is used to create cache keys for embedding results.
type Generator interface {
	Generate(text string, opts ...embedding.Option) string
}

// SimpleGenerator is a concrete implementation of the Generator interface that generates
// a simple key by concatenating the text and model without hashing.
type SimpleGenerator struct{}

var _ Generator = (*SimpleGenerator)(nil)

// NewSimpleGenerator creates a new [SimpleGenerator] instance.
func NewSimpleGenerator() *SimpleGenerator {
	return &SimpleGenerator{}
}

func (g *SimpleGenerator) Generate(text string, opts ...embedding.Option) string {
	options := embedding.GetCommonOptions(nil, opts...)
	model := ""
	if options.Model != nil {
		model = *options.Model
	}

	return fmt.Sprintf("%s-%s", text, model)
}

// HashGenerator is a concrete implementation of the [Generator] interface that uses a hash function
// to generate a unique key based on the provided text and optional embedding options.
// It wraps a [SimpleGenerator] and applies a hash function to the generated key.
//
// Note: Because of the use of the [hash.Hash] algorithm, there is a probability that data
// with different text and options will generate the same key. This is a trade-off
// between uniqueness and performance. If you need guaranteed uniqueness, consider
// using a different generator or a more complex hashing strategy.
type HashGenerator struct {
	*SimpleGenerator
	hasher hash.Hash
}

var _ Generator = (*HashGenerator)(nil)

// NewHashGenerator creates a new [HashGenerator] with the specified hash function.
func NewHashGenerator(hasher hash.Hash) *HashGenerator {
	return &HashGenerator{
		SimpleGenerator: NewSimpleGenerator(),
		hasher:          hasher,
	}
}

func (g *HashGenerator) Generate(text string, opts ...embedding.Option) string {
	plainText := g.SimpleGenerator.Generate(text, opts...)
	return fmt.Sprintf("%x", g.hasher.Sum([]byte(plainText)))
}
