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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/getkin/kin-openapi/openapi3gen"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/openai"
	openai2 "github.com/cloudwego/eino-ext/libs/acl/openai"
)

func main() {
	accessKey := os.Getenv("OPENAI_API_KEY")

	type Person struct {
		Name   string `json:"name"`
		Height int    `json:"height"`
		Weight int    `json:"weight"`
	}
	personSchema, err := openapi3gen.NewSchemaRefForValue(&Person{}, nil)
	if err != nil {
		log.Fatalf("NewSchemaRefForValue failed, err=%v", err)
	}

	ctx := context.Background()
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey: accessKey,
		Model:  "gpt-4o",
		ResponseFormat: &openai2.ChatCompletionResponseFormat{
			Type: openai2.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai2.ChatCompletionResponseFormatJSONSchema{
				Name:        "person",
				Description: "data that describes a person",
				Strict:      false,
				Schema:      personSchema.Value,
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
