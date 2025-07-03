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
	"reflect"
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
					ChunkSize:        5,
					OverlapSize:      2,
					Separators:       []string{"a", "b", "c"},
					KeepType:         KeepTypeStart,
					GenerateUniqueID: true,
				},
				input: input,
			},
			wantOutput: []*schema.Document{
				{Content: "1a23"},
				{Content: "a45"},
				{Content: "a67890"},
				{Content: "c1"},
				{Content: "a234"},
				{Content: "b5678"},
				{Content: "a90"},
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
