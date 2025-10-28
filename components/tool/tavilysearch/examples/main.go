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

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/tavilysearch"
)

func main() {
	ctx := context.Background()
	country := "china"
	ts, err := tavilysearch.NewTool(ctx, &tavilysearch.Config{
		APIKey:  os.Getenv("TAVILY_API_KEY"),
		Country: &country,
	})
	if err != nil {
		log.Fatalf("create tavily search tool failed, %v", err)
		return
	}
	req := tavilysearch.SearchRequest{
		Query: "What is transformer?",
	}
	reqStr, _ := sonic.MarshalString(&req)
	tout, err := ts.InvokableRun(ctx, reqStr)
	if err != nil {
		log.Fatalf("invokable run failed, %v", err)
		return
	}
	fmt.Println(tout)
}
