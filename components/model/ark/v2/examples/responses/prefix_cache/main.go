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
	"log"

	"fmt"
	"strings"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {

	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	// Cache a reusable prefix. Availability and minimum size depend on the model.
	prefix := strings.Repeat("Reference context for this example. ", 256) + "Remember the code PREFIX_731."
	info, err := cm.CreatePrefixCache(ctx, []*schema.Message{schema.SystemMessage(prefix)}, 3600)
	if err != nil {
		return err
	}
	fmt.Println("Prefix response:", info.ResponseID)
	reply, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("What was the code? Reply with the code only.")}, ark.WithCache(&ark.CacheOption{HeadPreviousResponseID: &info.ResponseID}))
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}
