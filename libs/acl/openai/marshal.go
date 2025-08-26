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

	"github.com/eino-contrib/jsonschema"
	"github.com/getkin/kin-openapi/openapi3"
)

type schemaUnion struct {
	Schema     *openapi3.Schema
	JSONSchema *jsonschema.Schema
}

func (s *schemaUnion) MarshalJSON() ([]byte, error) {
	if s.JSONSchema != nil {
		return json.Marshal(s.JSONSchema)
	}
	return json.Marshal(s.Schema)
}

func (s *schemaUnion) UnmarshalJSON(data []byte) error {
	s.JSONSchema = &jsonschema.Schema{}
	return json.Unmarshal(data, s.JSONSchema)
}

func (c *ChatCompletionResponseFormatJSONSchema) MarshalJSON() ([]byte, error) {
	type ChatCompletionResponseFormatJSONSchema_ ChatCompletionResponseFormatJSONSchema

	sc := &struct {
		*ChatCompletionResponseFormatJSONSchema_ `json:",inline"`
		SchemaUnion                              *schemaUnion `json:"schema"`
	}{
		ChatCompletionResponseFormatJSONSchema_: (*ChatCompletionResponseFormatJSONSchema_)(c),
		SchemaUnion: &schemaUnion{
			Schema:     c.Schema,
			JSONSchema: c.JSONSchema,
		},
	}

	return json.Marshal(sc)
}

func (c *ChatCompletionResponseFormatJSONSchema) UnmarshalJSON(data []byte) error {
	type ChatCompletionResponseFormatJSONSchema_ ChatCompletionResponseFormatJSONSchema

	sc := &struct {
		*ChatCompletionResponseFormatJSONSchema_ `json:",inline"`
		SchemaUnion                              *schemaUnion `json:"schema"`
	}{
		ChatCompletionResponseFormatJSONSchema_: (*ChatCompletionResponseFormatJSONSchema_)(c),
		SchemaUnion:                             &schemaUnion{},
	}

	err := json.Unmarshal(data, sc)
	if err != nil {
		return err
	}

	c.JSONSchema = sc.SchemaUnion.JSONSchema

	return nil
}
