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
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"math/rand/v2"
	"reflect"
	"testing"
)

type randomEmbedding struct {
	vecLen int
}

func (r *randomEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	var ret [][]float64
	for range texts {
		piece := make([]float64, r.vecLen)
		for j := range piece {
			piece[j] = rand.Float64()
		}
		ret = append(ret, piece)
	}
	return ret, nil
}

func TestSemanticSplitter(t *testing.T) {
	type args struct {
		ctx  context.Context
		docs []*schema.Document
		opts []document.TransformerOption
	}
	tests := []struct {
		name      string
		config    *Config
		input     []*schema.Document
		outputLen int
	}{
		{
			name: "success",
			config: &Config{
				Embedding:    &randomEmbedding{vecLen: 5},
				BufferSize:   1,
				MinChunkSize: 9,
				Separators:   []string{"."},
				LenFunc:      nil,
				Percentile:   0.5,
				IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
					return fmt.Sprintf("%s_part%d", originalID, splitIndex)
				},
			},
			input: []*schema.Document{{
				Content: "1234567890.1234567890.1234567890.1234567890.1234567890.1234567890",
			}},
			outputLen: 4,
		},
		{
			name: "corner case: text has not exceeded MinChunkSize",
			config: &Config{
				Embedding:    &randomEmbedding{vecLen: 5},
				BufferSize:   1,
				MinChunkSize: 9,
				Separators:   []string{"."},
				LenFunc:      nil,
				Percentile:   0.5,
			},
			input: []*schema.Document{{
				Content: "1.1.1.1",
			}},
			outputLen: 1,
		},
		{
			name: "corner case: percentile is too big",
			config: &Config{
				Embedding:    &randomEmbedding{vecLen: 5},
				BufferSize:   1,
				MinChunkSize: 9,
				Separators:   []string{"."},
				LenFunc:      nil,
				Percentile:   0.9999,
			},
			input: []*schema.Document{{
				Content: "1234567890.1234567890.1234567890.1234567890.1234567890.1234567890",
			}},
			outputLen: 2,
		},
		{
			name: "corner case: percentile is too small",
			config: &Config{
				Embedding:    &randomEmbedding{vecLen: 5},
				BufferSize:   1,
				MinChunkSize: 9,
				Separators:   []string{"."},
				LenFunc:      nil,
				Percentile:   0.00001,
			},
			input: []*schema.Document{{
				Content: "1234567890.1234567890.1234567890.1234567890.1234567890.1234567890",
			}},
			outputLen: 6,
		},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSplitter(ctx, tt.config)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 10; i++ {
				got, err := s.Transform(ctx, tt.input)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(len(got), tt.outputLen) {
					t.Errorf("Transform() got = %v, want %v", got, tt.outputLen)
				}
			}
		})
	}
}
