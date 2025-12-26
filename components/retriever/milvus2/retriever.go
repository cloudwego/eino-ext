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

package milvus2

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// RetrieverConfig contains configuration for the Milvus2 retriever.
type RetrieverConfig struct {
	// Client is an optional pre-configured Milvus client.
	// If not provided, the component will create one using ClientConfig.
	Client *milvusclient.Client

	// ClientConfig for creating Milvus client if Client is not provided.
	// Supports both standard Milvus (Address) and Zilliz Cloud (URI + APIKey).
	ClientConfig *milvusclient.ClientConfig

	// Collection is the collection name in Milvus.
	// Default: "eino_collection"
	Collection string

	// Partitions to search. Empty means search all partitions.
	Partitions []string

	// VectorField is the name of the vector field in the collection.
	// Default: "vector"
	VectorField string

	// OutputFields specifies which fields to return in search results.
	// Default: all fields
	OutputFields []string

	// TopK is the number of results to return.
	// Default: 5
	TopK int

	// ScoreThreshold filters results with scores below this threshold.
	ScoreThreshold *float64

	// ConsistencyLevel for Milvus operations.
	// Default: ConsistencyLevelBounded
	ConsistencyLevel ConsistencyLevel

	// SearchMode defines the search strategy.
	// Required.
	SearchMode SearchMode

	// DocumentConverter converts Milvus search results to EINO documents.
	// If nil, uses default conversion.
	DocumentConverter func(ctx context.Context, result milvusclient.ResultSet) ([]*schema.Document, error)

	// Embedding is the embedder for query vectorization.
	// Optional. Required if SearchMode uses vector search.
	Embedding embedding.Embedder

	// SparseEmbedding is the embedder for sparse query vectorization.
	// Optional. Use this if your search mode involves sparse vectors.
	SparseEmbedding SparseEmbedder
}

// Retriever implements the retriever.Retriever interface for Milvus 2.x using the V2 SDK.
type Retriever struct {
	client *milvusclient.Client
	config *RetrieverConfig
}

// NewRetriever creates a new Milvus2 retriever with the provided configuration.
// It returns an error if the configuration is invalid.
func NewRetriever(ctx context.Context, conf *RetrieverConfig) (*Retriever, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}

	cli := conf.Client
	if cli == nil {
		if conf.ClientConfig == nil {
			return nil, fmt.Errorf("[NewRetriever] either Client or ClientConfig must be provided")
		}
		var err error
		cli, err = milvusclient.New(ctx, conf.ClientConfig)
		if err != nil {
			return nil, fmt.Errorf("[NewRetriever] failed to create milvus client: %w", err)
		}
	}

	hasCollection, err := cli.HasCollection(ctx, milvusclient.NewHasCollectionOption(conf.Collection))
	if err != nil {
		return nil, fmt.Errorf("[NewRetriever] failed to check collection: %w", err)
	}
	if !hasCollection {
		return nil, fmt.Errorf("[NewRetriever] collection %q not found", conf.Collection)
	}

	loadState, err := cli.GetLoadState(ctx, milvusclient.NewGetLoadStateOption(conf.Collection))
	if err != nil {
		return nil, fmt.Errorf("[NewRetriever] failed to get load state: %w", err)
	}
	if loadState.State != entity.LoadStateLoaded {
		loadTask, err := cli.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(conf.Collection))
		if err != nil {
			return nil, fmt.Errorf("[NewRetriever] failed to load collection: %w", err)
		}
		if err := loadTask.Await(ctx); err != nil {
			return nil, fmt.Errorf("[NewRetriever] failed to await collection load: %w", err)
		}
	}

	return &Retriever{
		client: cli,
		config: conf,
	}, nil
}

// Retrieve searches for documents matching the given query.
// It returns the matching documents or an error.
func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          &r.config.VectorField,
		TopK:           &r.config.TopK,
		ScoreThreshold: r.config.ScoreThreshold,
		Embedding:      r.config.Embedding,
	}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *co.TopK,
		ScoreThreshold: co.ScoreThreshold,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	switch sm := r.config.SearchMode.(type) {
	case QuerySearchMode:
		docs, err = r.retrieveQuery(ctx, sm, query, opts...)
	case HybridSearchMode:
		docs, err = r.retrieveHybrid(ctx, sm, co, query, opts...)
	case IteratorSearchMode:
		docs, err = r.retrieveIterator(ctx, sm, co, query, opts...)
	default:
		docs, err = r.retrieveStandard(ctx, co, query, opts...)
	}

	if err != nil {
		return nil, err
	}

	docs = r.applyScoreThreshold(docs, co.ScoreThreshold)

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})
	return docs, nil
}

// retrieveQuery handles QuerySearchMode (scalar/metadata-only search).
func (r *Retriever) retrieveQuery(ctx context.Context, mode QuerySearchMode, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	queryOpt, err := mode.BuildQueryOption(ctx, r.config, query, opts...)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] failed to build query option: %w", err)
	}

	resultSet, err := r.client.Query(ctx, queryOpt)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] failed to execute query: %w", err)
	}

	return r.queryResultSetToDocuments(resultSet)
}

// retrieveHybrid handles HybridSearchMode.
func (r *Retriever) retrieveHybrid(ctx context.Context, mode HybridSearchMode, co *retriever.Options, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	var queryVector []float32
	if r.config.Embedding != nil {
		var err error
		queryVector, err = r.embedQuery(ctx, co.Embedding, query)
		if err != nil {
			return nil, err
		}
	}

	var querySparseVector map[int]float64
	if r.config.SparseEmbedding != nil {
		var err error
		querySparseVector, err = r.embedSparseQuery(ctx, r.config.SparseEmbedding, query)
		if err != nil {
			return nil, err
		}
	}

	searchOpt, err := mode.BuildHybridSearchOption(ctx, r.config, queryVector, querySparseVector, opts...)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] failed to build hybrid search option: %w", err)
	}

	results, err := r.client.HybridSearch(ctx, searchOpt)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] hybrid search failed: %w", err)
	}
	if len(results) == 0 {
		return []*schema.Document{}, nil
	}

	return r.config.DocumentConverter(ctx, results[0])
}

// retrieveIterator handles IteratorSearchMode.
func (r *Retriever) retrieveIterator(ctx context.Context, mode IteratorSearchMode, co *retriever.Options, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	queryVector, err := r.embedQuery(ctx, co.Embedding, query)
	if err != nil {
		return nil, err
	}

	iterOpt, err := mode.BuildSearchIteratorOption(ctx, r.config, queryVector, opts...)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] failed to build search iterator option: %w", err)
	}

	iterator, err := r.client.SearchIterator(ctx, iterOpt)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] failed to create search iterator: %w", err)
	}

	var allDocs []*schema.Document
	for {
		res, err := iterator.Next(ctx)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("[Retriever] iterator next failed: %w", err)
		}
		if res.ResultCount == 0 {
			break
		}

		batchDocs, err := r.config.DocumentConverter(ctx, res)
		if err != nil {
			return nil, fmt.Errorf("[Retriever] failed to convert batch results: %w", err)
		}
		allDocs = append(allDocs, batchDocs...)
	}

	return allDocs, nil
}

// retrieveStandard handles standard SearchMode (Approximate, Range, etc.).
func (r *Retriever) retrieveStandard(ctx context.Context, co *retriever.Options, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	queryVector, err := r.embedQuery(ctx, co.Embedding, query)
	if err != nil {
		return nil, err
	}

	searchOpt, err := r.config.SearchMode.BuildSearchOption(ctx, r.config, queryVector, opts...)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] failed to build search option: %w", err)
	}

	results, err := r.client.Search(ctx, searchOpt)
	if err != nil {
		return nil, fmt.Errorf("[Retriever] search failed: %w", err)
	}
	if len(results) == 0 {
		return []*schema.Document{}, nil
	}

	return r.config.DocumentConverter(ctx, results[0])
}

// embedQuery embeds the query string into a vector.
func (r *Retriever) embedQuery(ctx context.Context, emb embedding.Embedder, query string) ([]float32, error) {
	if emb == nil {
		return nil, fmt.Errorf("[Retriever] embedding not provided")
	}

	vectors, err := emb.EmbedStrings(r.makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[Retriever] failed to embed query: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("[Retriever] invalid embedding result: expected 1, got %d", len(vectors))
	}

	queryVector := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		queryVector[i] = float32(v)
	}
	return queryVector, nil
}

// embedSparseQuery embeds the query string into a sparse vector.
func (r *Retriever) embedSparseQuery(ctx context.Context, emb SparseEmbedder, query string) (map[int]float64, error) {
	if emb == nil {
		return nil, fmt.Errorf("[Retriever] sparse embedding not provided")
	}

	vectors, err := emb.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("[Retriever] failed to embed sparse query: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("[Retriever] invalid sparse embedding result: expected 1, got %d", len(vectors))
	}

	return vectors[0], nil
}

// applyScoreThreshold filters documents below the score threshold.
func (r *Retriever) applyScoreThreshold(docs []*schema.Document, threshold *float64) []*schema.Document {
	if threshold == nil {
		return docs
	}

	filtered := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if score, ok := doc.MetaData["score"].(float64); ok && score >= *threshold {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// queryResultSetToDocuments converts a Query result set to documents.
func (r *Retriever) queryResultSetToDocuments(resultSet milvusclient.ResultSet) ([]*schema.Document, error) {
	docs := make([]*schema.Document, 0, resultSet.ResultCount)

	getField := func(fieldName string, idx int) (any, bool) {
		col := resultSet.GetColumn(fieldName)
		if col == nil {
			return nil, false
		}
		val, err := col.Get(idx)
		return val, err == nil
	}

	for i := 0; i < resultSet.ResultCount; i++ {
		idVal, ok := getField(defaultIDField, i)
		if !ok {
			continue
		}
		idStr := fmt.Sprintf("%v", idVal)

		contentVal, _ := getField(defaultContentField, i)
		contentStr := ""
		if contentVal != nil {
			contentStr = fmt.Sprintf("%v", contentVal)
		}

		meta := make(map[string]any)
		if metaVal, ok := getField(defaultMetadataField, i); ok {
			if fieldBytes, ok := metaVal.([]byte); ok {
				var m map[string]any
				if err := sonic.Unmarshal(fieldBytes, &m); err == nil {
					for k, v := range m {
						meta[k] = v
					}
				}
			}
		}

		docs = append(docs, &schema.Document{
			ID:       idStr,
			Content:  contentStr,
			MetaData: meta,
		})
	}

	return docs, nil
}

// GetType returns the type of the retriever.
func (r *Retriever) GetType() string {
	return typ
}

// IsCallbacksEnabled checks if callbacks are enabled for this retriever.
func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

// validate checks the configuration and sets default values.
func (c *RetrieverConfig) validate() error {
	if c.Client == nil && c.ClientConfig == nil {
		return fmt.Errorf("[NewRetriever] milvus client or client config not provided")
	}
	if c.SearchMode == nil {
		return fmt.Errorf("[NewRetriever] search mode not provided")
	}
	if c.Embedding == nil {
		// Embedding is required unless the search mode is QuerySearchMode (scalar filtering only).
		if _, ok := c.SearchMode.(QuerySearchMode); !ok {
			return fmt.Errorf("[NewRetriever] embedding not provided; it is required for vector search modes. " +
				"Please provide an Embedding or use QuerySearchMode for metadata-only filtering")
		}
	}
	if c.Collection == "" {
		c.Collection = defaultCollection
	}
	if c.VectorField == "" {
		c.VectorField = defaultVectorField
	}
	if c.TopK <= 0 {
		c.TopK = defaultTopK
	}
	if c.DocumentConverter == nil {
		c.DocumentConverter = defaultDocumentConverter()
	}
	return nil
}

// defaultDocumentConverter returns the default result to document converter.
func defaultDocumentConverter() func(ctx context.Context, result milvusclient.ResultSet) ([]*schema.Document, error) {
	return func(ctx context.Context, result milvusclient.ResultSet) ([]*schema.Document, error) {
		docs := make([]*schema.Document, 0, result.ResultCount)

		for i := 0; i < result.ResultCount; i++ {
			doc := &schema.Document{
				MetaData: make(map[string]any),
			}

			if i < len(result.Scores) {
				doc.MetaData["score"] = float64(result.Scores[i])
				doc = doc.WithScore(float64(result.Scores[i]))
			}

			for _, field := range result.Fields {
				val, err := field.Get(i)
				if err != nil {
					continue
				}

				switch field.Name() {
				case "id":
					if id, ok := val.(string); ok {
						doc.ID = id
					} else if idStr, err := field.GetAsString(i); err == nil {
						doc.ID = idStr
					}
				case defaultContentField:
					if content, ok := val.(string); ok {
						doc.Content = content
					} else if contentStr, err := field.GetAsString(i); err == nil {
						doc.Content = contentStr
					}
				case defaultMetadataField:
					if metaBytes, ok := val.([]byte); ok {
						var meta map[string]any
						if err := sonic.Unmarshal(metaBytes, &meta); err == nil {
							for k, v := range meta {
								doc.MetaData[k] = v
							}
						}
					}
				default:
					doc.MetaData[field.Name()] = val
				}
			}

			docs = append(docs, doc)
		}

		return docs, nil
	}
}

// makeEmbeddingCtx creates a context with embedding callback information.
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
