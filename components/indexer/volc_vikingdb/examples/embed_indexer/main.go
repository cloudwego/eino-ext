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
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb"
)

func main() {
	ctx := context.Background()
	ak := os.Getenv("VOLC_VIKING_DB_AK")
	sk := os.Getenv("VOLC_VIKING_DB_SK")
	collectionName := "eino_test"

	/*
	 * 下面示例中提前构建了一个名为 eino_test 的数据集 (collection)，字段配置为:
	 * 字段名称			字段类型			向量维度
	 * ID				string
	 * vector			vector			1024
	 * content			string
	 * extra_field_1	string
	 *
	 * component 使用时注意:
	 * 1. ID / vector / content 的字段名称与类型与上方配置一致
	 * 2. vector 向量维度需要与 ModelName 对应的模型所输出的向量维度一致
	 */

	cfg := &volc_vikingdb.IndexerConfig{
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
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin: false,
			Embedding:  &mockEmbedding{},
		},
		AddBatchSize: 10,
	}

	indexer, err := volc_vikingdb.NewIndexer(ctx, cfg)
	if err != nil {
		fmt.Printf("NewIndexer failed, %v\n", err)
		return
	}

	doc := &schema.Document{
		ID:      "mock_id_1",
		Content: "A ReAct prompt consists of few-shot task-solving trajectories, with human-written text reasoning traces and actions, as well as environment observations in response to actions",
	}
	volc_vikingdb.SetExtraDataFields(doc, map[string]interface{}{"extra_field_1": "mock_ext_abc"})
	volc_vikingdb.SetExtraDataTTL(doc, 1000)

	docs := []*schema.Document{doc}
	resp, err := indexer.Store(ctx, docs)
	if err != nil {
		fmt.Printf("Store failed, %v\n", err)
		return
	}

	fmt.Printf("vikingDB store success, docs=%v, resp ids=%v\n", docs, resp)
}

type mockEmbedding struct{}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	slice := make([]float64, 1024)
	for i := range slice {
		slice[i] = 1.1
	}

	return [][]float64{slice}, nil
}
