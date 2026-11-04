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
	"net/http"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func TestResponsesToolDefinitionsWire(t *testing.T) {
	var parameters jsonschema.Schema
	check(t, json.Unmarshal([]byte(`{"type":"object","properties":{"value":{"anyOf":[{"type":"integer"},{"type":"null"}]}},"required":["value"],"additionalProperties":false,"$defs":{"label":{"type":"string","pattern":"^[a-z]+$"}}}`), &parameters))
	parameters.Extras = map[string]any{"x-sequence": json.Number("9007199254740993")}
	expectedParameters, err := json.Marshal(&parameters)
	check(t, err)
	for _, mode := range []string{"generate", "stream"} {
		for _, source := range []string{"bound", "option"} {
			for _, modified := range []bool{false, true} {
				for _, strict := range []struct {
					name  string
					value *bool
				}{{"unset", nil}, {"true", ptr(true)}, {"false", ptr(false)}} {
					name := mode + "/" + source + "/" + strict.name
					if modified {
						name += "/modifier"
					}
					t.Run(name, func(t *testing.T) {
						base := &schema.ToolInfo{Name: "inspect", Desc: "Inspect a value", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&parameters), Extra: map[string]any{"local-only": "metadata"}}
						tools := []*schema.ToolInfo{base, WithToolStrict(base, true), WithToolStrict(base, false), {Name: "no_args"}}
						tools[1].Name, tools[2].Name = "strict", "flexible"
						before, err := json.Marshal(tools)
						check(t, err)
						config := &ResponsesAPIConfig{ToolStrict: strict.value, EnableToolWebSearch: &responses.WebSearchTool{Limit: responses.NewOptInt64(2)}}
						if modified {
							config.RequestPayloadModifier = func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) { return raw, nil }
						}
						cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
							var body struct {
								Tools  []map[string]json.RawMessage
								Stream bool
							}
							check(t, json.NewDecoder(r.Body).Decode(&body))
							equal(t, 5, len(body.Tools))
							expectedStrict := []*bool{strict.value, ptr(true), ptr(false), strict.value}
							for i, tool := range body.Tools[:4] {
								equal(t, `"function"`, string(tool["type"]))
								var name string
								check(t, json.Unmarshal(tool["name"], &name))
								equal(t, tools[i].Name, name)
								if expectedStrict[i] == nil {
									equal(t, json.RawMessage(nil), tool["strict"])
								} else {
									raw, err := json.Marshal(*expectedStrict[i])
									check(t, err)
									equalRawJSON(t, raw, tool["strict"])
								}
								if i == 3 {
									equal(t, json.RawMessage(nil), tool["parameters"])
									equal(t, json.RawMessage(nil), tool["description"])
								} else {
									equalRawJSON(t, expectedParameters, tool["parameters"])
									equal(t, `"Inspect a value"`, string(tool["description"]))
								}
								equal(t, json.RawMessage(nil), tool["extra"])
								equal(t, json.RawMessage(nil), tool["local-only"])
							}
							equal(t, `"web_search"`, string(body.Tools[4]["type"]))
							equal(t, "2", string(body.Tools[4]["limit"]))
							writeResponsesHistoryReply(w, false, body.Stream)
						}, config)
						var opts []model.Option
						if source == "bound" {
							check(t, cm.BindTools(tools))
						} else {
							opts = append(opts, model.WithTools(tools))
						}
						callResponsesWithOptions(t, cm, mode, []*schema.Message{schema.UserMessage("hello")}, opts...)
						after, err := json.Marshal(tools)
						check(t, err)
						equalRawJSON(t, before, after)
					})
				}
			}
		}
	}
}

func TestResponsesToolDefinitionsPresence(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, kind := range []string{"omitted", "cleared", "search", "cleared with search"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				config := &ResponsesAPIConfig{}
				if kind == "search" || kind == "cleared with search" {
					config.EnableToolWebSearch = &responses.WebSearchTool{}
				}
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					switch kind {
					case "omitted":
						if _, exists := body["tools"]; exists {
							t.Error("unset tools must be omitted")
						}
					case "cleared":
						equal(t, []any{}, body["tools"])
					default:
						tools := body["tools"].([]any)
						equal(t, 1, len(tools))
						equal(t, "web_search", tools[0].(map[string]any)["type"])
					}
					writeResponsesHistoryReply(w, false, body["stream"] == true)
				}, config)
				var opts []model.Option
				if kind == "cleared" || kind == "cleared with search" {
					check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
					opts = append(opts, model.WithTools(nil))
				}
				callResponsesWithOptions(t, cm, mode, []*schema.Message{schema.UserMessage("hello")}, opts...)
			})
		}
	}
}

func TestResponsesInvalidToolsBeforeHTTP(t *testing.T) {
	cm := testResponsesModel(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid tools reached HTTP") }, nil)
	for _, tools := range [][]*schema.ToolInfo{
		{nil},
		{{Name: "invalid", Extra: map[string]any{toolStrictKey: "true"}}},
		{{Name: "invalid", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(jsonschema.FalseSchema)}},
	} {
		if _, err := cm.WithTools(tools); err == nil {
			t.Error("expected binding validation error")
		}
		for _, mode := range []string{"generate", "stream"} {
			var err error
			if mode == "generate" {
				_, err = cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")}, model.WithTools(tools))
			} else {
				var reader *schema.StreamReader[*schema.Message]
				reader, err = cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hello")}, model.WithTools(tools))
				if reader != nil {
					reader.Close()
				}
			}
			if err == nil {
				t.Error("expected request validation error")
			}
		}
	}
	_, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")}, model.WithToolChoice(schema.ToolChoiceForced))
	if err == nil {
		t.Error("forced selection without tools must fail")
	}
}
