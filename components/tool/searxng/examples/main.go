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
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/searxng"
)

func main() {
	ctx := context.Background()

	searxngURL := os.Getenv("SEARXNG_URL")
	if searxngURL == "" {
		// Using default public SearXNG instance, you can also find your own instance at https://searx.space/
		searxngURL = "https://searxng.asenser.cn/"
		log.Printf("Using default SearXNG instance: %s", searxngURL)
		log.Println("You can also use a custom instance by setting the SEARXNG_URL environment variable")
	}

	// Create search request configuration
	requestConfig := searxng.SearchRequestConfig{
		TimeRange:  searxng.TimeRangeMonth,
		Language:   searxng.LanguageEn,
		SafeSearch: searxng.SafeSearchNone,
		Engines: []searxng.Engine{
			searxng.EngineGoogle,
			searxng.EngineDuckDuckGo,
		}, // Use multiple search engines
	}

	// Create search tool
	searchTool, err := searxng.BuildSearchInvokeTool(&searxng.ClientConfig{
		BaseUrl:    searxngURL,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		Headers: map[string]string{
			"User-Agent": "SearXNG-Example/1.0",
		},
		RequestConfig: &requestConfig,
	})
	if err != nil {
		log.Fatal(err)
	}

	// requestConfig was already passed when creating the search tool, only basic parameters needed here
	req := searxng.SearchRequest{
		Query:  "CloudWeGo Eino",
	}

	args, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}

	// Execute search
	resp, err := searchTool.InvokableRun(ctx, string(args))
	if err != nil {
		log.Fatal(err)
	}

	var searchResp searxng.SearchResponse
	if err = json.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatal(err)
	}

	// Print results
	fmt.Println("Search Results:")
	fmt.Println("==============")
	fmt.Printf("Query: %s\n", searchResp.Query)
	fmt.Printf("Number of Results: %d\n\n", searchResp.NumberOfResults)

	for i, result := range searchResp.Results {
		fmt.Printf("%d. Title: %s\n", i+1, result.Title)
		fmt.Printf("   URL: %s\n", result.URL)
		fmt.Printf("   Description: %s\n\n", result.Content)
	}
	fmt.Println("==============")

	// Search Results:
	// ==============
	// Query: CloudWeGo Eino
	// Number of Results: 10

	// 1. Title: Eino: User Manual | CloudWeGo
	//    URL: https://www.cloudwego.io/docs/eino/
	//    Description: Eino provides rich capabilities such as atomic components, integrated components, component orchestration, and aspect extension that assist in AI application development, which can help developers more simply and conveniently develop AI applications with a clear architecture, easy maintenance, and high availability.

	// 2. Title: eino package - github.com/cloudwego/eino - Go Packages
	//    URL: https://pkg.go.dev/github.com/cloudwego/eino
	//    Description: The Eino framework consists of several parts: Eino (this repo): Contains Eino's type definitions, streaming mechanism, component abstractions, orchestration capabilities, aspect mechanisms, etc. EinoExt: Component implementations, callback handlers implementations, component usage examples, and various tools such as evaluators, prompt optimizers.

	// 3. Title: Large Language Model Application Development Framework — Eino is Now ...
	//    URL: https://cloudwego.cn/docs/eino/overview/eino_open_source/
	//    Description: Today, after more than six months of internal use and iteration at ByteDance, the Golang-based comprehensive LLM application development framework — Eino, has been officially open-sourced on CloudWeGo! Based on clear "component" definitions, Eino provides powerful process "orchestration" covering the entire development lifecycle, aiming to help developers create the most ...

	// 4. Title: Eino: Overview | CloudWeGo
	//    URL: https://www.cloudwego.io/docs/eino/overview/
	//    Description: Introduction Eino['aino] (pronounced similarly to "I know, hoping that the framework can achieve the vision of "I know") aims to be the ultimate LLM application development framework in Golang. Drawing inspiration from many excellent LLM application development frameworks in the open-source community such as LangChain & LlamaIndex, etc., as well as learning from cutting-edge research ...

	// 5. Title: Eino · Issue #12 · aaronchenwei/awesome-ai-agent-builder
	//    URL: https://github.com/aaronchenwei/awesome-ai-agent-builder/issues/12
	//    Description: Eino ['aino] (pronounced similarly to "I know") aims to be the ultimate LLM application development framework in Golang. Drawing inspirations from many excellent LLM application development frameworks in the open-source community such as LangChain & LlamaIndex, etc., as well as learning from cutting-edge research and real world applications, Eino offers an LLM application development framework ...

	// 6. Title: Large Language Model Application Development Framework — Eino is Now Open Source! | CloudWeGo
	//    URL: https://www.cloudwego.cn/zh/docs/eino/overview/eino_open_source/
	//    Description: Today, after more than six months of internal use and iteration at ByteDance, the Golang-based comprehensive LLM application development framework — Eino, has been officially open-sourced on CloudWeGo! Based on clear "component" definitions, Eino provides powerful process "orchestration" covering the entire development lifecycle, aiming to help developers create the most profound LLM applications in the fastest way.

	// 7. Title: cloudwego/eino v0.3.29 Released! Key Features Explained, Building a More Robust Era of Subgraph Extraction and Smart Prompting
	//    URL: https://blog.51cto.com/moonfdd/14037491
	//    Description: cloudwego/eino v0.3.29 has been released! With key features explained, it builds a more robust era of subgraph extraction and smart prompting. Eino is an important open-source tool under cloudwego, aimed at providing efficient graph data extraction, processing, and analysis capabilities for cloud-native and distributed systems.

	// 8. Title: Eino: Quick start | CloudWeGo
	//    URL: http://www.cloudwego.io/docs/eino/quick_start/
	//    Description: Brief Description Eino offers various component abstractions for AI application development scenarios and provides multiple implementations, making it very simple to quickly develop an application using Eino. This directory provides several of the most common AI-built application examples to help you get started with Eino quickly. These small applications are only for getting started quickly ...

	// 9. Title: The structure of the Eino Framework | CloudWeGo
	//    URL: https://www.cloudwego.io/docs/eino/overview/eino_framework_structure/
	//    Description: Overall Structure Six key concepts in Eino: Components Abstraction Each Component has a corresponding interface abstraction and multiple implementations. Can be used directly or orchestrated When orchestrated, the node's input/output matches the interface abstraction Similar to out-of-the-box atomic components like ChatModel, PromptTemplate, Retriever, Indexer etc. The Component concept in ...

	// 10. Title: Eino: Overview | CloudWeGo
	//    URL: https://www.cloudwego.io/zh/docs/eino/overview/
	//    Description: Introduction Eino['aino] (pronounced similarly to "I know", hoping that the framework can achieve the vision of "I know") aims to provide the ultimate LLM application development framework based on the Golang language. Drawing inspiration from many excellent LLM application development frameworks in the open-source community such as LangChain and LlamaIndex, while learning from cutting-edge research and practical applications, it provides a framework that emphasizes simplicity, extensibility, reliability, and effectiveness ...

}
