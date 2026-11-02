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

	config.SessionCache = &ark.SessionCacheConfig{EnableCache: true, TTL: 3600}
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	first, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("Remember the code CACHE_731. Reply OK.")})
	if err != nil {
		return err
	}
	id, _ := ark.GetResponseID(first)
	fmt.Println("Cached response:", id)
	// The adapter finds the cached response and sends only the incremental input.
	reply, err := cm.Generate(ctx, []*schema.Message{first, schema.UserMessage("What was the code? Reply with the code only.")})
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}
