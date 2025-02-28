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
	"os"

	"github.com/cloudwego/eino-ext/components/retriever/dify"
)

// dify 的文档参考 https://docs.dify.ai/zh-hans/guides/knowledge-base/knowledge-and-documents-maintenance/maintain-dataset-via-api

var (
	Endpoint  = "https://api.dify.ai/v1"
	APIKey    = "dataset-api-key"
	DatasetID = "dataset-id"
)

func main() {
	APIKey = os.Getenv("DIFY_DATASET_API_KEY")
	Endpoint = os.Getenv("DIFY_ENDPOINT")
	DatasetID = os.Getenv("DIFY_DATASET_ID")
	// 创建基本的 Dify Retriever
	basicExample()

	// 使用分数阈值的示例
	scoreThresholdExample()
}

func basicExample() {
	ctx := context.Background()

	// 创建基本的 Dify Retriever
	ret, err := dify.NewRetriever(ctx, &dify.RetrieverConfig{
		APIKey:    APIKey,
		Endpoint:  Endpoint,
		DatasetID: DatasetID,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}

	// 执行检索
	docs, err := ret.Retrieve(ctx, "一个简单的例子")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
	}

	// 处理检索结果
	for _, doc := range docs {
		fmt.Printf("文档ID: %s\n", doc.ID)
		fmt.Printf("文档内容: %s\n", doc.Content)
		fmt.Printf("相关性分数: %v\n\n", doc.MetaData["_score"])
	}
}

func scoreThresholdExample() {
	ctx := context.Background()

	// 创建带有分数阈值的 Dify Retriever
	threshold := 0.7 // 设置相关性分数阈值
	ret, err := dify.NewRetriever(ctx, &dify.RetrieverConfig{
		APIKey:    APIKey,
		Endpoint:  Endpoint,
		DatasetID: DatasetID,
		RetrievalModel: &dify.RetrievalModel{
			SearchMethod:   dify.SearchMethodHybrid,
			TopK:           ptrOf(10),
			ScoreThreshold: ptrOf(threshold),
		},
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}

	// 执行检索，只返回相关性分数大于阈值的文档
	docs, err := ret.Retrieve(ctx, "一个简单的例子")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
	}

	// 处理检索结果
	fmt.Printf("找到 %d 个相关性分数大于 %f 的文档\n", len(docs), threshold)
	for _, doc := range docs {
		fmt.Printf("文档ID: %s\n", doc.ID)
		fmt.Printf("文档内容: %s\n", doc.Content)
		fmt.Printf("相关性分数: %v\n\n", doc.MetaData["_score"])
	}
}

// ptrOf 返回传入值的指针
func ptrOf[T any](v T) *T {
	return &v
}
