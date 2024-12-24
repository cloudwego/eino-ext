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
	"net/http"
	"time"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
)

var (
	// all default values are from github.com/volcengine/volcengine-go-sdk/service/arkruntime/config.go
	defaultBaseURL        = "https://ark.cn-beijing.volces.com/api/v3"
	defaultRegion         = "cn-beijing"
	defaultRetryTimes int = 2
	defaultTimeout        = 10 * time.Minute
	defaultClient         = http.Client{Timeout: defaultTimeout}
)

type EmbeddingConfig struct {
	// URL of ark endpoint, default "https://ark.cn-beijing.volces.com/api/v3".
	BaseURL string
	// Region of ark endpoint, default "cn-beijing", see more
	Region string

	HTTPClient *http.Client   `json:"-"`
	Timeout    *time.Duration `json:"timeout"`
	RetryTimes *int           `json:"retry_times"`

	// one of APIKey or ak/sk must be set for authorization.
	APIKey               string
	AccessKey, SecretKey string

	// endpoint_id of the model you use in ark platform, mostly like `ep-20xxxxxxx-xxxxx`.
	Model string
	// A unique identifier representing your end-user, which will help to monitor and detect abuse. see more at https://github.com/volcengine/volcengine-go-sdk/blob/master/service/arkruntime/model/embeddings.go
	User *string
	// Dimensions The number of dimensions the resulting output embeddings should have, different between models.
	Dimensions *int
}

type Embedder struct {
	client *arkruntime.Client
	conf   *EmbeddingConfig
}

func buildClient(config *EmbeddingConfig) *arkruntime.Client {
	if len(config.BaseURL) == 0 {
		config.BaseURL = defaultBaseURL
	}
	if len(config.Region) == 0 {
		config.Region = defaultRegion
	}
	if config.Timeout == nil {
		config.Timeout = &defaultTimeout
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &defaultClient
	}
	if config.RetryTimes == nil {
		config.RetryTimes = &defaultRetryTimes
	}

	if len(config.APIKey) > 0 {
		return arkruntime.NewClientWithApiKey(config.APIKey,
			arkruntime.WithHTTPClient(config.HTTPClient),
			arkruntime.WithRetryTimes(*config.RetryTimes),
			arkruntime.WithBaseUrl(config.BaseURL),
			arkruntime.WithRegion(config.Region),
			arkruntime.WithTimeout(*config.Timeout))
	}

	return arkruntime.NewClientWithAkSk(config.AccessKey, config.SecretKey,
		arkruntime.WithHTTPClient(config.HTTPClient),
		arkruntime.WithRetryTimes(*config.RetryTimes),
		arkruntime.WithBaseUrl(config.BaseURL),
		arkruntime.WithRegion(config.Region),
		arkruntime.WithTimeout(*config.Timeout))
}

func NewEmbedder(ctx context.Context, config *EmbeddingConfig) (*Embedder, error) {

	client := buildClient(config)

	return &Embedder{
		client: client,
		conf:   config,
	}, nil
}

func (e *Embedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) (
	embeddings [][]float64, err error) {

	var (
		cbm, cbmOK = callbacks.ManagerFromCtx(ctx)
	)

	defer func() {
		if err != nil && cbmOK {
			_ = cbm.OnError(ctx, err)
		}
	}()

	req := e.genRequest(texts, opts...)
	conf := &embedding.Config{
		Model:          req.Model,
		EncodingFormat: string(req.EncodingFormat),
	}

	if cbmOK {
		ctx = cbm.OnStart(ctx, &embedding.CallbackInput{
			Texts:  texts,
			Config: conf,
		})
	}

	resp, err := e.client.CreateEmbeddings(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("[Ark]EmbedStrings error: %v", err)
	}

	var usage *embedding.TokenUsage

	usage = &embedding.TokenUsage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}

	embeddings = make([][]float64, len(resp.Data))
	for i, d := range resp.Data {
		embeddings[i] = toFloat64(d.Embedding)
	}

	if cbmOK {
		_ = cbm.OnEnd(ctx, &embedding.CallbackOutput{
			Embeddings: embeddings,
			Config:     conf,
			TokenUsage: usage,
		})
	}

	return embeddings, nil
}

func (e *Embedder) GetType() string {
	return getType()
}

func (e *Embedder) IsCallbacksEnabled() bool {
	return true
}

func (e *Embedder) genRequest(texts []string, opts ...embedding.Option) (
	req model.EmbeddingRequestStrings) {
	options := &embedding.Options{
		Model: &e.conf.Model,
	}

	options = embedding.GetCommonOptions(options, opts...)

	req = model.EmbeddingRequestStrings{
		Input:          texts,
		Model:          dereferenceOrZero(options.Model),
		User:           dereferenceOrZero(e.conf.User),
		EncodingFormat: model.EmbeddingEncodingFormatFloat, // only support Float for now?
		Dimensions:     dereferenceOrZero(e.conf.Dimensions),
	}

	return req
}

func toFloat64(in []float32) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}
