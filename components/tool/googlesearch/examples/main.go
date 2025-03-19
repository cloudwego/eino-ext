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
	"encoding/json"
	"os"

	"github.com/cloudwego/eino-ext/components/tool/googlesearch"
)

func main() {
	ctx := context.Background()

	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	googleSearchEngineID := os.Getenv("GOOGLE_SEARCH_ENGINE_ID")

	if googleAPIKey == "" || googleSearchEngineID == "" {
		panic("[GOOGLE_API_KEY] and [GOOGLE_SEARCH_ENGINE_ID] must set")
	}

	// create tool
	searchTool, err := googlesearch.NewTool(ctx, &googlesearch.Config{
		APIKey:         googleAPIKey,
		SearchEngineID: googleSearchEngineID,
		Lang:           "zh-CN",
		Num:            5,
	})
	if err != nil {
		panic(err)
	}

	// prepare params
	req := googlesearch.SearchRequest{
		Query: "Golang concurrent programming",
		Num:   3,
		Lang:  "en",
	}

	args, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	// do search
	result, err := searchTool.InvokableRun(ctx, string(args))
	if err != nil {
		panic(err)
	}

	// res
	println(result) // in JSON
}
