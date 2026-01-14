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
	ClientConfig *milvusclient.ClientConfig

	// Collection is the collection name in Milvus.
	// Default: "eino_collection"
	Collection string

	// Partitions to search. Empty means search all partitions.
	Partitions []string

	// VectorField is the name of the vector field in the collection.
	// Default: "vector"
	VectorField string

	// SparseVectorField is the field name for sparse vectors.
	// Default: "sparse_vector"
	SparseVectorField string

	// OutputFields specifies which fields to return in search results.
	// Default: all fields
	OutputFields []string

	// TopK is the number of results to return.
	// Default: 5
	TopK int

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

	cli, err := initClient(ctx, conf)
	if err != nil {
		return nil, err
	}

	if err := loadCollection(ctx, cli, conf); err != nil {
		return nil, err
	}

	return &Retriever{
		client: cli,
		config: conf,
	}, nil
}

func initClient(ctx context.Context, conf *RetrieverConfig) (*milvusclient.Client, error) {
	if conf.Client != nil {
		return conf.Client, nil
	}

	if conf.ClientConfig == nil {
		return nil, fmt.Errorf("[NewRetriever] either Client or ClientConfig must be provided")
	}

	cli, err := milvusclient.New(ctx, conf.ClientConfig)
	if err != nil {
		return nil, fmt.Errorf("[NewRetriever] failed to create milvus client: %w", err)
	}

	return cli, nil
}

func loadCollection(ctx context.Context, cli *milvusclient.Client, conf *RetrieverConfig) error {
	hasCollection, err := cli.HasCollection(ctx, milvusclient.NewHasCollectionOption(conf.Collection))
	if err != nil {
		return fmt.Errorf("[NewRetriever] failed to check collection: %w", err)
	}
	if !hasCollection {
		return fmt.Errorf("[NewRetriever] collection %q not found", conf.Collection)
	}

	loadState, err := cli.GetLoadState(ctx, milvusclient.NewGetLoadStateOption(conf.Collection))
	if err != nil {
		return fmt.Errorf("[NewRetriever] failed to get load state: %w", err)
	}
	if loadState.State != entity.LoadStateLoaded {
		loadTask, err := cli.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(conf.Collection))
		if err != nil {
			return fmt.Errorf("[NewRetriever] failed to load collection: %w", err)
		}
		if err := loadTask.Await(ctx); err != nil {
			return fmt.Errorf("[NewRetriever] failed to await collection load: %w", err)
		}
	}
	return nil
}

// Retrieve searches for documents matching the given query.
// It returns the matching documents or an error.
func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query: query,
		TopK:  r.config.TopK,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	docs, err = r.config.SearchMode.Retrieve(ctx, r.client, r.config, query, opts...)
	if err != nil {
		return nil, err
	}

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})
	return docs, nil
}

// EmbedQuery embeds the query string into a vector.
// It is exposed for SearchMode implementations to use.
func EmbedQuery(ctx context.Context, emb embedding.Embedder, query string) ([]float32, error) {
	if emb == nil {
		return nil, fmt.Errorf("[Retriever] embedding not provided")
	}

	// We can't access r.makeEmbeddingCtx here easily without passing r.
	// For now, we assume the SearchMode implementation handles the context if needed,
	// or we pass a context that already has the run info if possible.
	// However, EmbedStrings usually expects just a context.
	// Let's check how makeEmbeddingCtx was used. It adds ComponentOfEmbedding callback info.
	// TODO: To fully support callbacks for embedding inside SearchMode, we might need a helper.
	// For now, let's keep it simple.

	vectors, err := emb.EmbedStrings(ctx, []string{query})
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

// applyScoreThreshold filters documents below the score threshold.
func (r *Retriever) applyScoreThreshold(docs []*schema.Document, threshold *float64) []*schema.Document {
	if threshold == nil {
		return docs
	}

	filtered := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc.Score() >= *threshold {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// QueryResultSetToDocuments converts a Query result set to documents.
func QueryResultSetToDocuments(resultSet milvusclient.ResultSet) ([]*schema.Document, error) {
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
	// Embedding validation is delegated to the specific SearchMode implementation.
	if c.Collection == "" {
		c.Collection = defaultCollection
	}
	if c.VectorField == "" {
		c.VectorField = defaultVectorField
	}
	if c.SparseVectorField == "" {
		c.SparseVectorField = defaultSparseVectorField
	}
	if len(c.OutputFields) == 0 {
		c.OutputFields = []string{"*"}
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
