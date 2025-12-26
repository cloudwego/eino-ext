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
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// IndexerConfig contains configuration for the Milvus2 indexer.
type IndexerConfig struct {
	// Client is an optional pre-configured Milvus client.
	// If not provided, the component will create one using ClientConfig.
	Client *milvusclient.Client

	// ClientConfig for creating Milvus client if Client is not provided.
	// Supports both standard Milvus (Address) and Zilliz Cloud (URI + APIKey).
	ClientConfig *milvusclient.ClientConfig

	// Collection is the collection name in Milvus.
	// Default: "eino_collection"
	Collection string

	// Description is the description for the collection.
	// Default: "the collection for eino"
	Description string

	// Dimension is the vector dimension.
	// Required when auto-creating the collection.
	Dimension int64

	// PartitionName is the default partition for insertion.
	// Optional.
	PartitionName string

	// ConsistencyLevel for Milvus operations.
	// Default: ConsistencyLevelBounded
	ConsistencyLevel ConsistencyLevel

	// EnableDynamicSchema enables dynamic field support for flexible metadata.
	// Default: false
	EnableDynamicSchema bool

	// MetricType is the metric type for vector similarity.
	// Default: L2
	MetricType MetricType

	// IndexBuilder specifies how to build the vector index.
	// If nil, uses AutoIndex (Milvus automatically selects the best index).
	// Use NewHNSWIndexBuilder(), NewIVFFlatIndexBuilder(), etc. for specific index types.
	IndexBuilder IndexBuilder

	// SparseIndexBuilder specifies how to build the sparse vector index.
	// Optional. If nil and SparseVectorField is set, uses SPARSE_INVERTED_INDEX by default.
	SparseIndexBuilder IndexBuilder

	// VectorField is the name of the vector field in the collection.
	// Default: "vector"
	VectorField string

	// SparseVectorField is the name of the sparse vector field in the collection.
	// Optional. If provided, sparse vectors will be extracted and indexed.
	SparseVectorField string

	// DocumentConverter converts EINO documents to Milvus columns.
	// If nil, uses default conversion (id, content, vector, metadata as JSON).
	DocumentConverter func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error)

	// Embedding is the embedder for vectorization.
	// Required.
	Embedding embedding.Embedder
}

// Indexer implements the indexer.Indexer interface for Milvus 2.x using the V2 SDK.
type Indexer struct {
	client *milvusclient.Client
	config *IndexerConfig
}

// NewIndexer creates a new Milvus2 indexer with the provided configuration.
// It returns an error if the configuration is invalid.
func NewIndexer(ctx context.Context, conf *IndexerConfig) (*Indexer, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}

	cli := conf.Client
	if cli == nil {
		if conf.ClientConfig == nil {
			return nil, fmt.Errorf("[NewIndexer] either Client or ClientConfig must be provided")
		}
		var err error
		cli, err = milvusclient.New(ctx, conf.ClientConfig)
		if err != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to create milvus client: %w", err)
		}
	}

	hasCollection, err := cli.HasCollection(ctx, milvusclient.NewHasCollectionOption(conf.Collection))
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to check collection: %w", err)
	}

	if !hasCollection {
		if conf.Dimension <= 0 {
			return nil, fmt.Errorf("[NewIndexer] dimension is required when collection does not exist")
		}
		if err := createCollection(ctx, cli, conf); err != nil {
			return nil, err
		}
	}

	loadState, err := cli.GetLoadState(ctx, milvusclient.NewGetLoadStateOption(conf.Collection))
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to get load state: %w", err)
	}
	if loadState.State != entity.LoadStateLoaded {
		indexes, err := cli.ListIndexes(ctx, milvusclient.NewListIndexOption(conf.Collection))
		if err != nil {
			// "index not found" error means no indexes exist - this is expected for new collections
			if !isIndexNotFoundError(err) {
				return nil, fmt.Errorf("[NewIndexer] failed to list indexes: %w", err)
			}
			indexes = nil
		}
		if len(indexes) == 0 {
			if err := createIndex(ctx, cli, conf); err != nil {
				return nil, err
			}
		}

		loadTask, err := cli.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(conf.Collection))
		if err != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to load collection: %w", err)
		}
		if err := loadTask.Await(ctx); err != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to await collection load: %w", err)
		}
	}

	return &Indexer{
		client: cli,
		config: conf,
	}, nil
}

// Store adds the provided documents to the Milvus collection.
// It returns the list of IDs for the stored documents or an error.
func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	co := indexer.GetCommonOptions(&indexer.Options{
		Embedding: i.config.Embedding,
	}, opts...)
	io := indexer.GetImplSpecificOptions(&ImplOptions{}, opts...)
	if io.Partition == "" {
		io.Partition = i.config.PartitionName
	}

	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{
		Docs: docs,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	emb := co.Embedding

	var vectors [][]float64
	if emb != nil {
		texts := make([]string, 0, len(docs))
		for _, doc := range docs {
			texts = append(texts, doc.Content)
		}

		vectors, err = emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
		if err != nil {
			return nil, fmt.Errorf("[Indexer.Store] failed to embed documents: %w", err)
		}
		if len(vectors) != len(docs) {
			return nil, fmt.Errorf("[Indexer.Store] embedding result length mismatch: need %d, got %d", len(docs), len(vectors))
		}
	}

	columns, err := i.config.DocumentConverter(ctx, docs, vectors)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to convert documents: %w", err)
	}

	insertOpt := milvusclient.NewColumnBasedInsertOption(i.config.Collection)
	if io.Partition != "" {
		insertOpt = insertOpt.WithPartition(io.Partition)
	}
	for _, col := range columns {
		insertOpt = insertOpt.WithColumns(col)
	}

	result, err := i.client.Insert(ctx, insertOpt)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to insert documents: %w", err)
	}

	flushTask, err := i.client.Flush(ctx, milvusclient.NewFlushOption(i.config.Collection))
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to flush collection: %w", err)
	}
	if err := flushTask.Await(ctx); err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to await flush: %w", err)
	}

	ids = make([]string, 0, result.IDs.Len())
	for idx := 0; idx < result.IDs.Len(); idx++ {
		idStr, err := result.IDs.GetAsString(idx)
		if err != nil {
			return nil, fmt.Errorf("[Indexer.Store] failed to get id: %w", err)
		}
		ids = append(ids, idStr)
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{
		IDs: ids,
	})

	return ids, nil
}

// GetType returns the type of the indexer.
func (i *Indexer) GetType() string {
	return typ
}

// IsCallbacksEnabled checks if callbacks are enabled for this indexer.
func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

// validate checks the configuration and sets default values.
func (c *IndexerConfig) validate() error {
	if c.Client == nil && c.ClientConfig == nil {
		return fmt.Errorf("[NewIndexer] milvus client or client config not provided")
	}
	if c.Collection == "" {
		c.Collection = defaultCollection
	}
	if c.Description == "" {
		c.Description = defaultDescription
	}
	if c.ConsistencyLevel <= 0 || c.ConsistencyLevel > 5 {
		c.ConsistencyLevel = defaultConsistencyLevel
	}
	if c.MetricType == "" {
		c.MetricType = L2
	}
	// Default VectorField only if we have a dense dimension or no sparse field.
	// If Dimension is 0 and SparseVectorField is set, we assume sparse-only mode.
	if c.VectorField == "" && (c.Dimension > 0 || c.SparseVectorField == "") {
		c.VectorField = defaultVectorField
	}
	if c.DocumentConverter == nil {
		c.DocumentConverter = defaultDocumentConverter(c.VectorField, c.SparseVectorField)
	}
	return nil
}

// createCollection creates a new Milvus collection with the default schema.
func createCollection(ctx context.Context, cli *milvusclient.Client, conf *IndexerConfig) error {
	sch := entity.NewSchema().
		WithField(entity.NewField().
			WithName(defaultIDField).
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(defaultMaxIDLen).
			WithIsPrimaryKey(true)).
		WithField(entity.NewField().
			WithName(defaultContentField).
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(defaultMaxContentLen)).
		WithField(entity.NewField().
			WithName(defaultMetadataField).
			WithDataType(entity.FieldTypeJSON)).
		WithDynamicFieldEnabled(conf.EnableDynamicSchema)

	if conf.VectorField != "" {
		sch.WithField(entity.NewField().
			WithName(conf.VectorField).
			WithDataType(entity.FieldTypeFloatVector).
			WithDim(conf.Dimension))
	} else if conf.SparseVectorField == "" {
		// Should not happen if validation passed, but safety check: at least one vector field required
		return fmt.Errorf("[NewIndexer] at least one vector field (dense or sparse) is required")
	}

	if conf.SparseVectorField != "" {
		sch.WithField(entity.NewField().
			WithName(conf.SparseVectorField).
			WithDataType(entity.FieldTypeSparseVector))
	}

	createOpt := milvusclient.NewCreateCollectionOption(conf.Collection, sch).
		WithConsistencyLevel(conf.ConsistencyLevel.toEntity())

	if err := cli.CreateCollection(ctx, createOpt); err != nil {
		return fmt.Errorf("[NewIndexer] failed to create collection: %w", err)
	}

	return nil
}

// createIndex creates an index on the vector field using the configured IndexBuilder.
func createIndex(ctx context.Context, cli *milvusclient.Client, conf *IndexerConfig) error {
	if conf.VectorField != "" {
		var idx index.Index
		if conf.IndexBuilder != nil {
			idx = conf.IndexBuilder.Build(conf.MetricType)
		} else {
			idx = index.NewAutoIndex(conf.MetricType.toEntity())
		}

		createIndexOpt := milvusclient.NewCreateIndexOption(conf.Collection, conf.VectorField, idx)

		createTask, err := cli.CreateIndex(ctx, createIndexOpt)
		if err != nil {
			return fmt.Errorf("[NewIndexer] failed to create index: %w", err)
		}
		if err := createTask.Await(ctx); err != nil {
			return fmt.Errorf("[NewIndexer] failed to await index creation: %w", err)
		}
	}

	if conf.SparseVectorField != "" {
		var sparseIdx index.Index
		if conf.SparseIndexBuilder != nil {
			sparseIdx = conf.SparseIndexBuilder.Build(IP)
		} else {
			sparseIdx = NewSparseInvertedIndexBuilder().Build(IP)
		}

		createSparseIndexOpt := milvusclient.NewCreateIndexOption(conf.Collection, conf.SparseVectorField, sparseIdx)

		createTask, err := cli.CreateIndex(ctx, createSparseIndexOpt)
		if err != nil {
			return fmt.Errorf("[NewIndexer] failed to create sparse index: %w", err)
		}
		if err := createTask.Await(ctx); err != nil {
			return fmt.Errorf("[NewIndexer] failed to await sparse index creation: %w", err)
		}
	}

	return nil
}

// defaultDocumentConverter returns the default document to column converter.
func defaultDocumentConverter(vectorField, sparseVectorField string) func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
	return func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
		ids := make([]string, 0, len(docs))
		contents := make([]string, 0, len(docs))
		vecs := make([][]float32, 0, len(docs))
		sparseVecs := make([]entity.SparseEmbedding, 0, len(docs))
		metadatas := make([][]byte, 0, len(docs))

		for idx, doc := range docs {
			ids = append(ids, doc.ID)
			contents = append(contents, doc.Content)

			var sourceVec []float64
			if len(vectors) == len(docs) {
				sourceVec = vectors[idx]
			} else {
				sourceVec = doc.DenseVector()
			}

			if len(sourceVec) == 0 {
				return nil, fmt.Errorf("vector data missing for document %d (id: %s)", idx, doc.ID)
			}

			vec := make([]float32, len(sourceVec))
			for i, v := range sourceVec {
				vec[i] = float32(v)
			}
			vecs = append(vecs, vec)

			if sparseVectorField != "" {
				sparseMap := doc.SparseVector()

				positions := make([]uint32, 0, len(sparseMap))
				values := make([]float32, 0, len(sparseMap))
				for k, v := range sparseMap {
					positions = append(positions, uint32(k))
					values = append(values, float32(v))
				}

				se, err := entity.NewSliceSparseEmbedding(positions, values)
				if err != nil {
					return nil, fmt.Errorf("failed to create sparse embedding: %w", err)
				}
				sparseVecs = append(sparseVecs, se)
			}

			metadata, err := sonic.Marshal(doc.MetaData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal metadata: %w", err)
			}
			metadatas = append(metadatas, metadata)
		}

		dim := 0
		if len(vecs) > 0 {
			dim = len(vecs[0])
		}

		columns := []column.Column{
			column.NewColumnVarChar(defaultIDField, ids),
			column.NewColumnVarChar(defaultContentField, contents),
			column.NewColumnJSONBytes(defaultMetadataField, metadatas),
		}

		if vectorField != "" {
			columns = append(columns, column.NewColumnFloatVector(vectorField, dim, vecs))
		}

		if sparseVectorField != "" {
			columns = append(columns, column.NewColumnSparseVectors(sparseVectorField, sparseVecs))
		}

		return columns, nil
	}
}

// makeEmbeddingCtx creates a context with embedding callback information.
func (i *Indexer) makeEmbeddingCtx(ctx context.Context, emb embedding.Embedder) context.Context {
	runInfo := &callbacks.RunInfo{
		Component: components.ComponentOfEmbedding,
	}

	if embType, ok := components.GetType(emb); ok {
		runInfo.Type = embType
	}

	runInfo.Name = runInfo.Type + string(runInfo.Component)

	return callbacks.ReuseHandlers(ctx, runInfo)
}

// isIndexNotFoundError checks if the error is an "index not found" error from Milvus.
// This is expected when querying indexes on a newly created collection.
func isIndexNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "index not found") ||
		strings.Contains(errMsg, "index doesn't exist")
}
