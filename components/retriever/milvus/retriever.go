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

	"github.com/cloudwego/eino-ext/components/retriever/milvus/search_mode"
)

// SearchMode is an alias for search_mode.SearchMode for convenience.
// Use search_mode.SearchModeHNSW, SearchModeIvfFlat, SearchModeAuto, or SearchModeFlat
// to create a SearchMode instance.
type SearchMode = search_mode.SearchMode

// RetrieverConfig provides configuration options for creating a Milvus Retriever.
type RetrieverConfig struct {
	// Client is the Milvus client used for database operations.
	// This field is required and must be a milvus-sdk-go client of version 2.4.x.
	Client client.Client

	// Collection specifies the collection name in the Milvus database.
	// Optional; defaults to "eino_collection".
	Collection string

	// Partition specifies the partitions to search within.
	// Optional; defaults to an empty slice (search all partitions).
	Partition []string

	// VectorField specifies the name of the vector field in the collection.
	// Optional; defaults to "vector".
	VectorField string

	// OutputFields specifies which fields to include in search results.
	// Optional; defaults to an empty slice (only return IDs and distances).
	OutputFields []string

	// DocumentConverter transforms Milvus search results into schema.Document instances.
	// Optional; defaults to a converter compatible with the built-in schema.
	DocumentConverter func(ctx context.Context, doc client.SearchResult) ([]*schema.Document, error)

	// VectorConverter transforms embedding vectors into Milvus entity.Vector format.
	// Optional; defaults to a BinaryVector converter.
	VectorConverter func(ctx context.Context, vectors [][]float64) ([]entity.Vector, error)

	// TopK specifies the maximum number of results to return.
	// Optional; defaults to 5.
	TopK int

	// ScoreThreshold filters results below this similarity score.
	// Optional; defaults to 0 (no filtering).
	ScoreThreshold float64

	// SearchMode defines the search strategy and parameters for different index types.
	// Use search_mode.SearchModeHNSW, SearchModeIvfFlat, SearchModeAuto, or SearchModeFlat.
	// Optional; defaults to AUTOINDEX with COSINE metric.
	// When SearchMode is set, MetricType and Sp fields are ignored.
	SearchMode SearchMode

	// Deprecated: MetricType is deprecated; set the metric type in SearchMode instead.
	MetricType entity.MetricType

	// Deprecated: Sp is deprecated; use SearchMode instead to configure search parameters.
	Sp entity.SearchParam

	// Embedding provides the embedding model for vectorizing query strings.
	// This field is required.
	Embedding embedding.Embedder
}

// Retriever performs similarity search queries against a Milvus collection.
// It implements the eino retriever.Retriever interface.
type Retriever struct {
	config RetrieverConfig
}

// NewRetriever creates a new Milvus Retriever with the given configuration.
// It validates the configuration, verifies the collection exists and its schema
// matches the expected fields, and loads the collection if needed.
// NewRetriever returns an error if the client is not ready, the collection
// does not exist, or required fields are missing from the configuration.
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

	// If SearchMode is not set, use legacy Sp field or create default
	if config.SearchMode == nil {
		if config.Sp == nil {
			dim, err := getCollectionDim(config.VectorField, collection.Schema)
			if err != nil {
				return nil, fmt.Errorf("[NewRetriever] failed to get collection dim: %w", err)
			}
			config.Sp = defaultSearchParam(config.ScoreThreshold, dim)
		}

		// get the score threshold from legacy Sp
		scoreThreshold, ok := config.Sp.Params()["range_filter"]
		if !ok {
			config.ScoreThreshold = 0
		} else {
			config.ScoreThreshold = scoreThreshold.(float64)
		}
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
			TopK:              config.TopK,
			ScoreThreshold:    config.ScoreThreshold,
			SearchMode:        config.SearchMode,
			MetricType:        config.MetricType,
			Sp:                config.Sp,
			Embedding:         config.Embedding,
		},
	}, nil
}

// Retrieve performs a similarity search for the given query string.
// It embeds the query using the configured Embedding model, searches the
// Milvus collection using the configured SearchMode, and returns matching documents.
// Retrieve returns an error if embedding fails, no results are found, or search fails.
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

	// Build search parameters from SearchMode or use legacy Sp
	var sp entity.SearchParam
	var metricType entity.MetricType

	if r.config.SearchMode != nil {
		// Use new SearchMode interface
		sp, err = r.config.SearchMode.BuildSearchParam(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("[milvus retriever] failed to build search param: %w", err)
		}
		metricType = r.config.SearchMode.MetricType()
	} else {
		// Fallback to legacy Sp and MetricType for backward compatibility
		sp = r.config.Sp
		metricType = r.config.MetricType
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
		metricType,
		*co.TopK,
		sp,
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

// GetType returns the type identifier for this retriever ("Milvus").
func (r *Retriever) GetType() string {
	return typ
}

// IsCallbacksEnabled indicates whether this retriever supports callbacks.
// It always returns true for Milvus retriever.
func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

// check validates the configuration and applies default values for optional fields.
// It returns an error if required fields (Client, Embedding) are missing.
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
