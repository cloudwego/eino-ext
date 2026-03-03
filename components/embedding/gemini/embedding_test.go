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

package gemini

import (
	"context"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
	"google.golang.org/genai"
)

func Test_EmbedStrings(t *testing.T) {
	PatchConvey("test EmbedStrings", t, func() {
		ctx := context.Background()
		mockCli := &genai.Client{
			Models: &genai.Models{},
		}

		embedder, err := NewEmbedder(ctx, &EmbeddingConfig{
			Client: mockCli,
			Model:  "gemini-embedding-001",
		})
		convey.So(err, convey.ShouldBeNil)

		mockResponse := &genai.EmbedContentResponse{
			Embeddings: []*genai.ContentEmbedding{
				{
					Values: []float32{0.1, 0.2},
				},
				{
					Values: []float32{0.3},
				},
				{
					Values: []float32{0.4, 0.5, 0.6},
				},
			},
		}
		expectedResult := [][]float64{
			{0.1, 0.2},
			{0.3},
			{0.4, 0.5, 0.6},
		}
		PatchConvey("test embedding success", func() {
			Mock(GetMethod(mockCli.Models, "EmbedContent")).Return(mockResponse, nil).Build()
			result, err := embedder.EmbedStrings(ctx, []string{"hello world", "你好，世界", "こんにちは世界"})
			if err != nil {
				t.Fatal(err)
			}
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(result), convey.ShouldEqual, 3)

			for i := range result {
				convey.So(len(result[i]), convey.ShouldEqual, len(expectedResult[i]))
			}
		})
	})
}
