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

package semantic

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

type Config struct {
	// Embedding is used to generate vectors for calculating difference between chunks.
	Embedding embedding.Embedder
	// BufferSize specifies how many chunks to concatenate before and after each chunk during embedding, which allows chunks to carry more information, thereby improving the accuracy of differences between chunks.
	BufferSize int
	// MinChunkSize specifies the minimum chunk's size. Chunks with size smaller than MinChunkSize will be concatenated to their adjacent chunks.
	MinChunkSize int
	// Separators are sequentially used to split text. ["\n", ".", "?", "!"] by default.
	Separators []string
	// LenFunc is used to calculate string length. Use builtin function len() by default.
	LenFunc func(s string) int
	// Percentile specifies the number of splitting. If the difference between two chunks is greater than X percentile, these two chunks will be split.
	Percentile float64
}

func NewSplitter(ctx context.Context, config *Config) (document.Transformer, error) {
	if config.Embedding == nil {
		return nil, fmt.Errorf("embedding should not be nil")
	}
	lenFunc := config.LenFunc
	if lenFunc == nil {
		lenFunc = func(s string) int { return len(s) }
	}
	seps := config.Separators
	if len(seps) == 0 {
		seps = []string{"\n", ".", "?", "!"}
	}
	percentile := config.Percentile
	if percentile == 0 {
		percentile = 0.9
	}
	return &splitter{
		embedding:    config.Embedding,
		bufferSize:   config.BufferSize,
		minChunkSize: config.MinChunkSize,
		separators:   seps,
		lenFunc:      lenFunc,
		percentile:   percentile,
	}, nil
}

type splitter struct {
	embedding    embedding.Embedder
	bufferSize   int
	minChunkSize int
	separators   []string
	lenFunc      func(s string) int
	percentile   float64
}

func (s *splitter) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	ret := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		splits, err := s.splitText(ctx, doc.Content, s.separators)
		if err != nil {
			return nil, fmt.Errorf("split document[%s] fail: %w", doc.ID, err)
		}
		for _, split := range splits {
			ret = append(ret, &schema.Document{
				ID:       doc.ID,
				Content:  split,
				MetaData: deepCopyMap(doc.MetaData),
			})
		}
	}
	return ret, nil
}

func (s *splitter) splitText(ctx context.Context, text string, separators []string) ([]string, error) {
	texts := []string{text}
	// split
	for i := range s.separators {
		texts = splitTexts(texts, separators[i])
	}

	sentencesLength := make([]int, len(texts))
	for i := range texts {
		sentencesLength[i] = s.lenFunc(texts[i])
	}

	if len(texts) == 1 {
		return texts, nil
	}

	// combine
	combinedSentences := make([]string, len(texts))
	for i := range texts {
		combinedSentence := texts[i]
		for j := 1; j <= s.bufferSize && i+j < len(texts); j++ {
			combinedSentence = combinedSentence + texts[i+j]
		}
		for j := 1; j <= s.bufferSize && i-j >= 0; j++ {
			combinedSentence = texts[i-j] + combinedSentence
		}
		combinedSentences[i] = combinedSentence
	}

	// embedding
	var vectors [][]float64
	v, err := s.embedding.EmbedStrings(ctx, combinedSentences)
	if err != nil {
		return nil, err
	}
	for i := range v {
		vectors = append(vectors, v[i])
	}

	// cosine distances
	distances := make([]float64, len(texts))
	for i := 1; i < len(texts); i++ {
		distances[i] = 1 - cosine(vectors[i-1], vectors[i])
	}

	threshold := calThreshold(distances, s.percentile)
	var splitIndexes []int
	for i := 1; i < len(distances); i++ {
		if distances[i] <= threshold {
			splitIndexes = append(splitIndexes, i)
		}
	}
	var ret []string
	var startIndex int
	for i := range splitIndexes {
		chunk := strings.Join(texts[startIndex:splitIndexes[i]], "")
		if len(chunk) < s.minChunkSize {
			continue
		}
		ret = append(ret, chunk)
		startIndex = splitIndexes[i]
	}
	ret = append(ret, strings.Join(texts[startIndex:], ""))
	return ret, nil
}

func (s *splitter) GetType() string {
	return "SemanticSplitter"
}

func cosine(vec1, vec2 []float64) float64 {
	dotProduct := dot(vec1, vec2)
	normVec1 := math.Sqrt(dot(vec1, vec1))
	normVec2 := math.Sqrt(dot(vec2, vec2))
	return dotProduct / (normVec1 * normVec2)
}

func dot(x, y []float64) float64 {
	var sum float64
	for i, v := range x {
		sum += y[i] * v
	}
	return sum
}

func splitTexts(texts []string, sep string) []string {
	var ret []string
	for i := range texts {
		ret = append(ret, strings.SplitAfter(texts[i], sep)...)
	}
	return ret
}

func calThreshold(distances []float64, percentile float64) float64 {
	sorted := make([]float64, len(distances))
	copy(sorted, distances)
	sort.Float64s(sorted)
	idx := int((1 - percentile) * float64(len(sorted)))
	if idx == 0 {
		idx = 1
	}
	return sorted[idx]
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	ret := make(map[string]interface{}, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}
