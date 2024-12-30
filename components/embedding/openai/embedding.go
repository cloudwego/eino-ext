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
	// if you want to use Azure OpenAI Service, set the next three fields. refs: https://learn.microsoft.com/en-us/azure/ai-services/openai/
	// ByAzure set this field to true when using Azure OpenAI Service, otherwise it does not need to be set.
	ByAzure bool `json:"by_azure"`
	// BaseURL https://{{$YOUR_RESOURCE_NAME}}.openai.azure.com, YOUR_RESOURCE_NAME is the name of your resource that you have created on Azure.
	BaseURL string `json:"base_url"`
	// APIVersion specifies the API version you want to use.
	APIVersion string `json:"api_version"`

	// APIKey is typically OPENAI_API_KEY, but if you have set up Azure, then it is Azure API_KEY.
	APIKey string `json:"api_key"`

	// Timeout specifies the http request timeout.
	Timeout time.Duration `json:"timeout"`

	// The following fields have the same meaning as the fields in the openai embedding API request. Ref: https://platform.openai.com/docs/api-reference/embeddings/create
	Model          string                          `json:"model"`
	EncodingFormat *openai.EmbeddingEncodingFormat `json:"encoding_format,omitempty"`
	Dimensions     *int                            `json:"dimensions,omitempty"`
	User           *string                         `json:"user,omitempty"`
}

var _ embedding.Embedder = (*Embedder)(nil)

type Embedder struct {
	cli *openai.EmbeddingClient
}

func NewEmbedder(ctx context.Context, config *EmbeddingConfig) (*Embedder, error) {
	var nConf *openai.EmbeddingConfig
	if config != nil {
		nConf = &openai.EmbeddingConfig{
			ByAzure:        config.ByAzure,
			BaseURL:        config.BaseURL,
			APIVersion:     config.APIVersion,
			APIKey:         config.APIKey,
			HTTPClient:     &http.Client{Timeout: config.Timeout},
			Model:          config.Model,
			EncodingFormat: config.EncodingFormat,
			Dimensions:     config.Dimensions,
			User:           config.User,
		}
	}
	cli, err := openai.NewEmbeddingClient(ctx, nConf)
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
