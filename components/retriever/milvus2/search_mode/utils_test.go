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

package search_mode

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	. "github.com/smartystreets/goconvey/convey"
)

// mockEmbedding implements embedding.Embedder for testing
type mockEmbedding struct {
	err  error
	dims int
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([][]float64, len(texts))
	dims := m.dims
	if dims == 0 {
		dims = 128
	}
	for i := range texts {
		result[i] = make([]float64, dims)
		for j := 0; j < dims; j++ {
			result[i][j] = 0.1
		}
	}
	return result, nil
}

func TestEmbedQuery(t *testing.T) {
	Convey("test EmbedQuery", t, func() {
		ctx := context.Background()

		Convey("test embedding success returns float32 vector", func() {
			mockEmb := &mockEmbedding{dims: 128}
			vector, err := EmbedQuery(ctx, mockEmb, "test query")
			So(err, ShouldBeNil)
			So(len(vector), ShouldEqual, 128)
			// First element should be 0.1 converted to float32
			So(vector[0], ShouldEqual, float32(0.1))
		})

		Convey("test embedding error", func() {
			mockEmb := &mockEmbedding{err: fmt.Errorf("embedding failed")}
			vector, err := EmbedQuery(ctx, mockEmb, "test query")
			So(err, ShouldNotBeNil)
			So(vector, ShouldBeNil)
		})

		Convey("test embedding empty result", func() {
			mockEmb := &mockEmbedding{dims: 0}
			// Even with dims=0, the mock returns 128 (default)
			vector, err := EmbedQuery(ctx, mockEmb, "test query")
			So(err, ShouldBeNil)
			So(len(vector), ShouldBeGreaterThan, 0)
		})
	})
}
