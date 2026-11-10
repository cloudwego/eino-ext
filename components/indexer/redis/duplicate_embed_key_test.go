/*
 * Copyright 2026 CloudWeGo Authors
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

package redis

import (
	"context"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	redisclient "github.com/redis/go-redis/v9"
)

type recordingPipelineHook struct{ commands []redisclient.Cmder }

func (h *recordingPipelineHook) DialHook(next redisclient.DialHook) redisclient.DialHook {
	return next
}
func (h *recordingPipelineHook) ProcessHook(next redisclient.ProcessHook) redisclient.ProcessHook {
	return next
}
func (h *recordingPipelineHook) ProcessPipelineHook(next redisclient.ProcessPipelineHook) redisclient.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redisclient.Cmder) error {
		h.commands = append(h.commands, cmds...)
		return nil
	}
}

type recordingEmbedder struct{ calls int }

func (e *recordingEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	e.calls++
	vectors := make([][]float64, len(texts))
	for i, text := range texts {
		vectors[i] = []float64{float64(len(text))}
	}
	return vectors, nil
}

func TestStoreDuplicateEmbedKey(t *testing.T) {
	for _, duplicate := range []bool{true, false} {
		name := "distinct"
		if duplicate {
			name = "duplicate"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			client := redisclient.NewClient(&redisclient.Options{Addr: "unused:6379"})
			defer client.Close()
			hook := &recordingPipelineHook{}
			client.AddHook(hook)
			embedder := &recordingEmbedder{}
			secondKey := "body_vector"
			if duplicate {
				secondKey = "title_vector"
			}
			idx, err := NewIndexer(ctx, &IndexerConfig{
				Client: client, Embedding: embedder, BatchSize: 10,
				DocumentToHashes: func(ctx context.Context, doc *schema.Document) (*Hashes, error) {
					return &Hashes{Key: doc.ID, Field2Value: map[string]FieldValue{
						"title": {Value: "short", EmbedKey: "title_vector"},
						"body":  {Value: "longer body", EmbedKey: secondKey},
					}}, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			ids, err := idx.Store(ctx, []*schema.Document{{ID: "doc"}})
			if duplicate {
				if err == nil {
					t.Fatalf("Store silently accepted duplicate EmbedKey: ids=%v, executed commands=%d", ids, len(hook.commands))
				}
				if !strings.Contains(err.Error(), "duplicate") {
					t.Fatalf("unexpected error: %v", err)
				}
				if embedder.calls != 0 || len(hook.commands) != 0 {
					t.Fatalf("duplicate mapping produced side effects: calls=%d, commands=%v", embedder.calls, hook.commands)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(ids) != 1 || ids[0] != "doc" || len(hook.commands) != 1 {
				t.Fatalf("unexpected result: ids=%v, commands=%v", ids, hook.commands)
			}
			args := hook.commands[0].Args()
			fields := make(map[string]any)
			for i := 2; i < len(args); i += 2 {
				fields[args[i].(string)] = args[i+1]
			}
			for key, want := range map[string]float64{"title_vector": 5, "body_vector": 11} {
				vector := float64(math.Float32frombits(binary.LittleEndian.Uint32(fields[key].([]byte))))
				if vector != want {
					t.Fatalf("%s vector = %v, want %v", key, vector, want)
				}
			}
		})
	}
}
