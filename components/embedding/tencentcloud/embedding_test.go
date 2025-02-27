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

package tencentcloud

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	hunyuan "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/hunyuan/v20230901"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
)

func TestSingleEmbedding(t *testing.T) {
	expectedRequest := hunyuan.NewGetEmbeddingRequest()
	expectedRequest.InputList = common.StringPtrs([]string{"input"})

	mockPromptTokens := int64(1)
	mockTotalTokens := int64(3)
	mockEmbedding := []float64{0.1}
	mockResponse := hunyuan.NewGetEmbeddingResponse()
	mockResponse.Response = &hunyuan.GetEmbeddingResponseParams{
		Data: []*hunyuan.EmbeddingData{
			{
				Embedding: common.Float64Ptrs(mockEmbedding),
			},
		},
		Usage: &hunyuan.EmbeddingUsage{
			PromptTokens: &mockPromptTokens,
			TotalTokens:  &mockTotalTokens,
		},
	}

	t.Run("tencentcloud single embedding test", func(t *testing.T) {
		ctx := context.Background()
		emb, err := NewEmbedder(ctx, &EmbeddingConfig{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Region:    "test_region",
		})
		if err != nil {
			t.Fatal(err)
		}

		defer mockey.Mock((*hunyuan.Client).GetEmbedding).To(func(client *hunyuan.Client, request *hunyuan.GetEmbeddingRequest) (response *hunyuan.GetEmbeddingResponse, err error) {
			if !reflect.DeepEqual(request.InputList, expectedRequest.InputList) {
				t.Fatal("hunyuan embedding request is unexpected")
				return
			}
			return mockResponse, nil
		}).Build().UnPatch()

		handler := callbacks.NewHandlerBuilder().
			OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
				nOutput := embedding.ConvCallbackOutput(output)
				if nOutput.TokenUsage.PromptTokens != int(mockPromptTokens) {
					t.Fatal("PromptTokens is unexpected")
				}
				if nOutput.TokenUsage.CompletionTokens != 0 {
					t.Fatal("CompletionTokens should be 0")
				}
				if nOutput.TokenUsage.TotalTokens != int(mockTotalTokens) {
					t.Fatal("TotalTokens is unexpected")
				}
				return ctx
			})
		ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{}, handler.Build())
		result, err := emb.EmbedStrings(ctx, []string{"input"})
		if err != nil {
			t.Fatal(err)
		}

		expectedResult := [][]float64{mockEmbedding}
		for i := range result {
			for j := range result[i] {
				if math.Abs(result[i][j]-expectedResult[i][j]) > 1e-7 {
					t.Fatal("result is unexpected")
				}
			}
		}
	})
}

func TestBatchEmbedding(t *testing.T) {
	expectedRequest := hunyuan.NewGetEmbeddingRequest()
	batchSize := 200
	testCaseNum := 1000

	batchInput := make([]string, 0, testCaseNum)
	for i := 0; i < testCaseNum; i++ {
		batchInput = append(batchInput, fmt.Sprintf("input%d", i))
	}

	expectedRequest.InputList = common.StringPtrs(batchInput)

	mockPromptTokens := int64(1000)
	mockTotalTokens := int64(1000)

	mockData := make([]*hunyuan.EmbeddingData, 0, testCaseNum)
	for i := 0; i < testCaseNum; i++ {
		mockData = append(mockData, &hunyuan.EmbeddingData{
			Embedding: []*float64{common.Float64Ptr(float64(i))},
		})
	}

	t.Run("tencentcloud batch embedding test", func(t *testing.T) {
		ctx := context.Background()
		emb, err := NewEmbedder(ctx, &EmbeddingConfig{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Region:    "test_region",
		})
		if err != nil {
			t.Fatal(err)
		}

		defer mockey.Mock((*hunyuan.Client).GetEmbedding).To(func(client *hunyuan.Client, request *hunyuan.GetEmbeddingRequest) (response *hunyuan.GetEmbeddingResponse, err error) {
			for l := 0; l < testCaseNum; l += batchSize {
				r := l + batchSize
				// 有其中一个batch等于request.InputList，则返回mockResponse
				if reflect.DeepEqual(request.InputList, expectedRequest.InputList[l:r]) {
					mockResponse := hunyuan.NewGetEmbeddingResponse()
					mockResponse.Response = &hunyuan.GetEmbeddingResponseParams{
						Data: mockData[l:r],
						Usage: &hunyuan.EmbeddingUsage{
							PromptTokens: common.Int64Ptr(mockPromptTokens * int64(batchSize) / int64(testCaseNum)),
							TotalTokens:  common.Int64Ptr(mockTotalTokens * int64(batchSize) / int64(testCaseNum)),
						},
					}
					return mockResponse, nil
				}
			}
			t.Fatal("request is unexpected")
			return nil, nil
		}).Build().UnPatch()

		handler := callbacks.NewHandlerBuilder().
			OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
				nOutput := embedding.ConvCallbackOutput(output)
				if nOutput.TokenUsage.PromptTokens != int(mockPromptTokens) {
					t.Fatal("PromptTokens is unexpected")
				}
				if nOutput.TokenUsage.CompletionTokens != 0 {
					t.Fatal("CompletionTokens should be 0")
				}
				return ctx
			})
		ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{}, handler.Build())
		result, err := emb.EmbedStrings(ctx, batchInput)
		if err != nil {
			t.Fatal(err)
		}

		expectedResult := make([][]float64, testCaseNum)
		for i := range expectedResult {
			expectedResult[i] = make([]float64, 1)
			expectedResult[i][0] = float64(i)
		}

		for i := range result {
			for j := range result[i] {
				if math.Abs(result[i][j]-expectedResult[i][j]) > 1e-7 {
					t.Fatal("result is unexpected")
				}
			}
		}
	})
}
