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

package markdown

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestMarkdownHeaderSplitter(t *testing.T) {
	tests := []struct {
		name   string
		config *HeaderConfig
		input  []*schema.Document
		want   []*schema.Document
	}{
		{
			name: "success",
			config: &HeaderConfig{
				Headers: map[string]string{
					"#":   "Header1",
					"##":  "Header2",
					"###": "Header3",
				},
				TrimHeaders: true,
				IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
					return fmt.Sprintf("%s_part%d", originalID, splitIndex)
				},
			},
			input: []*schema.Document{{
				ID:       "id",
				Content:  "# Header1\n\n ```code1\ncode2\ncode3\n```\n ## Header2\n\nContent1\n\n ### Header3 \n\n Content2 \n\n ## Header4\n\n Content3",
				MetaData: map[string]interface{}{},
			}},
			want: []*schema.Document{{
				ID:      "id_part0",
				Content: "```code1\ncode2\ncode3\n```",
				MetaData: map[string]interface{}{
					"Header1": "Header1",
				},
			}, {
				ID:      "id_part1",
				Content: "Content1",
				MetaData: map[string]interface{}{
					"Header1": "Header1",
					"Header2": "Header2",
				},
			}, {
				ID:      "id_part2",
				Content: "Content2",
				MetaData: map[string]interface{}{
					"Header1": "Header1",
					"Header2": "Header2",
					"Header3": "Header3",
				},
			}, {
				ID:      "id_part3",
				Content: "Content3",
				MetaData: map[string]interface{}{
					"Header1": "Header1",
					"Header2": "Header4",
				},
			}},
		},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			splitter, err := NewHeaderSplitter(ctx, tt.config)
			if err != nil {
				t.Fatal(err)
			}
			ret, err := splitter.Transform(ctx, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ret, tt.want) {
				t.Errorf("NewHeaderSplitter() got = %v, want %v", ret, tt.want)
			}
		})
	}
}

func TestHeaderSplitterCodeFenceBoundaries(t *testing.T) {
	tests := []struct{ name, open, inside, close string }{
		{"shorter backticks", "````markdown", "```", "````"},
		{"six backticks", "``````markdown", "text", "``````"},
		{"shorter tildes", "~~~~markdown", "~~~", "~~~~"},
		{"backticks with suffix", "```markdown", "```not-a-close", "```"},
		{"tildes with suffix", "~~~markdown", "~~~not-a-close", "~~~"},
		{"different fence type", "```markdown", "~~~", "```"},
		{"longer closing fence", "```markdown", "text", "`````"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, trim := range []bool{false, true} {
				splitter, err := NewHeaderSplitter(context.Background(), &HeaderConfig{Headers: map[string]string{"#": "title"}, TrimHeaders: trim})
				if err != nil {
					t.Fatal(err)
				}
				code := tt.open + "\n" + tt.inside + "\n# inside code\n" + tt.close
				docs, err := splitter.Transform(context.Background(), []*schema.Document{{Content: "# First\n" + code + "\n# Second\nafter"}})
				if err != nil {
					t.Fatal(err)
				}
				if len(docs) != 2 {
					t.Fatalf("trim=%t: got %d chunks, want 2: %+v", trim, len(docs), docs)
				}
				wantFirst, wantSecond := code, "after"
				if !trim {
					wantFirst = "# First\n" + code
					wantSecond = "# Second\nafter"
				}
				if docs[0].Content != wantFirst || docs[0].MetaData["title"] != "First" {
					t.Errorf("trim=%t: first chunk = %+v", trim, docs[0])
				}
				if docs[1].Content != wantSecond || docs[1].MetaData["title"] != "Second" {
					t.Errorf("trim=%t: second chunk = %+v", trim, docs[1])
				}
			}
		})
	}
}
