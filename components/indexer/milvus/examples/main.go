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
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

func main() {
	// Get the environment variables
	addr := os.Getenv("MILVUS_ADDR")
	username := os.Getenv("MILVUS_USERNAME")
	password := os.Getenv("MILVUS_PASSWORD")

	// Create a client
	ctx := context.Background()
	cli, err := client.NewClient(ctx, client.Config{
		Address:  addr,
		Username: username,
		Password: password,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		return
	}
	defer cli.Close()

	// Create an indexer
	indexer, err := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
		Client:    cli,
		Embedding: &mockEmbedding{},
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
		return
	}
	log.Printf("Indexer created success")

	// Store documents
	//docs := []*schema.Document{
	//	{
	//		ID:      "milvus-1",
	//		Content: "milvus is an open-source vector database",
	//		MetaData: map[string]any{
	//			"h1": "milvus",
	//			"h2": "open-source",
	//			"h3": "vector database",
	//		},
	//	},
	//	{
	//		ID:      "milvus-2",
	//		Content: "milvus is a distributed vector database",
	//	},
	//}

	// 直接调用 store，无需手动分片，由内部方法实现异步分批插入
	docs := UseSplitter("./examples/test.md")
	start := time.Now()
	_, err = indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store: %v", err)
		return
	}

	elapsed := time.Since(start)

	log.Printf(" All docs stored success. Total time: %v", elapsed)
}

type vector struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	bytes, err := os.ReadFile("./examples/embeddings.json")
	if err != nil {
		return nil, err
	}
	var v vector
	if err := sonic.Unmarshal(bytes, &v); err != nil {
		return nil, err
	}
	res := make([][]float64, 0, len(v.Data))
	for _, data := range v.Data {
		res = append(res, data.Embedding)
	}
	return res, nil
}

// UseSplitter 测试用 markdown 分割器
func UseSplitter(filePath string) []*schema.Document {
	ctx := context.Background()
	// 初始化分割器
	splitter, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{
			"#":   "h1",
			"##":  "h2",
			"###": "h3",
		},
		TrimHeaders: false,
	})
	if err != nil {
		panic("Failed to create a Splitter:" + err.Error())
	}
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("文件不存在: %v", err)
		}
		fmt.Printf("无法访问文件: %v", err)
	}
	// 检查文件权限
	mode := fileInfo.Mode()
	if mode&0400 == 0 { // 检查所有者读权限
		fmt.Printf("没有读取权限")
	}
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
	docs := []*schema.Document{
		{
			ID:      "doc1",
			Content: string(bytes),
		},
	}
	results, err := splitter.Transform(ctx, docs)
	if err != nil {
		panic(err)
	}
	// 处理分割结果
	for i, doc := range results {
		//println("片段", i+1, ":", doc.Content)
		doc.ID = docs[0].ID + "_" + strconv.Itoa(i)
	}
	return results
}
