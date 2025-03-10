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
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	req "github.com/cloudwego/eino-ext/components/tool/httprequest/get"
)

func main() {
	// Configure the GET tool
	config := &req.Config{
		Headers: map[string]string{
			"User-Agent": "MyCustomAgent",
		},
		HttpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{},
		},
	}

	ctx := context.Background()

	// Create the GET tool
	tool, err := req.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// Prepare the GET request payload
	request := &req.GetRequest{
		URL: "https://jsonplaceholder.typicode.com/posts",
	}

	jsonReq, err := sonic.Marshal(request)
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	// Execute the GET request using the InvokableTool interface
	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("GET request failed: %v", err)
	}

	fmt.Println(resp)
}
