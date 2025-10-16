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
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"

	cm, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               "ernie-3.5-8k",
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	ir, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("你好"),
	})

	// Alternatively, you can use the following calling methods
	//ir, err = cm.Generate(ctx, []*schema.Message{
	//	{Role: schema.User,
	//		UserInputMultiContent: []schema.MessageInputPart{
	//			{Type: schema.ChatMessagePartTypeText, Text: "你好"},
	//		}},
	//})

	// If the model supports multimodal scenarios, you can use the following call methods
	//ir, err = cm.Generate(ctx, []*schema.Message{
	//	{Role: schema.User,
	//		UserInputMultiContent: []schema.MessageInputPart{
	//			{Type: schema.ChatMessagePartTypeText, Text: "介绍下如下图片内容"},
	//			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{
	//				URL: of("https://img0.baidu.com/it/u=4078387433,1356951957&fm=253&fmt=auto&app=138&f=JPEG?w=800&h=1034"),
	//			}}},
	//		}},
	//})

	if err != nil {
		log.Fatalf("Generate of qianfan failed, err=%v", err)
	}

	fmt.Println(ir)
	// assistant: 你好！我是文心一言，很高兴与你交流。请问你有什么想问我的吗？无论是关于知识、创作还是其他任何问题，我都会尽力回答你。
}

func of[T any](t T) *T {
	return &t
}
