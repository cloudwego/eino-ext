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

package ark

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestAllowedToolNamesWire(t *testing.T) {
	for _, mode := range []string{"generate", "stream", "batch"} {
		for _, binding := range []string{"bound", "per-call"} {
			for _, selection := range []struct {
				name   string
				choice schema.ToolChoice
				names  []string
				want   any
			}{
				{"allowed", schema.ToolChoiceAllowed, []string{"inspect"}, "auto"},
				{"allowed subset", schema.ToolChoiceAllowed, []string{"safe", "inspect"}, "auto"},
				{"forced subset", schema.ToolChoiceForced, []string{"safe", "inspect"}, "required"},
				{"forced single", schema.ToolChoiceForced, []string{"safe"}, map[string]any{"type": "function", "function": map[string]any{"name": "safe"}}},
			} {
				t.Run(mode+"/"+binding+"/"+selection.name, func(t *testing.T) {
					tools := []*schema.ToolInfo{toolInfo("safe"), toolInfo("inspect"), toolInfo("other")}
					config := &ChatModelConfig{Model: "test"}
					if mode == "batch" {
						config.BatchChat = &BatchChatConfig{EnableBatchChat: true, BatchChatAsyncRetryTimeout: time.Second}
					}
					cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
						body := decodeRequest(t, r)
						var names []string
						for _, tool := range body["tools"].([]any) {
							names = append(names, tool.(map[string]any)["function"].(map[string]any)["name"].(string))
						}
						equal(t, selection.names, names)
						equal(t, selection.want, body["tool_choice"])
						if mode == "stream" {
							w.Header().Set("Content-Type", "text/event-stream")
							sseChunk(w, modifierChunk)
							sseChunk(w, "[DONE]")
						} else {
							writeResponse(w)
						}
					}, config)
					opts := []fmodel.Option{fmodel.WithToolChoice(selection.choice, selection.names...)}
					if binding == "bound" {
						check(t, cm.BindTools(tools))
					} else {
						opts = append(opts, fmodel.WithTools(tools))
					}
					if mode == "stream" {
						reader, err := cm.Stream(context.Background(), nil, opts...)
						check(t, err)
						defer reader.Close()
						_, err = reader.Recv()
						check(t, err)
						_, err = reader.Recv()
						equal(t, io.EOF, err)
					} else {
						_, err := cm.Generate(context.Background(), nil, opts...)
						check(t, err)
					}
					// Restricting a call must not change the tools on the shared model.
					if binding == "bound" {
						var names []string
						for _, tool := range cm.tools {
							names = append(names, tool.Function.Name)
						}
						equal(t, []string{"safe", "inspect", "other"}, names)
					}
				})
			}
		}
	}
}

func TestUnknownAllowedToolNameDoesNotSendRequest(t *testing.T) {
	for _, choice := range []schema.ToolChoice{schema.ToolChoiceAllowed, schema.ToolChoiceForced} {
		t.Run(string(choice), func(t *testing.T) {
			cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("unknown allowed tool name reached HTTP transport")
				writeResponse(w)
			}, nil)
			check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("safe")}))
			_, err := cm.Generate(context.Background(), nil, fmodel.WithToolChoice(choice, "safe", "missing"))
			if err == nil {
				t.Fatal("expected unknown allowed tool name error")
			}
		})
	}
}
