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

package exampleutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

// RunResponses shares only environment configuration and a bounded context.
// Each example shows its own model construction and API calls.
func RunResponses(run func(context.Context, *ark.ResponsesAPIConfig) error) error {
	config := &ark.ResponsesAPIConfig{
		APIKey: os.Getenv("ARK_API_KEY"), Model: os.Getenv("ARK_MODEL"), BaseURL: os.Getenv("ARK_BASE_URL"),
		Thinking: &chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingModeDisabled)},
	}
	if config.APIKey == "" || config.Model == "" {
		return errors.New("set ARK_API_KEY and ARK_MODEL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	return run(ctx, config)
}

func PrintResponse(message *schema.Message) {
	fmt.Println(message.Content)
	id, _ := ark.GetResponseID(message)
	fmt.Println("Response:", id)
	if message.ResponseMeta != nil {
		fmt.Printf("Usage: %+v\n", message.ResponseMeta.Usage)
	}
}
