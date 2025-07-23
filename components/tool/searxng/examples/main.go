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
		// 使用默认的公共 SearXNG 实例, 你也可以使用自己的实例在 https://searx.space/ 中搜索
		searxngURL = "https://searxng.asenser.cn/"
		log.Printf("使用默认 SearXNG 实例: %s", searxngURL)
		log.Println("你也可以通过设置 SEARXNG_URL 环境变量来使用自定义实例")
	}

	// 创建搜索请求配置
	requestConfig := searxng.SearchRequestConfig{
		TimeRange:  searxng.TimeRangeMonth,
		Language:   searxng.LanguageZh,
		SafeSearch: searxng.SafeSearchNone,
		Engines: []searxng.Engine{
			searxng.EngineGoogle,
			searxng.EngineDuckDuckGo,
		}, // 使用多个搜索引擎
	}

	// 创建搜索工具
	searchTool, err := searxng.BuildSearchInvokeTool(&searxng.ClientConfig{
		BaseUrl:    searxngURL,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		Headers: map[string]string{
			"User-Agent": "SearXNG-Example/1.0",
		},
	}, &requestConfig)
	if err != nil {
		log.Fatal(err)
	}

	// 创建搜索工具时已经传入了 requestConfig，这里只需要基本参数
	req := searxng.SearchRequest{
		Query:  "CloudWeGo Eino",
		PageNo: 1,
	}

	args, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}

	// 执行搜索
	resp, err := searchTool.InvokableRun(ctx, string(args))
	if err != nil {
		log.Fatal(err)
	}

	var searchResp searxng.SearchResponse
	if err = json.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatal(err)
	}

	// 打印结果
	fmt.Println("搜索结果:")
	fmt.Println("==============")
	fmt.Printf("查询: %s\n", searchResp.Query)
	fmt.Printf("结果数量: %d\n\n", searchResp.NumberOfResults)

	for i, result := range searchResp.Results {
		fmt.Printf("%d. 标题: %s\n", i+1, result.Title)
		fmt.Printf("   链接: %s\n", result.URL)
		fmt.Printf("   描述: %s\n\n", result.Content)
	}
	fmt.Println("==============")

	// 搜索结果:
	// ==============
	// 查询: CloudWeGo Eino
	// 结果数量: 10

	// 1. 标题: Eino: User Manual | CloudWeGo
	//    链接: https://www.cloudwego.io/docs/eino/
	//    描述: Eino provides rich capabilities such as atomic components, integrated components, component orchestration, and aspect extension that assist in AI application development, which can help developers more simply and conveniently develop AI applications with a clear architecture, easy maintenance, and high availability.

	// 2. 标题: eino package - github.com/cloudwego/eino - Go Packages
	//    链接: https://pkg.go.dev/github.com/cloudwego/eino
	//    描述: The Eino framework consists of several parts: Eino (this repo): Contains Eino's type definitions, streaming mechanism, component abstractions, orchestration capabilities, aspect mechanisms, etc. EinoExt: Component implementations, callback handlers implementations, component usage examples, and various tools such as evaluators, prompt optimizers.

	// 3. 标题: Large Language Model Application Development Framework — Eino is Now ...
	//    链接: https://cloudwego.cn/docs/eino/overview/eino_open_source/
	//    描述: Today, after more than six months of internal use and iteration at ByteDance, the Golang-based comprehensive LLM application development framework — Eino, has been officially open-sourced on CloudWeGo! Based on clear "component" definitions, Eino provides powerful process "orchestration" covering the entire development lifecycle, aiming to help developers create the most ...

	// 4. 标题: Eino: Overview | CloudWeGo
	//    链接: https://www.cloudwego.io/docs/eino/overview/
	//    描述: Introduction Eino['aino] (pronounced similarly to "I know, hoping that the framework can achieve the vision of "I know") aims to be the ultimate LLM application development framework in Golang. Drawing inspiration from many excellent LLM application development frameworks in the open-source community such as LangChain & LlamaIndex, etc., as well as learning from cutting-edge research ...

	// 5. 标题: Eino · Issue #12 · aaronchenwei/awesome-ai-agent-builder
	//    链接: https://github.com/aaronchenwei/awesome-ai-agent-builder/issues/12
	//    描述: Eino ['aino] (pronounced similarly to "I know") aims to be the ultimate LLM application development framework in Golang. Drawing inspirations from many excellent LLM application development frameworks in the open-source community such as LangChain & LlamaIndex, etc., as well as learning from cutting-edge research and real world applications, Eino offers an LLM application development framework ...

	// 6. 标题: 大语言模型应用开发框架 —— Eino 正式开源! | CloudWeGo
	//    链接: https://www.cloudwego.cn/zh/docs/eino/overview/eino_open_source/
	//    描述: 今天，经过字节跳动内部半年多的使用和迭代，基于 Golang 的大模型应用综合开发框架 —— Eino，已在 CloudWeGo 正式开源啦! Eino 基于明确的"组件"定义，提供强大的流程"编排"，覆盖开发全流程，旨在帮助开发者以最快的速度实现最有深度的大模型应用。 你是否曾有这种感受：想要为自己的 ...

	// 7. 标题: cloudwego/eino v0.3.29全新登场!关键特性详解，构建更稳健的子图抽取和智能提示新时代
	//    链接: https://blog.51cto.com/moonfdd/14037491
	//    描述: cloudwego/eino v0.3.29全新登场! 关键特性详解，构建更稳健的子图抽取和智能提示新时代，eino是cloudwego旗下的一个重要开源工具，旨在为云原生及分布式系统提供高效的图数据抽取、处理和分析能力。

	// 8. 标题: Eino: Quick start | CloudWeGo
	//    链接: http://www.cloudwego.io/docs/eino/quick_start/
	//    描述: Brief Description Eino offers various component abstractions for AI application development scenarios and provides multiple implementations, making it very simple to quickly develop an application using Eino. This directory provides several of the most common AI-built application examples to help you get started with Eino quickly. These small applications are only for getting started quickly ...

	// 9. 标题: The structure of the Eino Framework | CloudWeGo
	//    链接: https://www.cloudwego.io/docs/eino/overview/eino_framework_structure/
	//    描述: Overall Structure Six key concepts in Eino: Components Abstraction Each Component has a corresponding interface abstraction and multiple implementations. Can be used directly or orchestrated When orchestrated, the node's input/output matches the interface abstraction Similar to out-of-the-box atomic components like ChatModel, PromptTemplate, Retriever, Indexer etc. The Component concept in ...

	// 10. 标题: Eino: 概述 | CloudWeGo
	//    链接: https://www.cloudwego.io/zh/docs/eino/overview/
	//    描述: 简介 Eino['aino] (近似音: i know，希望框架能达到 "i know" 的愿景) 旨在提供基于 Golang 语言的终极大模型应用开发框架。 它从开源社区中的诸多优秀 LLM 应用开发框架，如 LangChain 和 LlamaIndex 等获取灵感，同时借鉴前沿研究成果与实际应用，提供了一个强调简洁性、可扩展性、可靠性与有效性，且更 ...

}
