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

	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func TestStructuredOutputSerialization(t *testing.T) {
	schemaJSON := []byte(`{"type":"object","properties":{"count":{"type":"integer","minimum":9007199254740993}},"required":["count"],"additionalProperties":false}`)
	format := &chat.ChatCompletionResponseFormat{
		Type: chat.NewOptResponseFormatType(chat.ResponseFormatTypeJSONSchema),
		JSONSchema: chat.NewOptChatCompletionResponseFormatJsonSchema(chat.ChatCompletionResponseFormatJsonSchema{
			Name: "result", Strict: chat.NewOptBool(true), Schema: schemaJSON,
		}),
	}
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		// Use the standard decoder as an independent check of Sonic and SDK output.
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		var body map[string]any
		check(t, decoder.Decode(&body))
		actual := body["response_format"].(map[string]any)
		equal(t, "json_schema", actual["type"])
		configuration := actual["json_schema"].(map[string]any)
		equal(t, true, configuration["strict"])
		equal(t, "result", configuration["name"])
		schema := configuration["schema"].(map[string]any)
		equal(t, false, schema["additionalProperties"])
		count := schema["properties"].(map[string]any)["count"].(map[string]any)
		equal(t, json.Number("9007199254740993"), count["minimum"])
		writeResponse(w)
	}, &ChatModelConfig{Model: "model", ResponseFormat: format})
	_, err := cm.Generate(context.Background(), nil)
	check(t, err)
	equal(t, string(schemaJSON), string(format.JSONSchema.Value.Schema))
}
