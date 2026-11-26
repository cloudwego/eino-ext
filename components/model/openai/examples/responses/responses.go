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

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	cm, err := openai.NewResponsesChatModel(ctx, &openai.ResponsesChatModelConfig{
		APIKey:          os.Getenv("OPENAI_API_KEY"),
		BaseURL:         os.Getenv("OPENAI_BASE_URL"),
		Model:           os.Getenv("OPENAI_MODEL"),
		ReasoningEffort: openai.ResponsesReasoningEffortMedium,
	})
	if err != nil {
		log.Fatal(err)
	}
	history := []*schema.Message{schema.UserMessage("Explain binary search in two sentences.")}
	answer, err := cm.Generate(ctx, history)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(answer.Content)
	history = append(history, answer, schema.UserMessage("What edge cases should I test?"))
	stream, err := cm.Stream(ctx, history)
	if err != nil {
		log.Fatal(err)
	}
	followup, err := schema.ConcatMessageStream(stream)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(followup.Content)
}
