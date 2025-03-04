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
	"time"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
)

type EmbeddingConfig struct {
	// Timeout specifies the maximum duration to wait for API responses
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// APIKey is your authentication key
	// Use OpenAI API key or Azure API key depending on the service
	// Required
	APIKey string `json:"api_key"`

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
	EncodingFormat *openai.EmbeddingEncodingFormat `json:"encoding_format,omitempty"`

	// Dimensions specifies the number of dimensions the resulting output embeddings should have
	// Optional. Only supported in text-embedding-3 and later models
	Dimensions *int `json:"dimensions,omitempty"`

	// User is a unique identifier representing your end-user
	// Optional. Helps OpenAI monitor and detect abuse
	User *string `json:"user,omitempty"`
}

var _ embedding.Embedder = (*Embedder)(nil)

type Embedder struct {
	cli *openai.Client
}

func NewEmbedder(ctx context.Context, config *EmbeddingConfig) (*Embedder, error) {
	var nConf *openai.Config
	if config != nil {
		var httpClient *http.Client

		if config.HTTPClient != nil {
			httpClient = config.HTTPClient
		} else {
			httpClient = &http.Client{Timeout: config.Timeout}
		}

		nConf = &openai.Config{
			ByAzure:        config.ByAzure,
			BaseURL:        config.BaseURL,
			APIVersion:     config.APIVersion,
			APIKey:         config.APIKey,
			HTTPClient:     httpClient,
			Model:          config.Model,
			EncodingFormat: config.EncodingFormat,
			Dimensions:     config.Dimensions,
			User:           config.User,
		}
	}
	cli, err := openai.NewClient(ctx, nConf)
	if err != nil {
		return nil, err
	}

	return &Embedder{
		cli: cli,
	}, nil
}

func (e *Embedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) (
	embeddings [][]float64, err error) {
	return e.cli.EmbedStrings(ctx, texts, opts...)
}

const typ = "OpenAI"

func (e *Embedder) GetType() string {
	return typ
}

func (e *Embedder) IsCallbacksEnabled() bool {
	return true
}
