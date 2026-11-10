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

package recursive

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestRecursiveSplitter(t *testing.T) {
	type args struct {
		ctx    context.Context
		config *Config
		input  []*schema.Document
	}
	ctx := context.Background()
	input := []*schema.Document{
		{Content: "1a23a45a67890c1a234b5678a90"},
	}
	tests := []struct {
		name       string
		args       args
		wantOutput []*schema.Document
		wantErr    bool
	}{
		{
			name: "none",
			args: args{
				ctx: ctx,
				config: &Config{
					ChunkSize:   5,
					OverlapSize: 2,
					Separators:  []string{"a", "b", "c"},
				},
				input: input,
			},
			wantOutput: []*schema.Document{
				{Content: "1a23"},
				{Content: "23a45"},
				{Content: "67890"},
				{Content: "1"},
				{Content: "234"},
				{Content: "5678"},
				{Content: "90"},
			},
		},
		{
			name: "start",
			args: args{
				ctx: ctx,
				config: &Config{
					ChunkSize:   5,
					OverlapSize: 2,
					Separators:  []string{"a", "b", "c"},
					KeepType:    KeepTypeStart,
					IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
						return fmt.Sprintf("%s_part%d", originalID, splitIndex)
					},
				},
				input: input,
			},
			wantOutput: []*schema.Document{
				{ID: "_part0", Content: "1a23"},
				{ID: "_part1", Content: "a45"},
				{ID: "_part2", Content: "a67890"},
				{ID: "_part3", Content: "c1"},
				{ID: "_part4", Content: "a234"},
				{ID: "_part5", Content: "b5678"},
				{ID: "_part6", Content: "a90"},
			},
		},
		{
			name: "end",
			args: args{
				ctx: ctx,
				config: &Config{
					ChunkSize:   5,
					OverlapSize: 2,
					Separators:  []string{"a", "b", "c"},
					KeepType:    KeepTypeEnd,
				},
				input: input,
			},
			wantOutput: []*schema.Document{
				{Content: "1a23a"},
				{Content: "45a"},
				{Content: "67890c"},
				{Content: "1a"},
				{Content: "234b"},
				{Content: "5678a"},
				{Content: "90"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSplitter(tt.args.ctx, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Fatal(err)
			}
			gotOutput, err := s.Transform(tt.args.ctx, tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transform error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotOutput, tt.wantOutput) {
				t.Errorf("splitText() gotOutput = %v, want %v", gotOutput, tt.wantOutput)
			}
		})
	}
}

func TestNewSplitterRejectsUnknownKeepType(t *testing.T) {
	ctx := context.Background()

	valid := []KeepType{KeepTypeNone, KeepTypeStart, KeepTypeEnd}
	for _, kt := range valid {
		s, err := NewSplitter(ctx, &Config{ChunkSize: 10, KeepType: kt})
		if err != nil {
			t.Fatalf("NewSplitter with valid KeepType %d: unexpected error: %v", kt, err)
		}
		if s == nil {
			t.Fatalf("NewSplitter with valid KeepType %d: nil splitter", kt)
		}
	}

	for _, kt := range []KeepType{KeepType(3), KeepType(99), KeepType(255)} {
		s, err := NewSplitter(ctx, &Config{ChunkSize: 10, KeepType: kt})
		if err == nil {
			t.Errorf("NewSplitter with KeepType %d: expected an error, got none", kt)
		}
		if s != nil {
			t.Errorf("NewSplitter with KeepType %d: expected a nil splitter alongside the error", kt)
		}
	}
}

// TestTransformNeverPanics walks every KeepType value the underlying uint8 can
// hold. Values outside the defined set must be refused by the constructor rather
// than reaching Transform, which used to panic on them.
func TestTransformNeverPanics(t *testing.T) {
	ctx := context.Background()
	doc := []*schema.Document{{Content: "The quick brown fox jumps over the lazy dog. Pack my box."}}

	for i := 0; i <= 255; i++ {
		kt := KeepType(i)
		s, err := NewSplitter(ctx, &Config{ChunkSize: 20, KeepType: kt})
		if err != nil {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Transform panicked with KeepType %d: %v", i, r)
				}
			}()
			if _, err := s.Transform(ctx, doc); err != nil {
				t.Fatalf("Transform with KeepType %d: unexpected error: %v", i, err)
			}
		}()
	}
}

// corpora that none of the non-empty default separators ("\n", ".", "?", "!")
// appear in, which is what forces the splitter down to character level. The
// Chinese one matters most: East Asian punctuation is absent from the default
// set, so Chinese text used to be almost unsplittable.
var chunkSizeCorpora = map[string]string{
	"ascii-no-punctuation": strings.Repeat("abcdefghij", 30),
	"chinese": "Eino 是一个强调简洁性、可扩展性与可靠性的大模型应用开发框架，它提供了一系列精心整理的组件抽象与实现，" +
		"并且内置了强大的编排能力，能够帮助开发者在人工智能应用开发的各个阶段实现标准化并提升效率，更多信息请查阅官方用户手册。",
	"english-long-line": "The quick brown fox jumps over the lazy dog while pack my box with five dozen liquor " +
		"jugs and how vexingly quick daft zebras jump in the five boxing wizards tournament",
}

// TestChunkSizeIsRespected asserts the invariant that gives ChunkSize its
// meaning: no emitted chunk may measure longer than the configured limit under
// the configured LenFunc.
func TestChunkSizeIsRespected(t *testing.T) {
	ctx := context.Background()

	for name, text := range chunkSizeCorpora {
		for _, chunkSize := range []int{4, 16, 60, 100} {
			for _, overlap := range []int{0, 3, 10} {
				if overlap >= chunkSize {
					continue
				}
				for _, kt := range []KeepType{KeepTypeNone, KeepTypeStart, KeepTypeEnd} {
					t.Run(fmt.Sprintf("%s/cs=%d/ov=%d/kt=%d", name, chunkSize, overlap, kt), func(t *testing.T) {
						s, err := NewSplitter(ctx, &Config{
							ChunkSize:   chunkSize,
							OverlapSize: overlap,
							KeepType:    kt,
						})
						if err != nil {
							t.Fatalf("NewSplitter: %v", err)
						}

						docs, err := s.Transform(ctx, []*schema.Document{{Content: text}})
						if err != nil {
							t.Fatalf("Transform: %v", err)
						}
						if len(docs) == 0 {
							t.Fatal("Transform returned no chunks for a non-empty document")
						}

						for i, d := range docs {
							if got := len(d.Content); got > chunkSize {
								t.Errorf("chunk %d measures %d, over the ChunkSize limit %d: %q",
									i, got, chunkSize, truncateForMessage(d.Content))
							}
						}
					})
				}
			}
		}
	}
}

// TestCharacterLevelFallbackKeepsEveryCharacter covers the other half of the
// fallback: degrading to character level must not silently drop content. With
// no overlap the concatenation has to reproduce the input exactly.
func TestCharacterLevelFallbackKeepsEveryCharacter(t *testing.T) {
	ctx := context.Background()
	text := strings.Repeat("abcdefghij", 30)

	s, err := NewSplitter(ctx, &Config{ChunkSize: 40, OverlapSize: 0, KeepType: KeepTypeNone})
	if err != nil {
		t.Fatalf("NewSplitter: %v", err)
	}

	docs, err := s.Transform(ctx, []*schema.Document{{Content: text}})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	var got strings.Builder
	for i, d := range docs {
		if i > 0 && len(d.Content) == 0 {
			t.Errorf("chunk %d is empty", i)
		}
		got.WriteString(d.Content)
	}

	if got.String() != text {
		t.Errorf("chunks do not reproduce the input.\n got: %d bytes\nwant: %d bytes",
			got.Len(), len(text))
	}
}

func truncateForMessage(s string) string {
	const max = 40
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max]) + "..."
}
