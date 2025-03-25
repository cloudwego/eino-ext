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

package dashscope

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
)

const (
	baseUrl    = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	dimensions = 1024
)

type EmbeddingConfig struct {
	// APIKey is typically OPENAI_API_KEY, but if you have set up Azure, then it is Azure API_KEY.
	APIKey string `json:"api_key"`
	// Timeout specifies the http request timeout.
	// If HTTPClient is set, Timeout will not be used.
	Timeout time.Duration `json:"timeout"`
	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// The following fields have the same meaning as the fields in the openai embedding API request.
	// OpenAI Ref: https://platform.openai.com/docs/api-reference/embeddings/create
	// DashScope Ref: https://help.aliyun.com/zh/model-studio/developer-reference/text-embedding-synchronous-api?spm=a2c4g.11186623.help-menu-2400256.d_3_3_9_2.532bf440ali5Ry&scm=20140722.H_2712515._.OR_help-T_cn~zh-V_1

	// Model available models: text_embedding_v / text_embedding_v2 / text_embedding_v3
	// Async embedding models not support.
	Model string `json:"model"`
	// Dimensions specify output vector dimension.
	// Only applicable to text-embedding-v3 model, can only be selected between three values: 1024, 768, and 512.
	// The default value is 1024.
	Dimensions *int `json:"dimensions,omitempty"`
}
type Embedder struct {
	cli *openai.Client
}

func NewEmbedder(ctx context.Context, config *EmbeddingConfig) (*Embedder, error) {
	encodingFmt := openai.EmbeddingEncodingFormatFloat // only support float currently

	var httpClient *http.Client

	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	ecfg := &openai.Config{
		BaseURL:        baseUrl,
		APIKey:         config.APIKey,
		HTTPClient:     httpClient,
		Model:          config.Model,
		EncodingFormat: &encodingFmt,
		Dimensions:     config.Dimensions,
	}

	if ecfg.Dimensions == nil {
		dim := dimensions
		ecfg.Dimensions = &dim
	}

	cli, err := openai.NewClient(ctx, ecfg)
	if err != nil {
		return nil, err
	}

	return &Embedder{cli: cli}, nil
}

func (e *Embedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return e.cli.EmbedStrings(ctx, texts, opts...)
}

const typ = "DashScope"

func (e *Embedder) GetType() string {
	return typ
}

func (e *Embedder) IsCallbacksEnabled() bool {
	return true
}
