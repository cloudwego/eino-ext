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

	"github.com/bytedance/sonic"
	post "github.com/cloudwego/eino-ext/components/tool/httprequest/post"
)

func main() {
	config := &post.Config{
		Headers: map[string]string{
			"User-Agent":   "MyCustomAgent",
			"Content-Type": "application/json; charset=UTF-8",
		},
	}

	ctx := context.Background()

	tool, err := post.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	request := &post.PostRequest{
		URL:  "https://jsonplaceholder.typicode.com/posts",
		Body: `{"title": "my title","body": "my body","userId": 1}`,
	}

	jsonReq, err := sonic.Marshal(request)

	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Post failed: %v", err)
	}

	fmt.Println(resp)
}
