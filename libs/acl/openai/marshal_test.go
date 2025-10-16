/*
 * Copyright 2025 CloudWeGo Authors
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

package openai

import (
	"encoding/json"
	"testing"

	"github.com/eino-contrib/jsonschema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/schema"
)

func TestChatCompletionResponseFormatJSONSchemaMarshal(t *testing.T) {
	c := &ChatCompletionResponseFormatJSONSchema{
		Schema: &openapi3.Schema{
			Type: string(schema.Object),
		},
	}
	data, err := json.Marshal(c)
	assert.NoError(t, err)
	assert.Equal(t, `{"name":"","strict":false,"schema":{"type":"object"}}`, string(data))

	c = &ChatCompletionResponseFormatJSONSchema{
		JSONSchema: &jsonschema.Schema{
			Type: string(schema.Object),
		},
	}
	data, err = json.Marshal(c)
	assert.NoError(t, err)
	assert.Equal(t, `{"name":"","strict":false,"schema":{"type":"object"}}`, string(data))
}

func TestChatCompletionResponseFormatJSONSchemaUnmarshalJSON(t *testing.T) {
	c := &ChatCompletionResponseFormatJSONSchema{}
	err := json.Unmarshal([]byte(`{"name":"","strict":false,"schema":{"type":"object"}}`), c)
	assert.NoError(t, err)
	assert.Equal(t, string(schema.Object), c.JSONSchema.Type)
}
