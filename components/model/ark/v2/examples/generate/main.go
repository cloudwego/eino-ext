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
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := exampleutil.Run(2*time.Minute, run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ChatModelConfig) error {
	cm, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return err
	}
	result, err := cm.Generate(ctx, []*schema.Message{
		{Role: schema.System, Content: "Answer concisely."},
		{Role: schema.User, Content: "What is a Go interface?"},
	})
	if err != nil {
		return err
	}
	fmt.Printf("%s\nrequest_id=%s\nusage=%+v\n", result.Content, ark.GetArkRequestID(result), result.ResponseMeta.Usage)
	return nil
}
