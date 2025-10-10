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
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
)

type Config struct {
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
}

type Retriever struct {
	client         *qdrant.Client
	collection     string
	embedding      embedding.Embedder
	scoreThreshold *float64
	topK           int
}

func NewRetriever(ctx context.Context, config *Config) (*Retriever, error) {
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

	topK := config.TopK
	if topK == 0 {
		topK = 5
	}

	return &Retriever{
		client:         config.Client,
		collection:     config.Collection,
		embedding:      config.Embedding,
		scoreThreshold: config.ScoreThreshold,
		topK:           topK,
	}, nil
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK:           &r.topK,
		ScoreThreshold: r.scoreThreshold,
		Embedding:      r.embedding,
	}, opts...)
	io := retriever.GetImplSpecificOptions(&implOptions{}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *co.TopK,
		Filter:         tryMarshalJsonString(io.Filter),
		ScoreThreshold: co.ScoreThreshold,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[qdrant retriever] embedding not provided")
	}
	vectors, err := emb.EmbedStrings(r.makeEmbeddingCtx(ctx, emb), []string{query})
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
		CollectionName: r.collection,
		Query:          qdrant.NewQueryDense(vec32),
		Limit:          qdrant.PtrOf(uint64(*co.TopK)),
		WithPayload:    qdrant.NewWithPayload(true),
	}
	if r.scoreThreshold != nil {
		searchReq.ScoreThreshold = qdrant.PtrOf(float32(*r.scoreThreshold))
	}
	if io.Filter != nil {
		searchReq.Filter = io.Filter
	}

	resp, err := r.client.Query(ctx, &searchReq)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] qdrant search failed: %w", err)
	}
	docs = make([]*schema.Document, 0, len(resp))
	for _, pt := range resp {
		doc := &schema.Document{
			ID:       pt.Id.GetUuid(),
			MetaData: map[string]any{},
		}

		if val, ok := pt.Payload[defaultContentKey]; ok {
			doc.Content = val.GetStringValue()
		}

		if val, ok := pt.Payload[defaultMetadataKey]; ok {
			doc.MetaData[defaultMetadataKey] = val.GetStructValue().Fields
		}

		doc.WithScore(float64(pt.Score))

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

func tryMarshalJsonString(input any) string {
	if input == nil {
		return ""
	}
	if b, err := json.Marshal(input); err == nil {
		return string(b)
	}
	return ""
}

const typ = "Qdrant"

func (r *Retriever) GetType() string {
	return typ
}

func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

var _ retriever.Retriever = &Retriever{}
