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

package redis

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

type RetrieverConfig struct {
	// Client is a Redis client representing a pool of zero or more underlying connections.
	// It's safe for concurrent use by multiple goroutines, which means is okay to pass
	// an existed Client to create a new Retriever component.
	// ***NOTICE***: two client conn options should be configured correctly to enable ft.search.
	// 1. Protocol: default is 3, here needs 2, or FT.Search result is raw.
	// 2. UnstableResp3: default is false, here needs true.
	Client *redis.Client
	// Index name of index to search.
	// see: https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#create-a-vector-index
	Index string
	// VectorField vector field name in search query, correspond to FieldValue.EmbedKey from redis indexer.
	// Default "vector_content"
	VectorField string
	// DistanceThreshold controls how to build search query.
	// If DistanceThreshold is set, use vector range search.
	// If DistanceThreshold is not set, use KNN vector search.
	// Default is nil.
	// Vector Range Queries: https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#vector-range-queries
	// KNN Vector Search: https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#knn-vector-search
	DistanceThreshold *float64
	// Dialect default 2.
	// see: https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/dialects/
	Dialect int
	// ReturnFields limits the attributes returned from the document. num is the number of attributes following the keyword.
	// Default []string{"content", "vector_content"}
	ReturnFields []string
	// DocumentConverter converts retrieved raw document to eino Document, default defaultResultParser.
	DocumentConverter func(ctx context.Context, doc redis.Document) (*schema.Document, error)
	// TopK limits number of results given, default 5.
	TopK int
	// Embedding vectorization method for query.
	Embedding embedding.Embedder
}

type Retriever struct {
	config *RetrieverConfig
}

func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
	if config.Embedding == nil {
		return nil, fmt.Errorf("[NewRetriever] embedding not provided for redis retriever")
	}

	if config.Index == "" {
		return nil, fmt.Errorf("[NewRetriever] redis index not provided")
	}

	if config.Client == nil {
		return nil, fmt.Errorf("[NewRetriever] redis client not provided")
	}

	if config.Dialect < 2 {
		// Support for vector search also was introduced in the 2.4
		config.Dialect = 2
	}

	if config.TopK == 0 {
		config.TopK = 5
	}

	if config.VectorField == "" {
		config.VectorField = defaultReturnFieldVectorContent
	}

	if len(config.ReturnFields) == 0 {
		config.ReturnFields = []string{
			defaultReturnFieldContent,
			defaultReturnFieldVectorContent,
		}
	}

	if config.DocumentConverter == nil {
		config.DocumentConverter = defaultResultParser(config.ReturnFields)
	}

	return &Retriever{
		config: config,
	}, nil
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          &r.config.Index,
		TopK:           &r.config.TopK,
		ScoreThreshold: r.config.DistanceThreshold,
		Embedding:      r.config.Embedding,
	}, opts...)
	io := retriever.GetImplSpecificOptions(&implOptions{}, opts...)

	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *co.TopK,
		Filter:         io.FilterQuery,
		ScoreThreshold: co.ScoreThreshold,
	})

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[redis retriever] embedding not provided")
	}

	vectors, err := emb.EmbedStrings(r.makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, err
	}

	if len(vectors) != 1 {
		return nil, fmt.Errorf("[redis retriever] invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	params := map[string]any{
		paramVector: vector2Bytes(vectors[0]),
	}

	var searchQuery string
	if r.config.DistanceThreshold != nil {
		params[paramDistanceThreshold] = dereferenceOrZero(r.config.DistanceThreshold)
		baseQuery := fmt.Sprintf("@%s:[VECTOR_RANGE $%s $%s]", r.config.VectorField, paramDistanceThreshold, paramVector)

		if io.FilterQuery != "" {
			baseQuery = io.FilterQuery + " " + baseQuery
		}

		searchQuery = fmt.Sprintf("%s=>{$yield_distance_as: %s}", baseQuery, SortByDistanceAttributeName)
	} else {
		filter := "*"
		if io.FilterQuery != "" {
			filter = io.FilterQuery
		}

		searchQuery = fmt.Sprintf("(%s)=>[KNN %d @%s $%s AS %s]",
			filter, *co.TopK, r.config.VectorField, paramVector, SortByDistanceAttributeName)
	}

	sr := make([]redis.FTSearchReturn, 0, len(r.config.ReturnFields))
	for _, field := range r.config.ReturnFields {
		sr = append(sr, redis.FTSearchReturn{FieldName: field})
	}

	searchOptions := &redis.FTSearchOptions{
		Return:         sr,
		SortBy:         []redis.FTSearchSortBy{{FieldName: SortByDistanceAttributeName, Asc: true}},
		Limit:          *co.TopK,
		DialectVersion: r.config.Dialect,
		Params:         params,
		WithScores:     false,
	}

	cmd := r.config.Client.FTSearchWithArgs(ctx, *co.Index, searchQuery, searchOptions)
	result, err := cmd.Result() // here required RESP protocol=2
	if err != nil {
		return nil, err
	}

	for _, raw := range result.Docs {
		doc, err := r.config.DocumentConverter(ctx, raw)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})

	return docs, nil

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

const typ = "Redis"

func (r *Retriever) GetType() string {
	return typ
}

func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

func defaultResultParser(returnFields []string) func(ctx context.Context, doc redis.Document) (*schema.Document, error) {
	return func(ctx context.Context, doc redis.Document) (*schema.Document, error) {
		resp := &schema.Document{
			ID:       doc.ID,
			Content:  "",
			MetaData: map[string]any{},
		}

		for _, field := range returnFields {
			val, found := doc.Fields[field]
			if !found {
				return nil, fmt.Errorf("[defaultResultParser] field=%s not found in doc, doc=%v", field, doc)
			}

			if field == defaultReturnFieldContent {
				resp.Content = val
			} else if field == defaultReturnFieldVectorContent {
				resp.WithDenseVector(Bytes2Vector([]byte(val)))
			} else {
				resp.MetaData[field] = val
			}
		}

		return resp, nil
	}
}
