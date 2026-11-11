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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func responsesRequestInputSnapshot(t *testing.T, cm *ResponsesAPIChatModel, common *model.Options, opts *arkOptions, tools []*schema.ToolInfo) map[string]any {
	t.Helper()
	state := struct {
		Config   ResponsesAPIConfig
		Bound    []*schema.ToolInfo
		Common   *model.Options
		Cache    *CacheOption
		Headers  map[string]string
		Search   *responses.WebSearchTool
		Thinking *responses.Thinking
		Tools    []*schema.ToolInfo
	}{cm.config, cm.rawTools, common, opts.cache, opts.customHeaders, opts.enableWebSearch, opts.responsesThinking, tools}
	raw, err := json.Marshal(&state)
	check(t, err)
	var snapshot map[string]any
	check(t, json.Unmarshal(raw, &snapshot))
	return snapshot
}

func assertResponsesRequestInputs(t *testing.T, before, after map[string]any) {
	t.Helper()
	for name, expected := range before {
		if !reflect.DeepEqual(expected, after[name]) {
			t.Errorf("request construction modified %s", name)
		}
	}
}

func TestResponsesRequestInputsAreReadOnly(t *testing.T) {
	for _, kind := range []string{"bound", "filtered", "cached named", "cached allowed"} {
		t.Run(kind, func(t *testing.T) {
			cm := testResponsesModel(t, func(http.ResponseWriter, *http.Request) { t.Error("request preparation reached HTTP") }, &ResponsesAPIConfig{
				Model: "original", Temperature: ptr(float32(0.5)), ToolStrict: ptr(true),
				CustomHeader:        map[string]string{"X-Instance": "original"},
				SessionCache:        &SessionCacheConfig{EnableCache: true, TTL: 3600},
				Thinking:            &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeEnabled)},
				EnableToolWebSearch: &responses.WebSearchTool{Limit: responses.NewOptInt64(2)},
			})
			check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other")}))
			// Keep a sentinel beyond len to detect writes through shared capacity.
			tools := []*schema.ToolInfo{toolInfo("inspect"), toolInfo("other"), toolInfo("sentinel")}
			options := []model.Option{WithMaxToolCalls(3)}
			if kind == "filtered" {
				options = append(options, model.WithTools(tools[:2]), model.WithToolChoice(schema.ToolChoiceAllowed, "inspect"))
			}
			if kind == "cached named" || kind == "cached allowed" {
				options = append(options, WithCache(&CacheOption{HeadPreviousResponseID: ptr("cached")}))
				choice := schema.ToolChoiceForced
				if kind == "cached allowed" {
					choice = schema.ToolChoiceAllowed
				}
				options = append(options, model.WithToolChoice(choice, "inspect"))
			}
			common, opts := cm.getOptions(options...)
			before := responsesRequestInputSnapshot(t, cm, common, opts, tools)
			specified := *opts
			req, err := cm.genRequest([]*schema.Message{schema.UserMessage("hello")}, common, opts)
			check(t, err)
			equal(t, int64(3), req.payload.MaxToolCalls.Or(0))
			assertResponsesRequestInputs(t, before, responsesRequestInputSnapshot(t, cm, common, opts, tools))
			equal(t, true, reflect.DeepEqual(specified, *opts))
			// Reusing the same Option values must also yield the original settings.
			nextCommon, nextOpts := cm.getOptions(options...)
			assertResponsesRequestInputs(t, before, responsesRequestInputSnapshot(t, cm, nextCommon, nextOpts, tools))
		})
	}
}

func TestResponsesSharedRequestOptions(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprint(cached), func(t *testing.T) {
			cm := testResponsesModel(t, func(http.ResponseWriter, *http.Request) { t.Error("request preparation reached HTTP") }, &ResponsesAPIConfig{ToolStrict: ptr(true)})
			check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other")}))
			var options []model.Option
			if cached {
				options = []model.Option{WithCache(&CacheOption{HeadPreviousResponseID: ptr("cached")}), model.WithToolChoice(schema.ToolChoiceForced, "inspect")}
			}
			common, opts := cm.getOptions(options...)
			before := responsesRequestInputSnapshot(t, cm, common, opts, nil)
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := cm.genRequest([]*schema.Message{schema.UserMessage("hello")}, common, opts)
					if err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			assertResponsesRequestInputs(t, before, responsesRequestInputSnapshot(t, cm, common, opts, nil))
		})
	}
}

func TestResponsesSameInstanceCacheLifecycle(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, tc := range []struct {
			name   string
			choice schema.ToolChoice
			names  []string
			wire   any
		}{
			{"allowed single", schema.ToolChoiceAllowed, []string{"inspect"}, "auto"},
			{"allowed multiple", schema.ToolChoiceAllowed, []string{"inspect", "other"}, "auto"},
			{"forced single", schema.ToolChoiceForced, []string{"inspect"}, map[string]any{"type": "function", "name": "inspect"}},
			{"forced multiple", schema.ToolChoiceForced, []string{"inspect", "other"}, "required"},
		} {
			for _, source := range []string{"bound", "option"} {
				t.Run(mode+"/"+tc.name+"/"+source, func(t *testing.T) {
					var count atomic.Int32
					var callbackTools [][]string
					requests := make(chan map[string]any, 4)
					config := &ResponsesAPIConfig{SessionCache: &SessionCacheConfig{EnableCache: true, TTL: 3600}, ToolStrict: ptr(true), CustomHeader: map[string]string{"X-Instance": "original"}, EnableToolWebSearch: &responses.WebSearchTool{}}
					cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
						body := decodeRequest(t, r)
						requests <- body
						if rejectCachedTools(w, body) {
							return
						}
						id := fmt.Sprintf("response-%d", count.Add(1))
						raw := fmt.Sprintf(`{"id":%q,"created_at":1,"object":"response","model":"test","status":"completed","expire_at":4102444800,"output":[{"type":"message","id":"msg","role":"assistant","status":"completed","content":[{"type":"output_text","text":"done","annotations":[]}]}]}`, id)
						if body["stream"] == true {
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(raw)}))
						} else {
							w.Header().Set("Content-Type", "application/json")
							_, _ = io.WriteString(w, raw)
						}
					}, config)
					check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other"), toolInfo("excluded")}))
					handler := callbacks.NewHandlerBuilder().OnStartFn(func(ctx context.Context, _ *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
						var names []string
						for _, tool := range input.(*model.CallbackInput).Tools {
							names = append(names, tool.Name)
						}
						callbackTools = append(callbackTools, names)
						return ctx
					}).Build()
					ctx := callbacks.InitCallbacks(context.Background(), nil, handler)
					options := []model.Option{model.WithToolChoice(tc.choice, tc.names...), WithParallelToolCalls(false)}
					if source == "option" {
						options = append(options, model.WithTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other"), toolInfo("override-excluded")}))
					}
					common, opts := cm.getOptions(options...)
					before := responsesRequestInputSnapshot(t, cm, common, opts, nil)
					history := []*schema.Message{schema.UserMessage("first")}
					call := func(options ...model.Option) *schema.Message {
						if mode == "stream" {
							sr, err := cm.Stream(ctx, history, options...)
							check(t, err)
							msg, err := readResponsesStream(t, sr)
							check(t, err)
							return msg
						}
						msg, err := cm.Generate(ctx, history, options...)
						check(t, err)
						return msg
					}
					first := call(options...)
					firstRequest := <-requests
					firstTools := firstRequest["tools"].([]any)
					equal(t, len(tc.names), len(firstTools))
					for i, tool := range firstTools {
						function := tool.(map[string]any)
						equal(t, tc.names[i], function["name"])
						equal(t, "function", function["type"])
						equal(t, true, function["strict"])
					}
					equal(t, tc.wire, firstRequest["tool_choice"])
					history = append(history, first, schema.UserMessage("second"))
					second := call(options...)
					cachedRequest := <-requests
					equal(t, "response-1", cachedRequest["previous_response_id"])
					equal(t, 1, len(cachedRequest["input"].([]any)))
					equal(t, nil, cachedRequest["tools"])
					equal(t, nil, cachedRequest["tool_choice"])
					history = append(history, second, schema.UserMessage("third"))
					check(t, InvalidateMessageCaches(history))
					_ = call(options...)
					replayedRequest := <-requests
					equal(t, nil, replayedRequest["previous_response_id"])
					equal(t, firstRequest["tools"], replayedRequest["tools"])
					// A later call without the restriction still has every binding and web search.
					_ = call()
					equal(t, 4, len((<-requests)["tools"].([]any)))
					equal(t, [][]string{tc.names, tc.names, tc.names, {"inspect", "other", "excluded"}}, callbackTools)
					afterCommon, afterOpts := cm.getOptions(options...)
					assertResponsesRequestInputs(t, before, responsesRequestInputSnapshot(t, cm, afterCommon, afterOpts, nil))
				})
			}
		}
	}
}

func TestResponsesPrefixCachePreservesOptions(t *testing.T) {
	requests := make(chan map[string]any, 2)
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		requests <- decodeRequest(t, r)
		writeResponses(w)
	}, &ResponsesAPIConfig{SessionCache: &SessionCacheConfig{EnableCache: true, TTL: 3600}, ToolStrict: ptr(true)})
	check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
	cache := &CacheOption{HeadPreviousResponseID: ptr("existing-response"), SessionCache: &SessionCacheConfig{EnableCache: true, TTL: 1800}}
	options := []model.Option{WithCache(cache)}
	common, opts := cm.getOptions(options...)
	before := responsesRequestInputSnapshot(t, cm, common, opts, nil)
	_, err := cm.CreatePrefixCache(context.Background(), []*schema.Message{schema.SystemMessage("prefix")}, 3600, options...)
	check(t, err)
	prefixRequest := <-requests
	equal(t, nil, prefixRequest["previous_response_id"])
	equal(t, true, prefixRequest["caching"].(map[string]any)["prefix"])
	equal(t, 1, len(prefixRequest["tools"].([]any)))
	_, err = cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("continue")}, options...)
	check(t, err)
	continuation := <-requests
	equal(t, "existing-response", continuation["previous_response_id"])
	equal(t, nil, continuation["tools"])
	afterCommon, afterOpts := cm.getOptions(options...)
	assertResponsesRequestInputs(t, before, responsesRequestInputSnapshot(t, cm, afterCommon, afterOpts, nil))
}
