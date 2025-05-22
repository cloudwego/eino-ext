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
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/retriever/pinecone"
	"github.com/cloudwego/eino/components/embedding"
)

func main() {
	ctx := context.Background()
	b, err := os.ReadFile("./examples/ds_query.json")
	if err != nil {
		panic(err)
	}

	var vs struct {
		Dense [][]float64 `json:"dense"`
	}

	if err := json.Unmarshal(b, &vs); err != nil {
		panic(err)
	}

	apiKey := os.Getenv("PINECONE_API_KEY")
	indexName := "eino-index-test"

	retriever, err := pinecone.NewRetriever(ctx, &pinecone.RetrieverConfig{
		ApiKey:    apiKey,
		IndexName: indexName,
		Namespace: "", // default name space
		Embedding: mockEmbedding{vs.Dense},
	})
	if err != nil {
		panic(err)
	}

	// query by plain text
	resp, err := retriever.Retrieve(ctx, "中国旅游景点")
	if err != nil {
		panic(err)
	}

	for _, doc := range resp {
		fmt.Printf("id=%s content=%s loc=%s, score=%v\n", doc.ID, doc.Content, doc.MetaData["location"], doc.Score())
		// fmt.Printf("dense=%v, sparse=%v\n", doc.DenseVector(), doc.SparseVector())
	}

	// id=2 content=2. 长城：位于中国，是世界七大奇迹之一，从秦至明代修筑而成，全长超过 2 万公里。 loc=中国, score=0.5455964803695679
	// id=5 content=5. 泰姬陵：位于印度阿格拉，由莫卧儿皇帝沙贾汉为纪念其妻子于 1653 年完工，是世界新七大奇迹之一。 loc=印度, score=0.46683382987976074
	// id=8 content=8. 尼亚加拉大瀑布：位于美国和加拿大交界处，由三个主要瀑布组成，其壮观的景象每年吸引数百万游客。 loc=美国, score=0.461596816778183
	// id=1 content=1. 埃菲尔铁塔：位于法国巴黎，是世界上最著名的地标之一，由居斯塔夫・埃菲尔设计并建于 1889 年。 loc=法国, score=0.4575657844543457
	// id=7 content=7. 卢浮宫：位于法国巴黎，是世界上最大的博物馆之一，馆藏丰富，包括达芬奇的《蒙娜丽莎》和希腊的《断臂维纳斯》。 loc=法国, score=0.4530628025531769
}

// mockEmbedding returns embeddings with 1024 dimensions
type mockEmbedding struct {
	dense [][]float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.dense, nil
}
