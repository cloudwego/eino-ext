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
	"time"

	"github.com/cloudwego/eino-ext/callbacks/apmplus"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
)

var cbHandler callbacks.Handler

func main() {
	ctx := context.Background()

	// init apmplus callback, for trace metrics and log
	fmt.Println("INFO: use apmplus as callback, watch at: https://console.volcengine.com/apmplus-server/region:apmplus-server+cn-beijing/console/overview/server?")

	cbh, shutdown := apmplus.NewApmplusHandler(&apmplus.Config{
		Host:        "apmplus-cn-beijing.volces.com:4317",
		AppKey:      "xxx",
		ServiceName: "eino-chat",
		Release:     "release/v0.0.1",
	})
	defer shutdown(ctx)

	callbacks.InitCallbackHandlers([]callbacks.Handler{cbh})
	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      "chat",
		Type:      "llm",
		Component: components.ComponentOfChatModel,
	}, cbh)

	// 使用模版创建messages
	log.Printf("===create messages===\n")
	messages := createMessagesFromTemplate()
	log.Printf("messages: %+v\n\n", messages)

	// 创建llm
	log.Printf("===create llm===\n")
	cm := createOllamaChatModel(ctx)
	log.Printf("create llm success\n\n")
	//
	log.Printf("===llm generate===\n")
	result := generate(ctx, cm, messages)
	log.Printf("result: %+v\n\n", result)

	log.Printf("===llm stream generate===\n")
	streamResult := stream(ctx, cm, messages)
	reportStream(streamResult)
	time.Sleep(10 * time.Second)
}
