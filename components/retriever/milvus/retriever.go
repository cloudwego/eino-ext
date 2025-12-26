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

// Package milvus provides a Retriever implementation using Milvus vector database.
//
// Deprecated: This package uses milvus-sdk-go v2.4.x which only supports Milvus 2.4.
// Use github.com/cloudwego/eino-ext/components/retriever/milvus2 instead.
package milvus

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type RetrieverConfig struct {
	// Client is the milvus client to be called
	// It requires the milvus-sdk-go client of version 2.4.x
	// Required
	Client client.Client

	// Default Retriever config
	// Collection is the collection name in the milvus database
	// Optional, and the default value is "eino_collection"
	Collection string
	// Partition is the collection partition name
	// Optional, and the default value is empty
	Partition []string
	// VectorField is the vector field name in the collection
	// Optional, and the default value is "vector"
	VectorField string
	// OutputFields is the fields to be returned
	// Optional, and the default value is empty
	OutputFields []string
	// DocumentConverter is the function to convert the search result to schema.Document
	// Optional, and the default value is defaultDocumentConverter
	DocumentConverter func(ctx context.Context, doc client.SearchResult) ([]*schema.Document, error)
	// VectorConverter is the function to convert the vectors to entity.Vector
	VectorConverter func(ctx context.Context, vectors [][]float64) ([]entity.Vector, error)
	// MetricType is the metric type for vector
	// Optional, and the default value is "HAMMING"
	MetricType entity.MetricType
	// TopK is the top k results to be returned
	// Optional, and the default value is 5
	TopK int
	// ScoreThreshold is the threshold for the search result
	// Optional, and the default value is 0
	ScoreThreshold float64
	// SearchParams
	// Optional, and the default value is entity.IndexAUTOINDEXSearchParam, and the level is 1
	Sp entity.SearchParam

	// Embedding is the embedding vectorization method for values needs to be embedded from schema.Document's content.
	// Required
	Embedding embedding.Embedder
}

type Retriever struct {
	config RetrieverConfig
}

func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
	if err := config.check(); err != nil {
		return nil, err
	}

	// pre-check for the milvus search config
	// check the collection is existed
	ok, err := config.Client.HasCollection(ctx, config.Collection)
	if err != nil {
		if errors.Is(err, client.ErrClientNotReady) {
			return nil, fmt.Errorf("[NewRetriever] milvus client not ready: %w", err)
		}
		if errors.Is(err, client.ErrStatusNil) {
			return nil, fmt.Errorf("[NewRetriever] milvus client response status has error: %w", err)
		}
		return nil, fmt.Errorf("[NewRetriever] failed to check collection: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("[NewRetriever] collection not found")
	}

	// load collection info
	collection, err := config.Client.DescribeCollection(ctx, config.Collection)
	if err != nil {
		return nil, fmt.Errorf("[NewRetriever] failed to describe collection: %w", err)
	}
	// check collection schema
	if err := checkCollectionSchema(config.VectorField, collection.Schema); err != nil {
		return nil, fmt.Errorf("[NewRetriever] collection schema not match: %w", err)
	}

	// check the collection load state
	if !collection.Loaded {
		// load collection
		if err := loadCollection(ctx, config); err != nil {
			return nil, fmt.Errorf("[NewRetriever] failed to load collection: %w", err)
		}
	}

	if config.Sp == nil {
		dim, err := getCollectionDim(config.VectorField, collection.Schema)
		if err != nil {
			return nil, fmt.Errorf("[NewRetriever] failed to get collection dim: %w", err)
		}
		config.Sp = defaultSearchParam(config.ScoreThreshold, dim)
	}

	// get the score threshold
	scoreThreshold, ok := config.Sp.Params()["range_filter"]
	if !ok {
		config.ScoreThreshold = 0
	} else {
		config.ScoreThreshold = scoreThreshold.(float64)
	}

	// build the retriever
	return &Retriever{
		config: RetrieverConfig{
			Client:            config.Client,
			Collection:        config.Collection,
			Partition:         config.Partition,
			VectorField:       config.VectorField,
			OutputFields:      config.OutputFields,
			DocumentConverter: config.DocumentConverter,
			VectorConverter:   config.VectorConverter,
			MetricType:        config.MetricType,
			TopK:              config.TopK,
			ScoreThreshold:    config.ScoreThreshold,
			Sp:                config.Sp,
			Embedding:         config.Embedding,
		},
	}, nil
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	// get common options
	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          &r.config.VectorField,
		TopK:           &r.config.TopK,
		ScoreThreshold: &r.config.ScoreThreshold,
		Embedding:      r.config.Embedding,
	}, opts...)
	// get impl specific options
	io := retriever.GetImplSpecificOptions(&ImplOptions{}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	// callback info on start
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *co.TopK,
		Filter:         io.Filter,
		ScoreThreshold: co.ScoreThreshold,
		Extra: map[string]any{
			"metric_type": r.config.MetricType,
		},
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	// get the embedding vector
	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[milvus retriever] embedding not provided")
	}

	// embedding the query
	vectors, err := emb.EmbedStrings(r.makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[milvus retriever] embedding has error: %w", err)
	}
	// check the embedding result
	if len(vectors) != 1 {
		return nil, fmt.Errorf("[milvus retriever] invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	// convert the embedding result to entity.Vector
	vec, err := r.config.VectorConverter(ctx, vectors)
	if err != nil {
		return nil, fmt.Errorf("[milvus retriever] failed to convert vector: %w", err)
	}

	// search the collection
	var results []client.SearchResult
	var searchParams []client.SearchQueryOptionFunc
	if io.SearchQueryOptFn != nil {
		searchParams = append(searchParams, io.SearchQueryOptFn)
	}

	results, err = r.config.Client.Search(
		ctx,
		r.config.Collection,
		r.config.Partition,
		io.Filter,
		r.config.OutputFields,
		vec,
		r.config.VectorField,
		r.config.MetricType,
		*co.TopK,
		r.config.Sp,
		searchParams...,
	)
	if err != nil {
		return nil, fmt.Errorf("[milvus retriever] search has error: %w", err)
	}
	// check the search result
	if len(results) == 0 {
		return nil, fmt.Errorf("[milvus retriever] no results found")
	}

	// convert the search result to schema.Document
	documents := make([]*schema.Document, 0, len(results))
	for _, result := range results {
		if result.Err != nil {
			return nil, fmt.Errorf("[milvus retriever] search result has error: %w", result.Err)
		}
		if result.IDs == nil || result.Fields == nil {
			return nil, fmt.Errorf("[milvus retriever] search result has no ids or fields")
		}
		document, err := r.config.DocumentConverter(ctx, result)
		if err != nil {
			return nil, fmt.Errorf("[milvus retriever] failed to convert search result to schema.Document: %w", err)
		}
		documents = append(documents, document...)
	}

	// callback info on end
	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: documents})

	return documents, nil
}

func (r *Retriever) GetType() string {
	return typ
}

func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

// check the retriever config and set the default value
func (r *RetrieverConfig) check() error {
	if r.Client == nil {
		return fmt.Errorf("[NewRetriever] milvus client not provided")
	}
	if r.Embedding == nil {
		return fmt.Errorf("[NewRetriever] embedding not provided")
	}
	if r.Sp == nil && r.ScoreThreshold < 0 {
		return fmt.Errorf("[NewRetriever] invalid search params")
	}
	if r.Collection == "" {
		r.Collection = defaultCollection
	}
	if r.Partition == nil {
		r.Partition = []string{}
	}
	if r.VectorField == "" {
		r.VectorField = defaultVectorField
	}
	if r.OutputFields == nil {
		r.OutputFields = []string{}
	}
	if r.DocumentConverter == nil {
		r.DocumentConverter = defaultDocumentConverter()
	}
	if r.VectorConverter == nil {
		r.VectorConverter = defaultVectorConverter()
	}
	if r.TopK == 0 {
		r.TopK = defaultTopK
	}
	if r.MetricType == "" {
		r.MetricType = defaultMetricType
	}
	return nil
}
