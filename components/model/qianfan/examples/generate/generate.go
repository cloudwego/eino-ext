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
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"
	modelName := os.Getenv("MODEL_NAME")
	cm, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               modelName,
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
		Seed:                of(0),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	ir, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})

	if err != nil {
		log.Fatalf("Generate of qianfan failed, err=%v", err)
	}

	fmt.Println(ir)
}

func of[T any](t T) *T {
	return &t
}
