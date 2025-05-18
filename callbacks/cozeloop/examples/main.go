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

	ccb "github.com/JqRrt/eino-ext/callbacks/cozeloop" // todo: 测试后改为github.com/cloudwego/eino-ext
	"github.com/cloudwego/eino/callbacks"
	"github.com/coze-dev/cozeloop-go"
)

func main() {
	// Set the following environment variables first (Assuming you are using a PAT token.).
	// COZELOOP_WORKSPACE_ID=your workspace id
	// COZELOOP_API_TOKEN=your token

	ctx := context.Background()
	client, err := cozeloop.NewClient()
	if err != nil {
		panic(err)
	}
	defer client.Close(ctx)
	// 在服务 init 时 once 调用
	handler := ccb.NewLoopHandler(client)
	callbacks.AppendGlobalHandlers(handler)
}
