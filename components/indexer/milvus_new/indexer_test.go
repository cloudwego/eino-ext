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

package milvus_new

import (
	"context"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/smartystreets/goconvey/convey"
)

// 模拟Embedding实现
type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = make([]float64, 768) // 使用768维向量
		for j := range result[i] {
			result[i][j] = 0.1
		}
	}
	return result, nil
}

func TestIndexer_Store(t *testing.T) {
	PatchConvey("test Indexer.Store", t, func() {
		ctx := context.Background()

		PatchConvey("test store with empty documents", func() {
			// 创建索引器
			mockEmb := &mockEmbedding{}
			indexer := &Indexer{
				config: IndexerConfig{
					Embedding: mockEmb,
				},
			}

			// 测试空文档的情况
			emptyDocs := []*schema.Document{}
			ids, err := indexer.Store(ctx, emptyDocs)
			convey.So(err, convey.ShouldBeError)
			convey.So(ids, convey.ShouldBeNil)
		})
	})
}
