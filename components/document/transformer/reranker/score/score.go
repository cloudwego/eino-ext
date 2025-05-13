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

package score

import (
	"context"
	"sort"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

type Config struct {
	// ScoreFieldKey specifies the key in metadata that stores the document score. Use Score() method to get score by default.
	ScoreFieldKey *string
}

// NewReranker creates a score-based document reranker optimized for LLM context processing.
//
// The reranker reorganizes documents based on their scores in a specific pattern:
// - Documents with higher scores are placed at both the beginning and end of the array
// - Documents with lower scores are placed in the middle
//
// This arrangement is based on research showing that LLMs exhibit better performance
// when relevant information appears at the beginning or end of the input context,
// known as the "primacy and recency effect" (https://arxiv.org/abs/2307.03172).
//
// The score can be obtained either from:
// - Document's Score() method (default)
// - A custom metadata field specified by ScoreFieldKey in the config
func NewReranker(ctx context.Context, config *Config) (document.Transformer, error) {
	var getter func(doc *schema.Document) float64
	if config.ScoreFieldKey == nil {
		getter = func(doc *schema.Document) float64 {
			return doc.Score()
		}
	} else {
		key := *config.ScoreFieldKey
		getter = func(doc *schema.Document) float64 {
			if doc.MetaData == nil {
				return 0
			}
			v, ok := doc.MetaData[key]
			if !ok {
				return 0
			}
			vv, okk := v.(float64)
			if !okk {
				return 0
			}
			return vv
		}
	}
	return &reranker{scoreGetter: getter}, nil
}

type reranker struct {
	scoreGetter func(doc *schema.Document) float64
}

func (r *reranker) Transform(ctx context.Context, src []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	copied := make([]*schema.Document, len(src))
	copy(copied, src)
	sortDocs := sortedDocuments{
		docs:        copied,
		scoreGetter: r.scoreGetter,
	}
	sort.Sort(sortDocs)

	ret := make([]*schema.Document, len(src))
	for i, d := range copied {
		if i%2 == 0 {
			ret[i/2] = d
		} else {
			ret[len(ret)-1-i/2] = d
		}
	}
	return ret, nil
}

func (r *reranker) GetType() string {
	return "ScoreReranker"
}

type sortedDocuments struct {
	docs        []*schema.Document
	scoreGetter func(doc *schema.Document) float64
}

func (s sortedDocuments) Len() int {
	return len(s.docs)
}
func (s sortedDocuments) Less(i, j int) bool {
	return s.scoreGetter(s.docs[i]) > s.scoreGetter(s.docs[j])
}
func (s sortedDocuments) Swap(i, j int) {
	s.docs[i], s.docs[j] = s.docs[j], s.docs[i]
}
