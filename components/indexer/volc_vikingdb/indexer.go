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

	"github.com/volcengine/volc-sdk-golang/service/vikingdb"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultAddBatchSize = 5
)

type IndexerConfig struct {
	Host              string `json:"host"`
	Region            string `json:"region"`
	AK                string `json:"ak"`
	SK                string `json:"sk"`
	Scheme            string `json:"scheme"`
	ConnectionTimeout int64  `json:"connection_timeout"` // second

	Collection string `json:"collection"`

	// WithMultiModal 如果数据集在平台向量化，需要配置此字段为true，无需再配置EmbeddingConfig
	WithMultiModal  bool            `json:"with_multi_modal"`
	EmbeddingConfig EmbeddingConfig `json:"embedding_config"`

	AddBatchSize int `json:"add_batch_size"`
}

type EmbeddingConfig struct {
	// UseBuiltin 是否使用 VikingDB 内置向量化方法 (embedding v2)
	// true 时需要配置 ModelName 和 UseSparse, false 时需要配置 Embedding
	// see: https://www.volcengine.com/docs/84313/1254617
	UseBuiltin bool `json:"use_builtin"`

	// ModelName 指定模型名称
	ModelName string `json:"model_name"`
	// UseSparse 是否返回稀疏向量
	// 支持提取稀疏向量的模型设置为 true 返回稠密+稀疏向量，设置为 false 仅返回稠密向量
	// 不支持稀疏向量的模型设置为 true 会报错
	UseSparse bool `json:"use_sparse"`

	// Embedding when UseBuiltin is false
	// If Embedding from here or from indexer.Option is provided, it will take precedence over built-in vectorization methods
	Embedding embedding.Embedder
}

type Indexer struct {
	config     *IndexerConfig
	service    *vikingdb.VikingDBService
	collection *vikingdb.Collection
	embModel   *vikingdb.EmbModel
}

func NewIndexer(ctx context.Context, config *IndexerConfig) (*Indexer, error) {
	if !config.WithMultiModal {
		if config.EmbeddingConfig.UseBuiltin && config.EmbeddingConfig.Embedding != nil {
			return nil, fmt.Errorf("[VikingDBIndexer] no need to provide Embedding when UseBuiltin embedding is true")
		} else if !config.EmbeddingConfig.UseBuiltin && config.EmbeddingConfig.Embedding == nil {
			return nil, fmt.Errorf("[VikingDBIndexer] need provide Embedding when UseBuiltin embedding is false")
		}
	}

	if config.AddBatchSize == 0 {
		config.AddBatchSize = defaultAddBatchSize
	}

	service := vikingdb.NewVikingDBService(config.Host, config.Region, config.AK, config.SK, config.Scheme)
	if config.ConnectionTimeout != 0 {
		service.SetConnectionTimeout(config.ConnectionTimeout)
	}

	collection, err := service.GetCollection(config.Collection)
	if err != nil {
		return nil, err
	}

	i := &Indexer{
		config:     config,
		service:    service,
		collection: collection,
		embModel:   nil,
	}

	if config.EmbeddingConfig.UseBuiltin {
		i.embModel = &vikingdb.EmbModel{
			ModelName: config.EmbeddingConfig.ModelName,
			Params: map[string]interface{}{
				vikingEmbeddingUseDense:  true,
				vikingEmbeddingUseSparse: config.EmbeddingConfig.UseSparse,
			},
		}
	}

	return i, nil
}

func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {

	options := indexer.GetCommonOptions(&indexer.Options{
		Embedding: i.config.EmbeddingConfig.Embedding,
	}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{Docs: docs})
	defer func() {
		if err != nil {
			ctx = callbacks.OnError(ctx, err)
		}
	}()

	ids = make([]string, 0, len(docs))
	for _, sub := range chunk(docs, i.config.AddBatchSize) {
		data, err := i.convertDocuments(ctx, sub, options)
		if err != nil {
			return nil, fmt.Errorf("convertDocuments failed: %w", err)
		}

		if err = i.collection.UpsertData(data); err != nil {
			return nil, fmt.Errorf("UpsertData failed: %w", err)
		}

		ids = append(ids, iter(sub, func(t *schema.Document) string { return t.ID })...)
	}

	ctx = callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})

	return ids, nil
}

func (i *Indexer) convertDocuments(ctx context.Context, docs []*schema.Document, options *indexer.Options) (data []vikingdb.Data, err error) {
	var (
		useBuiltinEmbedding = i.config.EmbeddingConfig.UseBuiltin && options.Embedding == nil

		dense  [][]float64
		sparse []map[string]interface{}
	)

	queries := iter(docs, func(doc *schema.Document) string {
		return doc.Content
	})

	if !i.config.WithMultiModal {
		if useBuiltinEmbedding {
			dense, sparse, err = i.builtinEmbedding(ctx, queries, options)
		} else {
			dense, err = i.customEmbedding(ctx, queries, options)
		}
		if err != nil {
			return nil, err
		}
	}

	data = make([]vikingdb.Data, len(docs))
	for idx := range docs {
		doc := docs[idx]
		d := vikingdb.Data{}

		if fields, ok := GetExtraVikingDBFields(doc); ok {
			d.Fields = fields
		}

		if ttl, ok := GetExtraVikingDBTTL(doc); ok {
			d.TTL = ttl
		}

		if d.Fields == nil {
			d.Fields = make(map[string]interface{})
		}

		d.Fields[defaultFieldID] = doc.ID
		d.Fields[defaultFieldContent] = doc.Content
		if !i.config.WithMultiModal {
			d.Fields[defaultFieldVector] = dense[idx]
			if len(sparse) != 0 {
				d.Fields[defaultFieldSparseVector] = sparse[idx]
			}
		}

		data[idx] = d
	}

	return data, nil
}

func (i *Indexer) builtinEmbedding(ctx context.Context, queries []string, options *indexer.Options) (
	dense [][]float64, sparse []map[string]interface{}, err error) {

	rawData := iter(queries, func(query string) vikingdb.RawData {
		return vikingdb.RawData{
			DataType: vikingdb.Text,
			Text:     query,
		}
	})

	items, err := i.service.EmbeddingV2(*i.embModel, rawData)
	if err != nil {
		return nil, nil, err
	}

	if rawDense, ok := items[vikingEmbeddingRespSentenceDense].([]interface{}); ok && len(rawDense) == len(queries) {
		dense, err = iterWithErr(rawDense, interfaceTof64Slice)
		if err != nil {
			return nil, nil, fmt.Errorf("[builtinEmbedding] conv dense embedding item failed, err=%w, data=%v", err, rawDense)
		}
	} else {
		return nil, nil, fmt.Errorf("[builtinEmbedding] parse dense embedding from result failed, data=%v, len=%v, exp len=%v",
			items, len(rawDense), len(queries))
	}

	if i.config.EmbeddingConfig.UseSparse {
		if rawSparse, ok := items[vikingEmbeddingRespSentenceSparse].([]interface{}); ok && len(rawSparse) == len(queries) {
			sparse, err = iterWithErr(rawSparse, interfaceToSparse)
			if err != nil {
				return nil, nil, fmt.Errorf("[builtinEmbedding] conv sparse embedding item failed, err=%w", err)
			}
		} else {
			return nil, nil, fmt.Errorf("[builtinEmbedding] parse sparse embedding from result failed, data=%v, len=%v, exp len=%v",
				items, len(rawSparse), len(queries))
		}
	}

	return dense, sparse, nil
}

func (i *Indexer) customEmbedding(ctx context.Context, queries []string, options *indexer.Options) (vector [][]float64, err error) {
	emb := options.Embedding
	vectors, err := emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), queries)
	if err != nil {
		return nil, err
	}

	if len(vectors) != len(queries) {
		return nil, fmt.Errorf("[customEmbedding] invalid return length of vector, got=%d, expected=%d", len(vectors), len(queries))
	}

	return vectors, nil
}

func (i *Indexer) makeEmbeddingCtx(ctx context.Context, emb embedding.Embedder) context.Context {
	runInfo := &callbacks.RunInfo{
		Component: components.ComponentOfEmbedding,
	}

	if embType, ok := components.GetType(emb); ok {
		runInfo.Type = embType
	}

	runInfo.Name = runInfo.Type + string(runInfo.Component)

	return callbacks.ReuseHandlers(ctx, runInfo)
}

func (i *Indexer) GetType() string {
	return typ
}

func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}
