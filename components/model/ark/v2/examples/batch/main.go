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
	"strings"
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := exampleutil.Run(30*time.Minute, run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ChatModelConfig) error {
	if !strings.HasPrefix(config.Model, "ep-bi-") {
		return fmt.Errorf("Batch Chat requires a dedicated ep-bi-* endpoint: set ARK_MODEL to that endpoint ID")
	}
	parallel := 2
	config.BatchChat = &ark.BatchChatConfig{
		EnableBatchChat:            true,
		BatchChatAsyncRetryTimeout: 30 * time.Minute,
		BatchMaxParallel:           &parallel,
	}
	cm, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return err
	}
	prompts := []string{"Define a Go interface in one sentence.", "Define a Go goroutine in one sentence."}
	type result struct {
		index   int
		message *schema.Message
		err     error
	}
	results := make(chan result, len(prompts))
	for i, prompt := range prompts {
		go func(index int, prompt string) {
			// Each Generate is a synchronous batch-endpoint request; concurrency
			// is controlled by the caller and the SDK's separate batch pool.
			msg, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, Content: prompt}})
			results <- result{index: index, message: msg, err: err}
		}(i, prompt)
	}
	for range prompts {
		result := <-results
		if result.err != nil {
			return fmt.Errorf("batch item %d: %w", result.index, result.err)
		}
		fmt.Printf("[%d] %s (request_id=%s)\n", result.index, result.message.Content, ark.GetArkRequestID(result.message))
	}
	return nil
}
