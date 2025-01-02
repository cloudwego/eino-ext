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

package volc_vikingdb

import (
	"context"
	"fmt"
	"strconv"

	"github.com/volcengine/volc-sdk-golang/service/vikingdb"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultTopK        = 100
	defaultPartition   = "default"
	defaultDenseWeight = 0.5
)

type RetrieverConfig struct {
	Host              string `json:"host"`
	Region            string `json:"region"`
	AK                string `json:"ak"`
	SK                string `json:"sk"`
	Scheme            string `json:"scheme"`
	ConnectionTimeout int64  `json:"connection_timeout"` // second

	Collection string `json:"collection"`
	Index      string `json:"index"`

	EmbeddingConfig EmbeddingConfig `json:"embedding_config"`

	// Partition 子索引划分字段, 索引中未配置时至空即可
	Partition string `json:"partition"`
	// TopK will be set with 100 if zero
	TopK           *int     `json:"top_k,omitempty"`
	ScoreThreshold *float64 `json:"score_threshold,omitempty"`
	// FilterDSL 标量过滤 filter 表达式 https://www.volcengine.com/docs/84313/1254609
	FilterDSL map[string]any `json:"filter_dsl,omitempty"`
}

type EmbeddingConfig struct {
	// UseBuiltin 是否使用 VikingDB 内置向量化方法 (embedding v2)
	// true 时需要配置 ModelName 和 UseSparse, false 时需要配置 Embedding
	// see: https://www.volcengine.com/docs/84313/1254568
	UseBuiltin bool `json:"use_builtin"`

	// ModelName 指定模型名称
	ModelName string `json:"model_name"`
	// UseSparse 是否返回稀疏向量
	// 支持提取稀疏向量的模型设置为 true 返回稠密+稀疏向量，设置为 false 仅返回稠密向量
	// 不支持稀疏向量的模型设置为 true 会报错
	UseSparse bool `json:"use_sparse"`
	// DenseWeight 对于标量过滤检索，dense_weight 用于控制稠密向量在检索中的权重。范围为[0.2，1], 仅在检索的索引为混合索引时有效
	// 默认值为 0.5
	DenseWeight float64 `json:"dense_weight"`

	// Embedding 使用自行指定的 embedding 替换 VikingDB 内置向量化方法
	Embedding embedding.Embedder
}

type Retriever struct {
	config   *RetrieverConfig
	service  *vikingdb.VikingDBService
	index    *vikingdb.Index
	embModel *vikingdb.EmbModel
}

func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
	if config.EmbeddingConfig.UseBuiltin && config.EmbeddingConfig.Embedding != nil {
		return nil, fmt.Errorf("[VikingDBRetriever] no need to provide Embedding when UseBuiltin embedding is true")
	} else if !config.EmbeddingConfig.UseBuiltin && config.EmbeddingConfig.Embedding == nil {
		return nil, fmt.Errorf("[VikingDBRetriever] need provide Embedding when UseBuiltin embedding is false")
	}

	service := vikingdb.NewVikingDBService(config.Host, config.Region, config.AK, config.SK, config.Scheme)
	if config.ConnectionTimeout != 0 {
		service.SetConnectionTimeout(config.ConnectionTimeout)
	}

	index, err := service.GetIndex(config.Collection, config.Index)
	if err != nil {
		return nil, err
	}

	if len(config.Partition) == 0 {
		config.Partition = defaultPartition
	}

	if config.TopK == nil {
		config.TopK = ptrOf(defaultTopK)
	}

	r := &Retriever{
		config:   config,
		service:  service,
		index:    index,
		embModel: nil,
	}

	if config.EmbeddingConfig.UseBuiltin {
		if config.EmbeddingConfig.UseSparse && config.EmbeddingConfig.DenseWeight == 0 {
			config.EmbeddingConfig.DenseWeight = defaultDenseWeight
		}

		r.embModel = &vikingdb.EmbModel{
			ModelName: config.EmbeddingConfig.ModelName,
			Params: map[string]interface{}{
				vikingEmbeddingUseDense:  true,
				vikingEmbeddingUseSparse: config.EmbeddingConfig.UseSparse,
			},
		}
	}

	return r, nil
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	defer func() {
		if err != nil {
			ctx = callbacks.OnError(ctx, err)
		}
	}()

	options := retriever.GetCommonOptions(&retriever.Options{
		Index:          &r.config.Index,
		SubIndex:       &r.config.Partition,
		TopK:           r.config.TopK,
		ScoreThreshold: r.config.ScoreThreshold,
		Embedding:      r.config.EmbeddingConfig.Embedding,
		DSLInfo:        r.config.FilterDSL,
	}, opts...)

	var (
		dense  []float64
		sparse map[string]interface{}
	)

	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           dereferenceOrZero(options.TopK),
		Filter:         tryMarshalJsonString(options.DSLInfo),
		ScoreThreshold: options.ScoreThreshold,
	})

	if r.config.EmbeddingConfig.UseBuiltin && options.Embedding == nil {
		dense, sparse, err = r.builtinEmbedding(ctx, query, options)
	} else {
		dense, err = r.customEmbedding(ctx, query, options)
	}

	if err != nil {
		return nil, err
	}

	result, err := r.index.SearchByVector(dense, r.makeSearchOption(sparse, options))
	if err != nil {
		return nil, err
	}

	docs = make([]*schema.Document, 0, len(result))
	for _, data := range result {
		if options.ScoreThreshold != nil && data.Score < *options.ScoreThreshold {
			continue
		}

		doc, err := r.data2Document(data)
		if err != nil {
			return nil, err
		}

		docs = append(docs, doc.WithDSLInfo(options.DSLInfo))
	}

	ctx = callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})

	return docs, nil
}

func (r *Retriever) builtinEmbedding(ctx context.Context, query string, options *retriever.Options) (dense []float64, sparse map[string]interface{}, err error) {
	data := vikingdb.RawData{
		DataType: vikingdb.Text,
		Text:     query,
	}

	items, err := r.service.EmbeddingV2(*r.embModel, data)
	if err != nil {
		return nil, nil, err
	}

	if rawDense, ok := items[vikingEmbeddingRespSentenceDense].([]interface{}); ok && len(rawDense) > 0 {
		dense, err = interfaceTof64Slice(rawDense[0])
		if err != nil {
			return nil, nil, fmt.Errorf("[builtinEmbedding] parse dense embedding first item failed, value=%v", rawDense[0])
		}
	} else {
		return nil, nil, fmt.Errorf("[builtinEmbedding] parse dense embedding from result failed, data=%v", items)
	}

	if r.config.EmbeddingConfig.UseSparse {
		if rawSparse, ok := items[vikingEmbeddingRespSentenceSparse].([]interface{}); ok && len(rawSparse) > 0 {
			sparse, ok = rawSparse[0].(map[string]interface{})
			if !ok {
				return nil, nil, fmt.Errorf("[builtinEmbedding] parse dense embedding first item failed, value=%v", rawSparse[0])
			}
		} else {
			return nil, nil, fmt.Errorf("[builtinEmbedding] parse sparse embedding from result failed, data=%v", items)
		}
	}

	return dense, sparse, nil
}

func (r *Retriever) customEmbedding(ctx context.Context, query string, options *retriever.Options) (vector []float64, err error) {
	emb := options.Embedding
	vectors, err := emb.EmbedStrings(r.makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, err
	}

	if len(vectors) != 1 { // unexpected
		return nil, fmt.Errorf("[customEmbedding] invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	return vectors[0], nil
}

func (r *Retriever) makeSearchOption(sparse map[string]interface{}, options *retriever.Options) *vikingdb.SearchOptions {
	searchOptions := vikingdb.NewSearchOptions()
	if options.DSLInfo != nil {
		searchOptions.SetFilter(options.DSLInfo)
	}

	if options.SubIndex != nil {
		searchOptions.SetPartition(*options.SubIndex)
	}

	if sparse != nil {
		searchOptions.
			SetSparseVectors(sparse).
			SetDenseWeight(r.config.EmbeddingConfig.DenseWeight)
	}

	if topK := dereferenceOrZero(options.TopK); topK != 0 {
		searchOptions.SetLimit(int64(topK))
	}

	return searchOptions
}

func (r *Retriever) makeEmbeddingCtx(ctx context.Context, emb embedding.Embedder) context.Context {
	runInfo := &callbacks.RunInfo{
		Component: components.ComponentOfEmbedding,
	}

	if embType, ok := components.GetType(emb); ok {
		runInfo.Type = embType
	}

	runInfo.Name = runInfo.Type + string(runInfo.Component)

	return callbacks.ReuseHandlers(ctx, runInfo)
}

func (r *Retriever) data2Document(data *vikingdb.Data) (*schema.Document, error) {
	var id string

	if si, ok := data.Id.(string); ok {
		id = si
	} else if ii, ok := data.Id.(int); ok {
		id = strconv.FormatInt(int64(ii), 10)
	} else if ii, ok := data.Id.(int64); ok {
		id = strconv.FormatInt(ii, 10)
	}

	doc := &schema.Document{
		ID:       id,
		MetaData: map[string]any{},
	}

	if val, ok := data.Fields[defaultFieldContent].(string); ok {
		doc.Content = val
	} else {
		return nil, fmt.Errorf("data2Document: content field not found in collection")
	}

	doc.WithScore(data.Score)
	doc.MetaData[ExtraKeyVikingDBFields] = data.Fields
	doc.MetaData[ExtraKeyVikingDBTTL] = data.TTL

	return doc, nil
}

func (r *Retriever) GetType() string {
	return typ
}

func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}
