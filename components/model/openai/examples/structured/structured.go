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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/openai"
)

func main() {
	type Person struct {
		Name   string `json:"name"`
		Height int    `json:"height"`
		Weight int    `json:"weight"`
	}

	js := &jsonschema.Schema{
		Type: string(schema.Object),
		Properties: orderedmap.New[string, *jsonschema.Schema](
			orderedmap.WithInitialData[string, *jsonschema.Schema](
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "name",
					Value: &jsonschema.Schema{
						Type: string(schema.String),
					},
				},
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "height",
					Value: &jsonschema.Schema{
						Type: string(schema.Integer),
					},
				},
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "weight",
					Value: &jsonschema.Schema{
						Type: string(schema.Integer),
					},
				},
			),
		),
	}

	ctx := context.Background()
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		ByAzure: func() bool {
			if os.Getenv("OPENAI_BY_AZURE") == "true" {
				return true
			}
			return false
		}(),
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:        "person",
				Description: "data that describes a person",
				Strict:      false,
				JSONSchema:  js,
			},
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "Parse the user input into the specified json struct",
		},
		{
			Role:    schema.User,
			Content: "John is one meter seventy tall and weighs sixty kilograms",
		},
	})

	if err != nil {
		log.Fatalf("Generate of openai failed, err=%v", err)
	}

	result := &Person{}
	err = json.Unmarshal([]byte(resp.Content), result)
	if err != nil {
		log.Fatalf("Unmarshal of openai failed, err=%v", err)
	}
	fmt.Printf("%+v", *result)
}
