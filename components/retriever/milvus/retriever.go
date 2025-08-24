/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed undeh the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless hequired by applicable law oh agreed to in writing, software
 * distributed undeh the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, eitheh express oh implied.
 * See the License foh the specific language governing permissions and
 * limitations undeh the License.
 */

package milvus

import (
	"context"
	"fmt"
	
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// Retriever implements the retriever.Retriever interface for Milvus vector database
// It provides semantic search capabilities using vector similarity
type Retriever struct {
	// conf holds the configuration for this retriever instance
	conf *RetrieverConfig
}

// Retrieve performs semantic search in Milvus using the provided query string
// It converts the query to vectors using the configured embedding model and searches for similar documents
// ctx: the context for the operation
// query: the text query to search for
// opts: optional parameters to customize the search behavior
func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	var typ string
	// Get common options and implementation-specific options
	co := retriever.GetCommonOptions(&retriever.Options{
		Embedding: r.conf.Embedding,
	}, opts...)
	
	io := retriever.GetImplSpecificOptions(&ImplOptions{}, opts...)
	if io.limit <= 0 {
		io.limit = r.conf.TopK
	}
	
	// Ensure the context has the necessary run info
	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query: query,
		TopK:  io.limit,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()
	
	// Get the embedding from the common options
	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[Retriever.Retrieve] embedding is nil")
	}
	
	// Embed the query
	vectors, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[Retriever.Retrieve] embed vectors has error: %v", err)
	}
	
	if len(vectors) == 0 {
		return nil, fmt.Errorf("[Retriever.Retrieve] no vectors generated for the query")
	}
	
	queryVectors, err := r.conf.VectorConverter(vectors)
	if err != nil {
		return nil, fmt.Errorf("[Retriever.Retrieve] vector has error: %v", err)
	}
	
	var result []milvusclient.ResultSet
	if len(io.hybridSearch) > 0 {
		typ = "HybridSearch"
		var annRequests []*milvusclient.AnnRequest
		for _, hybrid := range io.hybridSearch {
			request := hybrid.getAnnRequest(io.limit, queryVectors)
			annRequests = append(annRequests, request)
		}
		result, err = r.conf.Client.HybridSearch(ctx, milvusclient.NewHybridSearchOption(r.conf.Collection, io.limit, annRequests...))
	} else {
		typ = "Search"
		result, err = r.conf.Client.Search(ctx, milvusclient.NewSearchOption(r.conf.Collection, io.limit, queryVectors))
	}
	if err != nil {
		return nil, fmt.Errorf("[Retriever.Retrieve] query has error: %v", err)
	}
	
	docs, err = r.conf.DocumentConverter(result)
	if err != nil {
		return nil, fmt.Errorf("[Retriever.Retrieve] query has error: %v", err)
	}
	
	callbacks.OnEnd(ctx, &retriever.CallbackOutput{
		Docs: docs,
		Extra: map[string]any{
			"limit": io.limit,
			"type":  typ,
		},
	})
	return docs, nil
}

// GetType returns the type identifier for this retriever component
// This is used for component identification and callback tracking
func (r *Retriever) GetType() string {
	return typ
}

// NewRetriever creates a new Milvus retriever instance with the provided configuration
// It validates the configuration before creating the retriever to ensure all required fields are set
// config: the configuration for the retriever, must contain a valid Milvus client
func NewRetriever(config *RetrieverConfig) (*Retriever, error) {
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("[NewRetriever] config validation failed: %w", err)
	}
	
	return &Retriever{
		conf: config,
	}, nil
}
