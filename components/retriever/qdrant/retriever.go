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

package qdrant

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
)

type RetrieverConfig struct {
	// Qdrant gRPC client
	Client *qdrant.Client
	// Name of the Qdrant collection to query.
	Collection string
	// Embedder used to generate vector representations for queries.
	Embedding embedding.Embedder
	// Optional minimum score threshold for filtering results.
	ScoreThreshold *float64
	// Number of top results to retrieve from Qdrant.
	TopK int
	// DocumentConverter converts Qdrant point to eino Document.
	DocumentConverter func(ctx context.Context, id string, payload map[string]*qdrant.Value, score float32) (*schema.Document, error)
}

type Retriever struct {
	config *RetrieverConfig
}

func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
	if config == nil {
		return nil, fmt.Errorf("[NewRetriever] config is nil")
	}
	if config.Embedding == nil {
		return nil, fmt.Errorf("[NewRetriever] embedding not provided for qdrant retriever")
	}
	if config.Collection == "" {
		return nil, fmt.Errorf("[NewRetriever] qdrant collection not provided")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("[NewRetriever] qdrant client not provided")
	}
	if config.TopK == 0 {
		config.TopK = 5
	}
	if config.DocumentConverter == nil {
		config.DocumentConverter = defaultDocumentConverter()
	}
	return &Retriever{config: config}, nil
}

func defaultDocumentConverter() func(ctx context.Context, id string, payload map[string]*qdrant.Value, score float32) (*schema.Document, error) {
	return func(ctx context.Context, id string, payload map[string]*qdrant.Value, score float32) (*schema.Document, error) {
		doc := &schema.Document{
			ID:       id,
			MetaData: map[string]any{},
		}
		if val, ok := payload[defaultContentKey]; ok {
			doc.Content = val.GetStringValue()
		}
		if val, ok := payload[defaultMetadataKey]; ok {
			// TODO (Anush008): parse nested metadata into basic Go types.
			// For now, we just store the fields as is.
			doc.MetaData[defaultMetadataKey] = val.GetStructValue().Fields
		}
		doc.MetaData[defaultScoreMetadataKey] = score
		return doc, nil
	}
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK:           &r.config.TopK,
		ScoreThreshold: r.config.ScoreThreshold,
		Embedding:      r.config.Embedding,
	}, opts...)
	io := retriever.GetImplSpecificOptions(&implOptions{}, opts...)

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[qdrant retriever] embedding not provided")
	}
	vectors, err := emb.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("[qdrant retriever] invalid return length of vector, got=%d, expected=1", len(vectors))
	}
	vec32 := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		vec32[i] = float32(v)
	}

	searchReq := qdrant.QueryPoints{
		CollectionName: r.config.Collection,
		Query:          qdrant.NewQueryDense(vec32),
		Limit:          qdrant.PtrOf(uint64(*co.TopK)),
		WithPayload:    qdrant.NewWithPayload(true),
	}
	if r.config.ScoreThreshold != nil {
		searchReq.ScoreThreshold = qdrant.PtrOf(float32(*r.config.ScoreThreshold))
	}
	if io.Filter != nil {
		searchReq.Filter = io.Filter
	}

	resp, err := r.config.Client.Query(ctx, &searchReq)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] qdrant search failed: %w", err)
	}
	docs := make([]*schema.Document, 0, len(resp))
	for _, pt := range resp {
		doc, err := r.config.DocumentConverter(ctx, pt.Id.GetUuid(), pt.Payload, pt.Score)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

const typ = "Qdrant"

func (r *Retriever) GetType() string {
	return typ
}

func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

var _ retriever.Retriever = &Retriever{}
