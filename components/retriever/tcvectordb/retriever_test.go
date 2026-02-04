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

// run with:
// go test -gcflags="all=-l -N" -v ./...

package tcvectordb

import (
	"context"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/smartystreets/goconvey/convey"
	"github.com/tencent/vectordatabase-sdk-go/tcvectordb"
)

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()

		PatchConvey("test embedding set error", func() {
			ret, err := NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: true, Embedding: &mockEmbedding{fn: func() ([][]float64, error) {
					return [][]float64{{1.1, 1.2}}, nil
				}}},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ret, convey.ShouldBeNil)

			ret, err = NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: false, Embedding: nil},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ret, convey.ShouldBeNil)
		})

		PatchConvey("test new rpc client error", func() {
			ret, err := NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: true, Embedding: nil},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ret, convey.ShouldBeNil)
		})

		PatchConvey("test find database error", func() {
			client := tcvectordb.RpcClient{}

			Mock(tcvectordb.NewRpcClient).Return(client, nil)
			ret, err := NewRetriever(ctx, &RetrieverConfig{
				URL:             "testurl",
				Username:        "testusername",
				Key:             "testkey",
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: true, Embedding: nil},
			})

			convey.So(err, convey.ShouldNotBeNil)
			convey.ShouldAlmostEqual(err.Error(), "[TcVectorDBRetriever] find or create db failed")
			convey.So(ret, convey.ShouldBeNil)
		})
	})
}

type mockEmbedding struct {
	fn func() ([][]float64, error)
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.fn()
}

func (m *mockEmbedding) GetType() string {
	return "asd"
}
