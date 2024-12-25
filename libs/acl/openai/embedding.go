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
	"fmt"
	"net/http"

	"github.com/sashabaranov/go-openai"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
)

type EmbeddingEncodingFormat string

const (
	EmbeddingEncodingFormatFloat  EmbeddingEncodingFormat = "float"
	EmbeddingEncodingFormatBase64 EmbeddingEncodingFormat = "base64"
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

	// HTTPClient is used to send request.
	HTTPClient *http.Client

	// The following fields have the same meaning as the fields in the openai embedding API request. Ref: https://platform.openai.com/docs/api-reference/embeddings/create
	Model          string                   `json:"model"`
	EncodingFormat *EmbeddingEncodingFormat `json:"encoding_format,omitempty"`
	Dimensions     *int                     `json:"dimensions,omitempty"`
	User           *string                  `json:"user,omitempty"`
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
	} else {
		clientConf = openai.DefaultConfig(config.APIKey)
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

	if options.Model == nil || len(*options.Model) == 0 {
		return nil, fmt.Errorf("open embedder uses empty model")
	}

	req := &openai.EmbeddingRequest{
		Input:          texts,
		Model:          openai.EmbeddingModel(*options.Model),
		User:           dereferenceOrZero(e.config.User),
		EncodingFormat: openai.EmbeddingEncodingFormat(dereferenceOrDefault(e.config.EncodingFormat, EmbeddingEncodingFormatFloat)),
		Dimensions:     dereferenceOrZero(e.config.Dimensions),
	}

	conf := &embedding.Config{
		Model:          string(req.Model),
		EncodingFormat: string(req.EncodingFormat),
	}

	ctx = callbacks.OnStart(ctx, &embedding.CallbackInput{
		Texts:  texts,
		Config: conf,
	})

	resp, err := e.cli.CreateEmbeddings(ctx, *req)
	if err != nil {
		return nil, err
	}

	embeddings = make([][]float64, len(resp.Data))
	for i, d := range resp.Data {
		res := make([]float64, len(d.Embedding))
		for j, emb := range d.Embedding {
			res[j] = float64(emb)
		}
		embeddings[i] = res
	}

	usage := &embedding.TokenUsage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}

	_ = callbacks.OnEnd(ctx, &embedding.CallbackOutput{
		Embeddings: embeddings,
		Config:     conf,
		TokenUsage: usage,
	})

	return embeddings, nil
}
