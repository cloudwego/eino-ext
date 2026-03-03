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

package ark

import (
	"context"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino/components/embedding"
)

func Test_EmbedStrings(t *testing.T) {
	PatchConvey("test buildClient", t, func() {

		buildClient(&EmbeddingConfig{
			AccessKey: "mock",
			SecretKey: "mock",
			BaseURL:   "mock",
			Model:     "mock",
			Region:    "mock",
		})

		buildClient(&EmbeddingConfig{
			APIKey: "mock",
			Model:  "mock",
		})
	})
	PatchConvey("test EmbedStrings", t, func() {
		ctx := context.Background()
		mockCli := &arkruntime.Client{}
		Mock(buildClient).Return(mockCli).Build()

		embedder, err := NewEmbedder(ctx, &EmbeddingConfig{})
		convey.So(err, convey.ShouldBeNil)

		PatchConvey("test embedding error", func() {
			Mock(GetMethod(mockCli, "CreateEmbeddings")).Return(nil, fmt.Errorf("mock err")).Build()

			vector, err := embedder.EmbedStrings(ctx, []string{"asd"})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(len(vector), convey.ShouldEqual, 0)
		})

		PatchConvey("test embedding success", func() {
			Mock(GetMethod(mockCli, "CreateEmbeddings")).Return(model.EmbeddingResponse{
				Data: []model.Embedding{
					{
						Embedding: []float32{1, 2, 3},
						Index:     0,
						Object:    "embedding",
					},
				},
				Usage: model.Usage{
					CompletionTokens: 1,
					PromptTokens:     2,
					TotalTokens:      3,
				},
			}, nil).Build()

			vector, err := embedder.EmbedStrings(ctx, []string{"asd"}, embedding.WithModel("mock"))
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(vector), convey.ShouldEqual, 1)
		})
	})
}

func TestAPITypeMultiModal(t *testing.T) {
	PatchConvey("test APITypeMultiModal", t, func() {
		ctx := context.Background()

		PatchConvey("test buildClient", func() {
			cfg := &EmbeddingConfig{APIType: nil}
			buildClient(cfg)
			convey.So(*cfg.APIType, convey.ShouldEqual, APITypeText)

			at := APITypeMultiModal
			cfg = &EmbeddingConfig{APIType: &at}
			buildClient(cfg)
			convey.So(*cfg.MaxConcurrentRequests, convey.ShouldEqual, 5)
		})

		PatchConvey("test EmbedStrings", func() {
			at := APITypeMultiModal
			mcr := 5
			cfg := &EmbeddingConfig{APIType: &at, MaxConcurrentRequests: &mcr}
			mockCli := &arkruntime.Client{}
			emb := &Embedder{client: mockCli, conf: cfg}

			PatchConvey("test CreateMultiModalEmbeddings error", func() {
				e := fmt.Errorf("mock err")
				Mock(GetMethod(mockCli, "CreateMultiModalEmbeddings")).Return(model.MultimodalEmbeddingResponse{}, e).Build()
				res, err := emb.EmbedStrings(ctx, []string{"asd", "qwe"})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[Ark] CreateMultiModalEmbeddings error: %w", e))
				convey.So(res, convey.ShouldBeNil)
			})

			PatchConvey("test CreateMultiModalEmbeddings success", func() {
				v := []float32{0.1, 0.2, 0.3}
				Mock(GetMethod(mockCli, "CreateMultiModalEmbeddings")).Return(model.MultimodalEmbeddingResponse{
					Data: model.MultimodalEmbedding{
						Embedding: v,
						Object:    "embedding",
					},
					Usage: model.MultimodalEmbeddingUsage{
						PromptTokens: 1,
						TotalTokens:  3,
					},
				}, nil).Build()
				res, err := emb.EmbedStrings(ctx, []string{"asd", "qwe"})
				convey.So(err, convey.ShouldBeNil)
				convey.So(len(res), convey.ShouldEqual, 2)
				convey.So(res[0], convey.ShouldEqual, toFloat64(v))
				convey.So(res[1], convey.ShouldEqual, toFloat64(v))
			})
		})
	})
}
