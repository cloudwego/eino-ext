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

package qianfan

import (
	"context"

	"github.com/baidubce/bce-qianfan-sdk/go/qianfan"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
)

// GetQianfanSingletonConfig qianfan config is singleton, you should set ak+sk / bear_token before init chat model
// Set with code: GetQianfanSingletonConfig().AccessKey = "your_access_key"
// Set with env: os.Setenv("QIANFAN_ACCESS_KEY", "your_iam_ak") or with env file
func GetQianfanSingletonConfig() *qianfan.Config {
	return qianfan.GetConfig()
}

type EmbeddingConfig struct {
	Model                 string
	LLMRetryCount         *int
	LLMRetryTimeout       *float32
	LLMRetryBackoffFactor *float32
}

type Embedder struct {
	conf  *EmbeddingConfig
	embed *qianfan.Embedding
}

func NewEmbedder(ctx context.Context, config *EmbeddingConfig) (*Embedder, error) {
	opts := []qianfan.Option{qianfan.WithModel(config.Model)}
	if config.LLMRetryCount != nil {
		opts = append(opts, qianfan.WithLLMRetryCount(*config.LLMRetryCount))
	}
	if config.LLMRetryTimeout != nil {
		opts = append(opts, qianfan.WithLLMRetryTimeout(*config.LLMRetryTimeout))
	}
	if config.LLMRetryBackoffFactor != nil {
		opts = append(opts, qianfan.WithLLMRetryBackoffFactor(*config.LLMRetryBackoffFactor))
	}

	return &Embedder{
		conf:  config,
		embed: qianfan.NewEmbedding(opts...),
	}, nil
}

func (e *Embedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) (embeddings [][]float64, err error) {
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	options := embedding.GetCommonOptions(&embedding.Options{
		Model: &e.conf.Model,
	}, opts...)

	conf := &embedding.Config{Model: *options.Model}

	ctx = callbacks.EnsureRunInfo(ctx, e.GetType(), components.ComponentOfEmbedding)
	ctx = callbacks.OnStart(ctx, &embedding.CallbackInput{
		Texts:  texts,
		Config: conf,
	})

	resp, err := e.embed.Do(ctx, &qianfan.EmbeddingRequest{Input: texts})
	if err != nil {
		return nil, err
	}

	embeddings = make([][]float64, len(resp.Data))
	for i := range resp.Data {
		embeddings[i] = resp.Data[i].Embedding
	}

	callbacks.OnEnd(ctx, &embedding.CallbackOutput{
		Embeddings: embeddings,
		Config:     conf,
		TokenUsage: &embedding.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	})

	return embeddings, nil
}

const typ = "QianFan"

func (e *Embedder) GetType() string {
	return typ
}

func (e *Embedder) IsCallbacksEnabled() bool {
	return true
}
