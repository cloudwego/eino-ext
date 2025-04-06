/*
 * Copyright 2025 CloudWeGo Authors
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

package tcvectordb

import (
	"context"
	"fmt"
	"github.com/cloudwego/eino/components/embedding"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/tencent/vectordatabase-sdk-go/tcvectordb"
)

type RetrieverConfig struct {

	// Basic Configs
	Url        string        `json:"url"`
	Username   string        `json:"username"`
	Key        string        `json:"key"`
	Database   string        `json:"database"`
	Collection string        `json:"collection"`
	Timeout    time.Duration `json:"timeout"`

	// Retrieve configs
	TopK           int      `json:"top_k"`
	ScoreThreshold *float64 `json:"score_threshold"`
	Index          string   `json:"index"`

	// Filter DSL, used for filtering retrieved results
	// see: https://cloud.tencent.com/document/product/1709/102655
	FilterDSL *tcvectordb.Filter `json:"filter_dsl"`

	CollectionConfig CollectionConfig `json:"collection_config"`

	// Embedding 配置
	EmbeddingConfig EmbeddingConfig `json:"embedding_config"`
}

type CollectionConfig struct {
	ShardNum uint32 `json:"shard_num"`

	ReplicaNum uint32 `json:"replica_num"`

	// length: [1,256]
	Description string `json:"description"`

	Indexes tcvectordb.Indexes `json:"indexes"`

	// Optional params
	Params *tcvectordb.CreateCollectionParams `json:"params,omitempty"`
}

type EmbeddingConfig struct {
	// UseBuiltin config
	// see: https://cloud.tencent.com/document/product/1709/102641#eb1fb0d9-15fa-4315-8386-7fe78bf82652
	UseBuiltin bool `json:"use_builtin"`

	// ModelName
	ModelName string `json:"model_name"`

	// UseSparse
	UseSparse bool `json:"use_sparse"`

	// Embedding when UseBuiltin is false
	// If Embedding from here is provided, it will take precedence over built-in vectorization methods
	Embedding embedding.Embedder
}

type Retriever struct {
	client     *tcvectordb.RpcClient
	collection *tcvectordb.Collection
	config     *RetrieverConfig
}

func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
	if config.EmbeddingConfig.UseBuiltin && config.EmbeddingConfig.Embedding != nil {
		return nil, fmt.Errorf("[TcVectorDBRetriever] no need to provide Embedding when UseBuiltin embedding is true")
	} else if !config.EmbeddingConfig.UseBuiltin && config.EmbeddingConfig.Embedding == nil {
		return nil, fmt.Errorf("[TcVectorDBRetriever] need provide Embedding when UseBuiltin embedding is false")
	}

	client, err := tcvectordb.NewRpcClient(config.Url, config.Username, config.Key, &tcvectordb.ClientOption{
		ReadConsistency: tcvectordb.EventualConsistency,
		Timeout:         config.Timeout,
	})

	if err != nil {
		return nil, fmt.Errorf("[TcVectorDBRetriever] create client failed: %w", err)
	}

	db, err := client.CreateDatabaseIfNotExists(ctx, config.Database)
	if err != nil {
		return nil, fmt.Errorf("[TcVectorDBRetriever] find or create db failed: %w", err)
	}
	collection, err := db.CreateCollectionIfNotExists(ctx, config.Collection, config.CollectionConfig.ShardNum, config.CollectionConfig.ReplicaNum, config.CollectionConfig.Description, config.CollectionConfig.Indexes, config.CollectionConfig.Params)
	if err != nil {
		return nil, fmt.Errorf("[TcVectorDBRetriever] find or create collection failed: %w", err)
	}

	return &Retriever{
		client:     client,
		collection: collection,
		config:     config,
	}, nil
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	defer func() {
		if err != nil {
			ctx = callbacks.OnError(ctx, err)
		}
	}()

	// 1. Handle options
	options := &retriever.Options{
		Index:          &r.config.Index,
		TopK:           &r.config.TopK,
		ScoreThreshold: r.config.ScoreThreshold,
		Embedding:      r.config.EmbeddingConfig.Embedding,
	}
	options = retriever.GetCommonOptions(options, opts...)

	// 2. Callbacks on start
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *options.TopK,
		ScoreThreshold: options.ScoreThreshold,
	})

	// 3. retrieve
	docs, err = r.doRetrieve(ctx, query, options)
	if err != nil {
		ctx = callbacks.OnError(ctx, err)
		return nil, err
	}

	// 4. callbacks on end
	ctx = callbacks.OnEnd(ctx, &retriever.CallbackOutput{
		Docs: docs,
	})

	return docs, nil
}

func (r *Retriever) doRetrieve(ctx context.Context, query string, opts *retriever.Options) ([]*schema.Document, error) {
	var queryVector []float32
	if r.config.EmbeddingConfig.UseBuiltin {
		// use builtin embedding of tc vector db
	} else {
		vectors, err := opts.Embedding.EmbedStrings(ctx, []string{query})
		if err != nil {
			return nil, fmt.Errorf("[TcVectorDBRetriever] embed query failed: %w", err)
		}
		// to float32
		queryVector = make([]float32, len(vectors[0]))
		for i, v := range vectors[0] {
			queryVector[i] = float32(v)
		}
	}

	searchParams := &tcvectordb.SearchDocumentParams{
		Limit:          int64(*opts.TopK),
		Filter:         r.config.FilterDSL,
		RetrieveVector: false,
	}

	searchResults, err := r.collection.Search(ctx, [][]float32{queryVector}, searchParams)
	if err != nil {
		return nil, fmt.Errorf("[TcVectorDBRetriever] search failed: %w", err)
	}

	var docs []*schema.Document
	if len(searchResults.Documents) > 0 {
		for _, doc := range searchResults.Documents[0] {

			if opts.ScoreThreshold != nil && float64(doc.Score) < *opts.ScoreThreshold {
				continue
			}

			document := &schema.Document{
				ID:      doc.Id,
				Content: doc.Fields[defaultFieldContent].String(),
			}

			// add metadata
			metadata := make(map[string]interface{})
			for k, v := range doc.Fields {
				if k != defaultFieldContent && k != defaultFieldVector && k != defaultFieldSparseVector {
					metadata[k] = v.Val
				}
			}
			document.MetaData = metadata

			docs = append(docs, document)
		}
	}

	return docs, nil
}
