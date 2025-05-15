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
	"time"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/tencent/vectordatabase-sdk-go/tcvectordb"
)

type RetrieverConfig struct {

	// Basic Configs
	URL        string        `json:"url"`        // Required The url of tencent vector db server
	Username   string        `json:"username"`   // Required username
	Key        string        `json:"key"`        // Required secret key
	Database   string        `json:"database"`   // Required The database you are going to connect
	Collection string        `json:"collection"` // Required The collection you are going to use
	Timeout    time.Duration `json:"timeout"`    // Optional Default 5 seconds

	// Retrieve configs
	// Required TopK is the top k for the retriever, which means the top number of documents to retrieve.
	TopK int `json:"top_k"`

	// Required ScoreThreshold is the score threshold for the retriever, eg 0.5 means the score of the document must be greater than 0.5.
	ScoreThreshold *float64 `json:"score_threshold"`

	// Optional Index is the index for the retriever, index in different retriever may be different.
	Index string `json:"index"`

	// Optional Filter DSL, used for filtering retrieved results
	// see: https://cloud.tencent.com/document/product/1709/102655
	FilterDSL *tcvectordb.Filter `json:"filter_dsl"`

	CollectionConfig CollectionConfig `json:"collection_config"`

	// Embedding configs
	EmbeddingConfig EmbeddingConfig `json:"embedding_config"`
}

// CollectionConfig Configs for collections in Tencent vector db
type CollectionConfig struct {

	// Required ShardNum is the number of shards of collection
	// range: [1,100]
	ShardNum uint32 `json:"shard_num"`

	// Required ReplicaNum is the number of replicas of the collection
	// range: [2,9]
	ReplicaNum uint32 `json:"replica_num"`

	// Required Index options see: https://cloud.tencent.com/document/product/1709/111856
	Indexes tcvectordb.Indexes `json:"indexes"`

	// Optional Description is the description of the collection
	// length: [1,256]
	Description string `json:"description"`

	// Optional Params: holds the parameters for creating a new collection
	Params *tcvectordb.CreateCollectionParams `json:"params,omitempty"`
}

type EmbeddingConfig struct {
	// Required UseBuiltin determines whether to use the builtin embedding function of tc vector db or not
	// see: https://cloud.tencent.com/document/product/1709/102641#eb1fb0d9-15fa-4315-8386-7fe78bf82652
	UseBuiltin bool `json:"use_builtin"`

	// Required if UseBuiltin is false
	// If Embedding from here is provided, it will take precedence over built-in vectorization methods
	Embedding embedding.Embedder
}

type Retriever struct {

	// RPC client of tc vector db
	client *tcvectordb.RpcClient

	// collection of vector db
	collection *tcvectordb.Collection

	// config configs
	config *RetrieverConfig
}

func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
	if config.EmbeddingConfig.UseBuiltin && config.EmbeddingConfig.Embedding != nil {
		return nil, fmt.Errorf("[TcVectorDBRetriever] no need to provide Embedding when UseBuiltin embedding is true")
	} else if !config.EmbeddingConfig.UseBuiltin && config.EmbeddingConfig.Embedding == nil {
		return nil, fmt.Errorf("[TcVectorDBRetriever] need provide Embedding when UseBuiltin embedding is false")
	}

	client, err := tcvectordb.NewRpcClient(config.URL, config.Username, config.Key, &tcvectordb.ClientOption{
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
	if !r.config.EmbeddingConfig.UseBuiltin {
		// In case not using the builtin embedding function of Tencent Vector DB
		vectors, err := opts.Embedding.EmbedStrings(ctx, []string{query})
		if err != nil {
			return nil, fmt.Errorf("[TcVectorDBRetriever] embed query failed: %v", err)
		}

		// 确保vectors[0]不为空
		if len(vectors) == 0 || len(vectors[0]) == 0 {
			return nil, fmt.Errorf("[TcVectorDBRetriever] embed query returned empty vectors")
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

	if searchResults == nil || searchResults.Documents == nil {
		return nil, fmt.Errorf("[TcVectorDBRetriever] search returned nil result")
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
