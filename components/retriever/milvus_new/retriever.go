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

package milvus_new

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

type RetrieverConfig struct {
	// Client is the milvus client to be called
	// It uses the new milvus/client/v2/milvusclient
	// Required
	Client milvusclient.Client

	// Default Retriever config
	// Collection is the collection name in the milvus database
	// Optional, and the default value is "eino_collection"
	Collection string
	// Partition is the collection partition name
	// Optional, and the default value is empty
	Partition string
	// VectorField is the vector field name in the collection
	// Optional, and the default value is "vector"
	VectorField string
	// OutputFields is the fields to be returned
	// Optional, and the default value is all fields except vector
	OutputFields []string
	// DocumentConverter is the function to convert the search result to schema.Document
	// Optional, and the default value is defaultDocumentConverter
	DocumentConverter func(ctx context.Context, columns []column.Column) ([]*schema.Document, error)
	// VectorConverter is the function to convert the vectors to binary vector bytes
	// Optional, and the default value is defaultVectorConverter
	VectorConverter func(ctx context.Context, vectors [][]float64) ([][]byte, error)
	// MetricType is the metric type for vector
	// Optional, and the default value is "HAMMING"
	MetricType MetricType
	// TopK is the top k results to be returned
	// Optional, and the default value is 5
	TopK int
	// ScoreThreshold is the threshold for the search result
	// Optional, and the default value is 0
	ScoreThreshold float64

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
	hasCollectionOpt := milvusclient.NewHasCollectionOption(config.Collection)
	ok, err := config.Client.HasCollection(ctx, hasCollectionOpt)
	if err != nil {
		return nil, fmt.Errorf("[NewRetriever] failed to check collection: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("[NewRetriever] collection not found")
	}

	// load collection info
	descCollOpt := milvusclient.NewDescribeCollectionOption(config.Collection)
	collection, err := config.Client.DescribeCollection(ctx, descCollOpt)
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
		if err := loadCollection(ctx, config.Client, config.Collection); err != nil {
			return nil, fmt.Errorf("[NewRetriever] failed to load collection: %w", err)
		}
	}

	// build the retriever
	return &Retriever{
		config: *config,
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
	vectors, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[milvus retriever] embedding has error: %w", err)
	}
	// check the embedding result
	if len(vectors) != 1 {
		return nil, fmt.Errorf("[milvus retriever] invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	// convert the embedding result to binary vector bytes
	vecBytes, err := r.config.VectorConverter(ctx, vectors)
	if err != nil {
		return nil, fmt.Errorf("[milvus retriever] failed to convert vector: %w", err)
	}

	// Convert [][]byte to []entity.Vector
	entityVectors := make([]entity.Vector, len(vecBytes))
	for i, vb := range vecBytes {
		entityVectors[i] = entity.BinaryVector(vb)
	}

	// prepare partition
	partitions := []string{}
	if r.config.Partition != "" {
		partitions = append(partitions, r.config.Partition)
	}
	if io.Partition != "" {
		// Override with runtime partition if provided
		partitions = []string{io.Partition}
	}

	// prepare search options
	var searchOpt milvusclient.SearchOption = milvusclient.NewSearchOption(r.config.Collection, *co.TopK, entityVectors).
		WithANNSField(r.config.VectorField).
		WithOutputFields(r.config.OutputFields...).
		WithConsistencyLevel(entity.ClBounded)

	if len(partitions) > 0 {
		searchOpt = searchOpt.(interface {
			WithPartitions(partitions ...string) milvusclient.SearchOption
		}).WithPartitions(partitions...)
	}

	if io.Filter != "" {
		searchOpt = searchOpt.(interface {
			WithFilter(expr string) milvusclient.SearchOption
		}).WithFilter(io.Filter)
	}

	// Add score threshold if provided
	if co.ScoreThreshold != nil && *co.ScoreThreshold > 0 {
		// Note: Milvus 2.6 uses range filter for score threshold
		searchOpt = searchOpt.(interface {
			WithFilter(expr string) milvusclient.SearchOption
		}).WithFilter(fmt.Sprintf("score >= %f", *co.ScoreThreshold))
	}

	// Apply custom search options if provided
	if io.SearchOptFn != nil {
		searchOpt = io.SearchOptFn(searchOpt)
	}

	// search the collection
	results, err := r.config.Client.Search(ctx, searchOpt)
	if err != nil {
		return nil, fmt.Errorf("[milvus retriever] search has error: %w", err)
	}

	// check the search result
	if len(results) == 0 {
		return []*schema.Document{}, nil
	}

	// convert the search result to schema.Document
	documents, err := r.config.DocumentConverter(ctx, results[0].Fields)
	if err != nil {
		return nil, fmt.Errorf("[milvus retriever] failed to convert search result to schema.Document: %w", err)
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
	// Check if Client is nil using reflection since it's an interface
	if reflect.ValueOf(r.Client).IsNil() {
		return fmt.Errorf("[NewRetriever] milvus client not provided")
	}
	if r.Embedding == nil {
		return fmt.Errorf("[NewRetriever] embedding not provided")
	}
	if r.Collection == "" {
		r.Collection = defaultCollection
	}
	if r.VectorField == "" {
		r.VectorField = defaultVectorField
	}
	if r.OutputFields == nil {
		r.OutputFields = []string{"id", "content", "metadata"}
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
		r.MetricType = MetricType(defaultMetricType)
	}
	return nil
}
