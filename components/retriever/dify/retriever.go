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

package dify

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// RetrieverConfig 定义了 Dify Retriever 的配置参数
type RetrieverConfig struct {
	// APIKey 是 Dify API 的认证密钥
	APIKey string
	// Endpoint 是 Dify API 的服务地址, 默认为: https://api.dify.ai/v1
	Endpoint string
	// DatasetID 是知识库的唯一标识
	DatasetID string
	// RetrievalModel 检索参数 选填，如不填，按照默认方式召回
	RetrievalModel *RetrievalModel
	// Timeout 定义了 HTTP 连接超时时间
	Timeout time.Duration
}

type Retriever struct {
	config *RetrieverConfig
	client *http.Client
}

func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if config.DatasetID == "" {
		return nil, fmt.Errorf("dataset_id is required")
	}

	if config.RetrievalModel != nil && config.RetrievalModel.SearchMethod == "" {
		return nil, fmt.Errorf("if retrieval_model is set, search_method is required")
	}

	if config.Endpoint == "" {
		config.Endpoint = defaultEndpoint
	}
	httpClient := &http.Client{}
	if config.Timeout != 0 {
		httpClient.Timeout = config.Timeout
	}
	return &Retriever{
		config: config,
		client: httpClient,
	}, nil
}

// Retrieve 根据查询文本检索相关文档
func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	// 设置回调和错误处理
	defer func() {
		if err != nil {
			ctx = callbacks.OnError(ctx, err)
		}
	}()

	// 合并检索选项
	baseOptions := &retriever.Options{}
	if r.config.RetrievalModel != nil {
		baseOptions.TopK = r.config.RetrievalModel.TopK
		baseOptions.ScoreThreshold = r.config.RetrievalModel.ScoreThreshold
	}
	options := retriever.GetCommonOptions(baseOptions, opts...)

	// 开始检索回调
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           dereferenceOrZero(options.TopK),
		ScoreThreshold: options.ScoreThreshold,
	})

	// 发送检索请求
	result, err := r.doPost(ctx, query, options)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve documents: %w", err)
	}
	// 转换为统一的 Document 格式
	docs = make([]*schema.Document, 0, len(result.Records))

	for _, record := range result.Records {
		if record == nil || record.Segment == nil {
			continue
		}
		if options.ScoreThreshold != nil && record.Score < *options.ScoreThreshold {
			continue
		}
		doc := record.toDoc()
		docs = append(docs, doc)
	}

	// 结束检索回调
	ctx = callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})

	return docs, nil
}

func (r *Retriever) GetType() string {
	return typ
}

func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}
