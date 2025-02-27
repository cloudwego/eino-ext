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

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	hunyuan "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/hunyuan/v20230901"
)

const defaultModel = "hunyuan-embedding"

type EmbeddingConfig struct {
	SecretID  string
	SecretKey string
	Region    string
}

var _ embedding.Embedder = (*Embedder)(nil)

// Embedder is a Tencent Cloud embedding client
type Embedder struct {
	client *hunyuan.Client
}

// NewEmbedder creates a new Tencent Cloud embedding client
func NewEmbedder(ctx context.Context, config *EmbeddingConfig) (*Embedder, error) {
	credential := common.NewCredential(
		config.SecretID,
		config.SecretKey,
	)
	profile := profile.NewClientProfile()

	client, err := hunyuan.NewClient(credential, config.Region, profile)
	if err != nil {
		return nil, err
	}

	return &Embedder{
		client: client,
	}, nil
}

func (e *Embedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) (
	embeddings [][]float64, err error,
) {
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	conf := &embedding.Config{
		Model: defaultModel, // hunyuan embedding does not specify model
	}

	ctx = callbacks.OnStart(ctx, &embedding.CallbackInput{
		Texts:  texts,
		Config: conf,
	})

	// NOTE: len of req.InputList must less equal than 200, so we need to split texts into batches
	// reference: https://pkg.go.dev/github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/hunyuan@v1.0.1093/v20230901#GetEmbeddingRequest.InputList
	batchSize := 200

	req := hunyuan.NewGetEmbeddingRequest()
	req.SetContext(ctx)

	promptTokens, totalTokens := 0, 0
	embeddings = make([][]float64, len(texts))
	for l := 0; l < len(texts); l += batchSize {
		r := min(l+batchSize, len(texts))

		req.InputList = common.StringPtrs(texts[l:r])
		rsp, err := e.client.GetEmbedding(req)
		if err != nil {
			return nil, err
		}

		for idx, d := range rsp.Response.Data {
			embeddings[l+idx] = make([]float64, len(d.Embedding))
			for i, emb := range d.Embedding { // *float64 -> float64
				embeddings[l+idx][i] = *emb
			}
		}

		promptTokens += int(*rsp.Response.Usage.PromptTokens)
		totalTokens += int(*rsp.Response.Usage.TotalTokens)
	}

	callbacks.OnEnd(ctx, &embedding.CallbackOutput{
		Embeddings: embeddings,
		Config:     conf,
		TokenUsage: &embedding.TokenUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: 0, // hunyuan embedding does not has completion tokens
			TotalTokens:      totalTokens,
		},
	})

	return embeddings, nil
}

const typ = "TencentCloud"

func (e *Embedder) GetType() string {
	return typ
}

func (e *Embedder) IsCallbacksEnabled() bool {
	return true
}
