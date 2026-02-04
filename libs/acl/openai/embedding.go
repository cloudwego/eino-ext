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

package openai

import (
	"context"
	"net/http"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/meguminnnnnnnnn/go-openai"
)

type EmbeddingEncodingFormat string

const (
	EmbeddingEncodingFormatFloat  EmbeddingEncodingFormat = "float"
	EmbeddingEncodingFormatBase64 EmbeddingEncodingFormat = "base64"
)

type EmbeddingConfig struct {
	// APIKey is your authentication key
	// Use OpenAI API key or Azure API key depending on the service
	// Required
	APIKey string `json:"api_key"`

	// HTTPClient is used to send HTTP requests
	// Optional. Default: http.DefaultClient
	HTTPClient *http.Client

	// The following three fields are only required when using Azure OpenAI Service, otherwise they can be ignored.
	// For more details, see: https://learn.microsoft.com/en-us/azure/ai-services/openai/

	// ByAzure indicates whether to use Azure OpenAI Service
	// Required for Azure
	ByAzure bool `json:"by_azure"`

	// BaseURL is the Azure OpenAI endpoint URL
	// Format: https://{YOUR_RESOURCE_NAME}.openai.azure.com. YOUR_RESOURCE_NAME is the name of your resource that you have created on Azure.
	// Required for Azure
	BaseURL string `json:"base_url"`

	// APIVersion specifies the Azure OpenAI API version
	// Required for Azure
	APIVersion string `json:"api_version"`

	// The following fields correspond to OpenAI's chat completion API parameters
	//Ref: https://platform.openai.com/docs/api-reference/embeddings/create

	// Model specifies the ID of the model to use for embedding generation
	// Required
	Model string `json:"model"`

	// EncodingFormat specifies the format of the embeddings output
	// Optional. Default: EmbeddingEncodingFormatFloat
	EncodingFormat *EmbeddingEncodingFormat `json:"encoding_format,omitempty"`

	// Dimensions specifies the number of dimensions the resulting output embeddings should have
	// Optional. Only supported in text-embedding-3 and later models
	Dimensions *int `json:"dimensions,omitempty"`

	// User is a unique identifier representing your end-user
	// Optional. Helps OpenAI monitor and detect abuse
	User *string `json:"user,omitempty"`

	// BatchSize specifies the number of texts to embed in a single request
	// Optional.
	BatchSize int `json:"batch_size,omitempty"`
}

var _ embedding.Embedder = (*EmbeddingClient)(nil)

type EmbeddingClient struct {
	cli    *openai.Client
	config *EmbeddingConfig
}

func NewEmbeddingClient(ctx context.Context, config *EmbeddingConfig) (*EmbeddingClient, error) {
	if config == nil {
		config = &EmbeddingConfig{Model: string(openai.AdaEmbeddingV2)}
	}

	var clientConf openai.ClientConfig

	if config.ByAzure {
		clientConf = openai.DefaultAzureConfig(config.APIKey, config.BaseURL)
		if config.APIVersion != "" {
			clientConf.APIVersion = config.APIVersion
		}
	} else {
		clientConf = openai.DefaultConfig(config.APIKey)
		if config.BaseURL != "" {
			clientConf.BaseURL = config.BaseURL
		}
	}

	clientConf.HTTPClient = config.HTTPClient
	if clientConf.HTTPClient == nil {
		clientConf.HTTPClient = http.DefaultClient
	}

	return &EmbeddingClient{
		cli:    openai.NewClientWithConfig(clientConf),
		config: config,
	}, nil
}

func (e *EmbeddingClient) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) (
	embeddings [][]float64, err error) {

	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	options := &embedding.Options{
		Model: &e.config.Model,
	}
	options = embedding.GetCommonOptions(options, opts...)

	req := &openai.EmbeddingRequest{
		Model:          openai.EmbeddingModel(*options.Model),
		User:           dereferenceOrZero(e.config.User),
		EncodingFormat: openai.EmbeddingEncodingFormat(dereferenceOrDefault(e.config.EncodingFormat, EmbeddingEncodingFormatFloat)),
		Dimensions:     dereferenceOrZero(e.config.Dimensions),
	}

	conf := &embedding.Config{
		Model:          string(req.Model),
		EncodingFormat: string(req.EncodingFormat),
	}

	embeddings = make([][]float64, 0, len(texts))
	usage := &embedding.TokenUsage{
		PromptTokens:     0,
		CompletionTokens: 0,
		TotalTokens:      0,
	}

	var batchSize int
	if e.config.BatchSize == 0 {
		batchSize = len(texts)
	} else {
		batchSize = e.config.BatchSize
	}

	ctx = callbacks.OnStart(ctx, &embedding.CallbackInput{
		Texts:  texts,
		Config: conf,
	})

	for i := 0; i < len(texts); i += batchSize {
		idx := i
		var end int
		if idx+batchSize > len(texts) {
			end = len(texts)
		} else {
			end = idx + batchSize
		}
		req.Input = texts[idx:end]
		resp, err2 := e.cli.CreateEmbeddings(ctx, *req)
		if err2 != nil {
			return nil, err2
		}

		for _, d := range resp.Data {
			res := make([]float64, len(d.Embedding))
			for k, emb := range d.Embedding {
				res[k] = float64(emb)
			}
			embeddings = append(embeddings, res)
		}

		usage.PromptTokens += resp.Usage.PromptTokens
		usage.CompletionTokens += resp.Usage.CompletionTokens
		usage.TotalTokens += resp.Usage.TotalTokens
	}

	_ = callbacks.OnEnd(ctx, &embedding.CallbackOutput{
		Embeddings: embeddings,
		Config:     conf,
		TokenUsage: usage,
	})

	return embeddings, nil
}
