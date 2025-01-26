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
	"log"
	"os"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	callbacksHelper "github.com/cloudwego/eino/utils/callbacks"

	"github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb"
)

func main() {
	ctx := context.Background()
	ak := os.Getenv("VOLC_VIKING_DB_AK")
	sk := os.Getenv("VOLC_VIKING_DB_SK")
	collectionName := "eino_test"
	indexName := "test_index_1"

	/*
	 * 下面示例中提前构建了一个名为 eino_test 的数据集 (collection)，并在此数据集上构建了一个名为 test_index_1 的 hnsw-hybrid 索引 (index)
	 * 数据集字段配置为:
	 * 字段名称			字段类型			向量维度
	 * ID				string
	 * vector			vector			1024
	 * sparse_vector 	sparse_vector
	 * content			string
	 * extra_field_1	string
	 *
	 * component 使用时注意:
	 * 1. ID / vector / sparse_vector / content 的字段名称与类型与上方配置一致
	 * 2. vector 向量维度需要与 ModelName 对应的模型所输出的向量维度一致
	 * 3. 部分模型不输出稀疏向量，此时 UseSparse 需要设置为 false，collection 可以不设置 sparse_vector 字段
	 */

	cfg := &volc_vikingdb.RetrieverConfig{
		// https://api-vikingdb.volces.com （华北）
		// https://api-vikingdb.mlp.cn-shanghai.volces.com（华东）
		// https://api-vikingdb.mlp.ap-mya.byteplus.com（海外-柔佛）
		Host:              "api-vikingdb.volces.com",
		Region:            "cn-beijing",
		AK:                ak,
		SK:                sk,
		Scheme:            "https",
		ConnectionTimeout: 0,
		Collection:        collectionName,
		Index:             indexName,
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin:  true,
			ModelName:   "bge-m3",
			UseSparse:   true,
			DenseWeight: 0.4,
		},
		Partition:      "", // 对应索引中的【子索引划分字段】, 未设置时至空即可
		TopK:           of(10),
		ScoreThreshold: of(0.1),
		FilterDSL:      nil, // 对应索引中的【标量过滤字段】，未设置时至空即可，表达式详见 https://www.volcengine.com/docs/84313/1254609
	}

	volcRetriever, err := volc_vikingdb.NewRetriever(ctx, cfg)
	if err != nil {
		log.Printf("NewRetriever failed, %v\n", err)
		return
	}

	log.Printf("===== call Indexer directly =====")

	query := "tourist attraction"
	docs, err := volcRetriever.Retrieve(ctx, query)
	if err != nil {
		log.Printf("vikingDB retrieve failed, %v\n", err)
		return
	}

	log.Printf("vikingDB retrieve success, query=%v, docs=%v", query, docs)

	log.Printf("===== call Indexer in chain =====")

	// 创建 callback handler
	handlerHelper := &callbacksHelper.RetrieverCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *retriever.CallbackInput) context.Context {
			log.Printf("input access, content: %s\n", input.Query)
			return ctx
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *retriever.CallbackOutput) context.Context {
			log.Printf("output finished, len: %v\n", len(output.Docs))
			return ctx
		},
		// OnError
	}

	// 使用 callback handler
	handler := callbacksHelper.NewHandlerHelper().
		Retriever(handlerHelper).
		Handler()

	chain := compose.NewChain[string, []*schema.Document]()
	chain.AppendRetriever(volcRetriever)

	// 在运行时使用
	run, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("chain.Compile failed, err=%v", err)
	}

	outDocs, err := run.Invoke(ctx, query, compose.WithCallbacks(handler))
	if err != nil {
		log.Fatalf("run.Invoke failed, err=%v", err)
	}

	log.Printf("vikingDB retrieve success, query=%v, docs=%v", query, outDocs)
}

func of[T any](v T) *T {
	return &v
}
