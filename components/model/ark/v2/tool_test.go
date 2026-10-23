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
	"net/http"
	"testing"

	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestToolStrictWire(t *testing.T) {
	for _, defaultValue := range []struct {
		name  string
		value *bool
	}{
		{"unset", nil}, {"enabled", ptr(true)}, {"disabled", ptr(false)},
	} {
		for _, mode := range []string{"bound", "per-call", "stream"} {
			t.Run(defaultValue.name+"/"+mode, func(t *testing.T) {
				tools := []*schema.ToolInfo{toolInfo("inherit"), WithToolStrict(toolInfo("strict"), true), WithToolStrict(toolInfo("flexible"), false)}
				cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					actual := body["tools"].([]any)
					equal(t, 3, len(actual))
					expected := []*bool{defaultValue.value, ptr(true), ptr(false)}
					for i, entry := range actual {
						function := entry.(map[string]any)["function"].(map[string]any)
						value, exists := function["strict"]
						if expected[i] == nil {
							equal(t, false, exists)
						} else {
							equal(t, true, exists)
							equal(t, *expected[i], value)
						}
						parameters := function["parameters"].(map[string]any)
						equal(t, "object", parameters["type"])
						equal(t, "string", parameters["properties"].(map[string]any)["path"].(map[string]any)["type"])
					}
					if mode == "stream" {
						w.Header().Set("Content-Type", "text/event-stream")
						sseChunk(w, modifierChunk)
						sseChunk(w, "[DONE]")
					} else {
						writeResponse(w)
					}
				}, &ChatModelConfig{Model: "model", ToolStrict: defaultValue.value})
				messages := []*schema.Message{{Role: schema.User, Content: "call a tool"}}
				if mode == "per-call" {
					_, err := cm.Generate(context.Background(), messages, fmodel.WithTools(tools))
					check(t, err)
					return
				}
				bound, err := cm.WithTools(tools)
				check(t, err)
				if mode == "stream" {
					reader, err := bound.Stream(context.Background(), messages)
					check(t, err)
					defer reader.Close()
					_, err = reader.Recv()
					check(t, err)
				} else {
					_, err = bound.Generate(context.Background(), messages)
					check(t, err)
				}
			})
		}
	}
}

func TestWithToolStrictPreservesSharedDefinition(t *testing.T) {
	original := toolInfo("shared")
	original.Extra = map[string]any{"label": "original"}
	enabled := WithToolStrict(original, true)
	disabled := WithToolStrict(enabled, false)
	enabled.Extra["label"] = "changed"
	equal(t, "original", original.Extra["label"])
	if _, exists := original.Extra[toolStrictKey]; exists {
		t.Fatal("original tool acquired strict override")
	}
	equal(t, true, enabled.Extra[toolStrictKey])
	equal(t, false, disabled.Extra[toolStrictKey])
	equal(t, "original", disabled.Extra["label"])
}
