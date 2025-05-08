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
	"strings"

	"github.com/cloudwego/eino-ext/components/indexer/pinecone"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	str := `1. 埃菲尔铁塔：位于法国巴黎，是世界上最著名的地标之一，由居斯塔夫・埃菲尔设计并建于 1889 年。
2. 长城：位于中国，是世界七大奇迹之一，从秦至明代修筑而成，全长超过 2 万公里。
3. 大峡谷国家公园：位于美国亚利桑那州，以其深邃的峡谷和壮丽景色而闻名，峡谷由科罗拉多河切割而成。
4. 罗马斗兽场：位于意大利罗马，于公元 70-80 年间建成，是古罗马帝国最大的圆形竞技场。
5. 泰姬陵：位于印度阿格拉，由莫卧儿皇帝沙贾汉为纪念其妻子于 1653 年完工，是世界新七大奇迹之一。
6. 悉尼歌剧院：位于澳大利亚悉尼港，是 20 世纪最具标志性的建筑之一，以其独特的帆船造型闻名于世。
7. 卢浮宫：位于法国巴黎，是世界上最大的博物馆之一，馆藏丰富，包括达芬奇的《蒙娜丽莎》和希腊的《断臂维纳斯》。
8. 尼亚加拉大瀑布：位于美国和加拿大交界处，由三个主要瀑布组成，其壮观的景象每年吸引数百万游客。
9. 圣索菲亚大教堂：位于土耳其伊斯坦布尔，最初建于公元 537 年，曾作为东正教大教堂和清真寺，现在是博物馆。
10. 马丘比丘：位于秘鲁安第斯山脉高原上的古印加遗址，是世界新七大奇迹之一，海拔超过 2400 米。`
	loc := []string{"法国", "中国", "美国", "意大利", "印度", "澳大利亚", "法国", "美国", "土耳其", "秘鲁"}

	// dense / sparse vector by bge-m3
	b, err := os.ReadFile("./examples/upsert_data/ds.json")
	if err != nil {
		panic(err)
	}

	var vs struct {
		Dense  [][]float64       `json:"dense"`
		Sparse []map[int]float64 `json:"sparse"`
	}

	if err := json.Unmarshal(b, &vs); err != nil {
		panic(err)
	}

	apiKey := os.Getenv("PINECONE_API_KEY")
	indexName := "eino-index-test" // replace with your own index
	indexer, err := pinecone.NewIndexer(ctx, &pinecone.IndexerConfig{
		ApiKey:    apiKey,
		IndexName: indexName,
		Namespace: "", // default name space
		Embedding: &mockEmbedding{vs.Dense},
	})
	if err != nil {
		panic(err)
	}

	var docs []*schema.Document
	for i, content := range strings.Split(str, "\n") {
		doc := &schema.Document{
			ID:      fmt.Sprintf("%d", i+1),
			Content: content,
			MetaData: map[string]any{
				"location": loc[i],
			},
		}

		// Eino Embedding interface doesn't support return of sparse vector,
		// so you need to set sparse vector of document manually
		doc.WithSparseVector(vs.Sparse[i])

		docs = append(docs, doc)
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		panic(err)
	}

	fmt.Println(ids)
	// [1 2 3 4 5 6 7 8 9 10]
}

// mockEmbedding returns embeddings with 1024 dimensions
type mockEmbedding struct {
	dense [][]float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.dense, nil
}
