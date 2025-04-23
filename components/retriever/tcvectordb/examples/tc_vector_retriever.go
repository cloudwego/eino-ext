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

package examples

import (
	"context"
	"fmt"
	"os"
	"time"

	oaiembedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	callbacksHelper "github.com/cloudwego/eino/utils/callbacks"

	"github.com/cloudwego/eino-ext/components/retriever/tcvectordb"
)

const (
	EmbeddingModel3Large    = "text-embedding-3-large"
	EmbeddingModelDimension = 3072
)

func main() {
	ctx := context.Background()
	userID := os.Getenv("TC_VECTOR_USER_ID")
	secretKey := os.Getenv("TC_VECTOR_SECRET_KEY")

	// using Openai embedding model
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	openaiBaseURL := os.Getenv("OPENAI_BASE_URL")

	// fake collection name and index name
	collectionName := "eino_examples"
	indexName := "example_index"

	/*
	 * 下面示例假设已经创建了一个名为 eino_examples 的数据集 (collection)，并配置了相应的索引
	 * 数据集字段配置为:
	 * 字段名称			字段类型		向量维度
	 * id				string
	 * vector			vector		3072
	 * content			string
	 * metadata			json
	 *
	 * 使用时注意:
	 * 1. 向量维度需要与 Embedder 输出的向量维度一致
	 * 2. 需要确保collection已创建并正确配置
	 */

	// 创建embedder
	embedder, err := oaiembedding.NewEmbedder(ctx, &oaiembedding.EmbeddingConfig{
		APIKey:     openaiAPIKey,
		BaseURL:    openaiBaseURL,
		Model:      EmbeddingModel3Large,
		Dimensions: &[]int{EmbeddingModelDimension}[0],
	})
	if err != nil {
		fmt.Printf("创建embedder失败: %v\n", err)
		return
	}

	// 创建tcvectordb检索配置
	cfg := &tcvectordb.RetrieverConfig{
		URL:            "http://127.0.0.1:8080", // URL provided by Tencent Cloud
		Username:       userID,                  // Username provided by Tencent Cloud
		Key:            secretKey,               // Key provided by Tencent Cloud
		Database:       "eino_db",               // example db
		Collection:     collectionName,          // The collection where data is stored
		Timeout:        time.Second * 5,
		TopK:           10,
		ScoreThreshold: of(0.75),
		Index:          indexName,
		EmbeddingConfig: tcvectordb.EmbeddingConfig{
			UseBuiltin: false,
			Embedding:  embedder,
		},
	}

	// 创建retriever实例
	tcRetriever, err := tcvectordb.NewRetriever(ctx, cfg)
	if err != nil {
		fmt.Printf("创建TcVectorDB检索器失败: %v\n", err)
		return
	}

	fmt.Println("===== 直接调用检索器 =====")

	query := "腾讯云向量数据库的特点"
	docs, err := tcRetriever.Retrieve(ctx, query)
	if err != nil {
		fmt.Printf("检索失败: %v\n", err)
		return
	}

	fmt.Printf("检索成功，查询=%v，文档数=%v\n", query, len(docs))
	for i, doc := range docs {
		fmt.Printf("文档 %d: 内容=%s, 得分=%f\n", i+1, doc.Content, doc.Score)
	}

	fmt.Println("===== 在Chain中使用检索器 =====")

	// 创建callback handler
	handlerHelper := &callbacksHelper.RetrieverCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *retriever.CallbackInput) context.Context {
			fmt.Printf("开始检索，查询内容: %s\n", input.Query)
			return ctx
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *retriever.CallbackOutput) context.Context {
			fmt.Printf("检索完成，结果数量: %v\n", len(output.Docs))
			return ctx
		},
		// OnError 可选
	}

	// 使用callback handler
	handler := callbacksHelper.NewHandlerHelper().
		Retriever(handlerHelper).
		Handler()

	// 创建chain
	chain := compose.NewChain[string, []*schema.Document]()
	chain.AppendRetriever(tcRetriever)

	// 运行chain
	run, err := chain.Compile(ctx)
	if err != nil {
		fmt.Printf("chain编译失败: %v\n", err)
		return
	}

	outDocs, err := run.Invoke(ctx, query, compose.WithCallbacks(handler))
	if err != nil {
		fmt.Printf("chain执行失败: %v\n", err)
		return
	}

	fmt.Printf("Chain检索成功，查询=%v，文档数=%v\n", query, len(outDocs))
	for i, doc := range outDocs {
		fmt.Printf("文档 %d: 内容=%s, 得分=%f\n", i+1, doc.Content, doc.Score)
	}

}

func of[T any](v T) *T {
	return &v
}
