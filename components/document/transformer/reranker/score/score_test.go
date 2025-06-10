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
	"math/rand"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

var scoreKey = "score"

var scoredDocs = []*schema.Document{
	{ID: "0", MetaData: map[string]any{scoreKey: 0.0}},
	{ID: "1", MetaData: map[string]any{scoreKey: 1.0}},
	{ID: "2", MetaData: map[string]any{scoreKey: 2.0}},
	{ID: "3", MetaData: map[string]any{scoreKey: 3.0}},
	{ID: "4", MetaData: map[string]any{scoreKey: 4.0}},
	{ID: "5", MetaData: map[string]any{scoreKey: 5.0}},
	{ID: "6", MetaData: map[string]any{scoreKey: 6.0}},
	{ID: "7", MetaData: map[string]any{scoreKey: 7.0}},
	{ID: "8", MetaData: map[string]any{scoreKey: 8.0}},
	{ID: "9", MetaData: map[string]any{scoreKey: 9.0}},
}

func TestScoreReranker(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		input  []*schema.Document
		wanted []*schema.Document
	}{
		{
			name:   "5cases",
			config: &Config{ScoreFieldKey: &scoreKey},
			input:  []*schema.Document{scoredDocs[0], scoredDocs[1], scoredDocs[2], scoredDocs[3], scoredDocs[4]},
			wanted: []*schema.Document{scoredDocs[4], scoredDocs[2], scoredDocs[0], scoredDocs[1], scoredDocs[3]},
		},
		{
			name:   "10cases",
			config: &Config{ScoreFieldKey: &scoreKey},
			input:  []*schema.Document{scoredDocs[0], scoredDocs[1], scoredDocs[2], scoredDocs[3], scoredDocs[4], scoredDocs[5], scoredDocs[6], scoredDocs[7], scoredDocs[8], scoredDocs[9]},
			wanted: []*schema.Document{scoredDocs[9], scoredDocs[7], scoredDocs[5], scoredDocs[3], scoredDocs[1], scoredDocs[0], scoredDocs[2], scoredDocs[4], scoredDocs[6], scoredDocs[8]},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			randomDocs(tt.input)
			r, err := NewReranker(ctx, tt.config)
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.Transform(ctx, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result, tt.wanted) {
				t.Fatalf("got %v, want %v", result, tt.wanted)
			}
		})
	}
}

func randomDocs(slice []*schema.Document) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}
