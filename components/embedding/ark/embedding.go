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
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

var (
	// all default values are from github.com/volcengine/volcengine-go-sdk/service/arkruntime/config.go
	defaultBaseURL    = "https://ark.cn-beijing.volces.com/api/v3"
	defaultRegion     = "cn-beijing"
	defaultRetryTimes = 2
	defaultTimeout    = 10 * time.Minute
)

type EmbeddingConfig struct {
	// Timeout specifies the maximum duration to wait for API responses
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: 10 minutes
	Timeout *time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// RetryTimes specifies the number of retry attempts for failed API calls
	// Optional. Default: 2
	RetryTimes *int `json:"retry_times"`

	// BaseURL specifies the base URL for Ark service
	// Optional. Default: "https://ark.cn-beijing.volces.com/api/v3"
	BaseURL string `json:"base_url"`
	// Region specifies the region where Ark service is located
	// Optional. Default: "cn-beijing"
	Region string `json:"region"`

	// The following three fields are about authentication - either APIKey or AccessKey/SecretKey pair is required
	// For authentication details, see: https://www.volcengine.com/docs/82379/1298459
	// APIKey takes precedence if both are provided
	APIKey    string `json:"api_key"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`

	// Model specifies the ID of endpoint on ark platform
	// Required
	Model string `json:"model"`

	// APIType specifies which api to use: text or multi-modal
	// Optional. Default APITypeText
	APIType *APIType `json:"api_type,omitempty"`

	// MaxConcurrentRequests specifies the maximum number of concurrent multi-modal embedding api calls allowed
	// Optional. Default: 5
	MaxConcurrentRequests *int `json:"max_concurrent_requests"`
}

type APIType string

const (
	// APITypeText uses /embeddings text embedding api, see:
	// VolcEngine
	// API Reference: https://www.volcengine.com/docs/82379/1521766
	// BaseURL: https://ark.cn-beijing.volces.com/api/v3
	APITypeText APIType = "text_api"

	// APITypeMultiModal uses /embeddings/multimodal multi-modal embedding api, see:
	// VolcEngine:
	// API Reference: https://www.volcengine.com/docs/82379/1523520
	// BaseURL: https://ark.cn-beijing.volces.com/api/v3
	// BytePlus:
	// API Reference: https://docs.byteplus.com/en/docs/ModelArk/1409290
	// BaseURL: https://ark.ap-southeast.bytepluses.com/api/v3
	APITypeMultiModal APIType = "multi_modal_api"
)

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
	if config.RetryTimes == nil {
		config.RetryTimes = &defaultRetryTimes
	}
	if config.APIType == nil {
		apiType := APITypeText
		config.APIType = &apiType
	} else if *config.APIType == APITypeMultiModal {
		if config.MaxConcurrentRequests == nil {
			defaultMaxConcurrentRequests := 5
			config.MaxConcurrentRequests = &defaultMaxConcurrentRequests
		}
	}

	opts := []arkruntime.ConfigOption{
		arkruntime.WithRetryTimes(*config.RetryTimes),
		arkruntime.WithBaseUrl(config.BaseURL),
		arkruntime.WithRegion(config.Region),
		arkruntime.WithTimeout(*config.Timeout),
	}
	if config.HTTPClient != nil {
		opts = append(opts, arkruntime.WithHTTPClient(config.HTTPClient))
	}

	if len(config.APIKey) > 0 {
		return arkruntime.NewClientWithApiKey(config.APIKey, opts...)
	}

	return arkruntime.NewClientWithAkSk(config.AccessKey, config.SecretKey, opts...)
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

	options := embedding.GetCommonOptions(&embedding.Options{
		Model: &e.conf.Model,
	}, opts...)
	encodingFormat := model.EmbeddingEncodingFormatFloat
	conf := &embedding.Config{
		Model:          dereferenceOrZero(options.Model),
		EncodingFormat: string(encodingFormat),
	}

	ctx = callbacks.EnsureRunInfo(ctx, e.GetType(), components.ComponentOfEmbedding)
	ctx = callbacks.OnStart(ctx, &embedding.CallbackInput{
		Texts:  texts,
		Config: conf,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	var usage *embedding.TokenUsage

	if e.conf.APIType == nil || *e.conf.APIType == APITypeText {
		resp, err := e.client.CreateEmbeddings(ctx, model.EmbeddingRequestStrings{
			Input:          texts,
			Model:          conf.Model,
			EncodingFormat: encodingFormat,
		})
		if err != nil {
			return nil, fmt.Errorf("[Ark] CreateEmbeddings error: %w", err)
		}

		usage = &embedding.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}

		embeddings = make([][]float64, len(resp.Data))
		for i, d := range resp.Data {
			embeddings[i] = toFloat64(d.Embedding)
		}
	} else {
		mu := sync.Mutex{}
		eg := errgroup.Group{}
		eg.SetLimit(*e.conf.MaxConcurrentRequests)
		usage = &embedding.TokenUsage{}
		embeddings = make([][]float64, len(texts))

		for i := 0; i < len(texts); i++ {
			idx := i
			text := texts[idx]

			eg.Go(func() error {
				res, err := e.client.CreateMultiModalEmbeddings(ctx, model.MultiModalEmbeddingRequest{
					Input: []model.MultimodalEmbeddingInput{
						{Type: model.MultiModalEmbeddingInputTypeText, Text: &text},
					},
					Model:          conf.Model,
					EncodingFormat: &encodingFormat,
				})
				if err != nil {
					return fmt.Errorf("[Ark] CreateMultiModalEmbeddings error: %w", err)
				}

				mu.Lock()
				defer mu.Unlock()

				usage.PromptTokens += res.Usage.PromptTokens
				usage.CompletionTokens += res.Usage.TotalTokens - res.Usage.PromptTokens
				usage.TotalTokens += res.Usage.TotalTokens
				embeddings[idx] = toFloat64(res.Data.Embedding)

				return nil
			})
		}

		if err = eg.Wait(); err != nil {
			return nil, err
		}
	}

	callbacks.OnEnd(ctx, &embedding.CallbackOutput{
		Embeddings: embeddings,
		Config:     conf,
		TokenUsage: usage,
	})

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
		EncodingFormat: model.EmbeddingEncodingFormatFloat, // only support Float for now?
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
