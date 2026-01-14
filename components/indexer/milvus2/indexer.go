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
	"log"
	"sort"

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
	ClientConfig *milvusclient.ClientConfig

	// Collection is the collection name in Milvus.
	// Default: "eino_collection"
	Collection string

	// Description is the description for the collection.
	// Default: "the collection for eino"
	Description string

	// PartitionName is the default partition for insertion.
	// Optional.
	PartitionName string

	// ConsistencyLevel for Milvus operations.
	// Default: ConsistencyLevelBounded
	ConsistencyLevel ConsistencyLevel

	// EnableDynamicSchema enables dynamic field support for flexible metadata.
	// Default: false
	EnableDynamicSchema bool

	// Vector defines the configuration for dense vector index.
	// Optional.
	Vector *VectorConfig

	// Sparse defines the configuration for sparse vector index.
	// Optional.
	Sparse *SparseVectorConfig

	// DocumentConverter converts EINO documents to Milvus columns.
	// If nil, uses default conversion (id, content, vector, metadata as JSON).
	DocumentConverter func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error)

	// Embedding is the embedder for vectorization.
	// Required.
	Embedding embedding.Embedder

	// Functions defines the Milvus built-in functions (e.g. BM25) to be added to the schema.
	// Optional.
	Functions []*entity.Function

	// FieldParams defines extra parameters for fields (e.g. "enable_analyzer": "true").
	// Key is field name, value is a map of parameter key-value pairs.
	// Optional.
	FieldParams map[string]map[string]string
}

// VectorConfig contains configuration for dense vector index.
type VectorConfig struct {
	// Dimension is the vector dimension.
	// Required when auto-creating the collection.
	Dimension int64

	// MetricType is the metric type for vector similarity.
	// Default: L2
	MetricType MetricType

	// IndexBuilder specifies how to build the vector index.
	// If nil, uses AutoIndex (Milvus automatically selects the best index).
	// Use NewHNSWIndexBuilder(), NewIVFFlatIndexBuilder(), etc. for specific index types.
	IndexBuilder IndexBuilder

	// VectorField is the name of the vector field in the collection.
	// Default: "vector"
	VectorField string
}

// SparseMethod defines the method for sparse vector generation.
type SparseMethod string

const (
	// SparseMethodAuto indicates that Milvus will automatically generate sparse vectors
	// (e.g. via built-in BM25 function) on the server side.
	SparseMethodAuto SparseMethod = "Auto"
	// SparseMethodPrecomputed indicates that the client will provide precomputed sparse vectors
	// (e.g. from document.SparseVector()).
	SparseMethodPrecomputed SparseMethod = "Precomputed"
)

// SparseVectorConfig contains configuration for sparse vector index.
type SparseVectorConfig struct {
	// IndexBuilder specifies how to build the sparse vector index.
	// Optional. If nil, uses SPARSE_INVERTED_INDEX by default.
	IndexBuilder SparseIndexBuilder

	// VectorField is the name of the sparse vector field in the collection.
	// Optional. Default: "sparse_vector"
	VectorField string

	// MetricType is the metric type for sparse vector similarity.
	// Optional. Default: BM25.
	MetricType MetricType

	// Method specifies the method for sparse vector generation.
	// Optional. Default: SparseMethodAuto if MetricType is BM25, otherwise SparseMethodPrecomputed.
	Method SparseMethod
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

	cli, err := initClient(ctx, conf)
	if err != nil {
		return nil, err
	}

	if err := initCollection(ctx, cli, conf); err != nil {
		return nil, err
	}

	return &Indexer{
		client: cli,
		config: conf,
	}, nil
}

func initClient(ctx context.Context, conf *IndexerConfig) (*milvusclient.Client, error) {
	if conf.Client != nil {
		return conf.Client, nil
	}

	if conf.ClientConfig == nil {
		return nil, fmt.Errorf("[NewIndexer] either Client or ClientConfig must be provided")
	}

	cli, err := milvusclient.New(ctx, conf.ClientConfig)
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to create milvus client: %w", err)
	}

	return cli, nil
}

func initCollection(ctx context.Context, cli *milvusclient.Client, conf *IndexerConfig) error {
	hasCollection, err := cli.HasCollection(ctx, milvusclient.NewHasCollectionOption(conf.Collection))
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to check collection: %w", err)
	}

	if !hasCollection {
		// Dimension is required only if Vector config is present or we want default behavior.
		// However, in this new design, if Collection doesn't exist, we need to know schema.
		// Schema builder checks conf.Vector. So we check it here too if needed?
		// Actually buildSchema checks conf.Vector.
		if conf.Vector != nil && conf.Vector.Dimension <= 0 {
			return fmt.Errorf("[NewIndexer] vector dimension is required when collection does not exist")
		}
		if err := createCollection(ctx, cli, conf); err != nil {
			return err
		}
	}

	loadState, err := cli.GetLoadState(ctx, milvusclient.NewGetLoadStateOption(conf.Collection))
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to get load state: %w", err)
	}
	if loadState.State != entity.LoadStateLoaded {
		// Try to create indexes. Ignore "already exists" errors.
		if err := createIndex(ctx, cli, conf); err != nil {
			return err
		}

		loadTask, err := cli.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(conf.Collection))
		if err != nil {
			return fmt.Errorf("[NewIndexer] failed to load collection: %w", err)
		}
		if err := loadTask.Await(ctx); err != nil {
			return fmt.Errorf("[NewIndexer] failed to await collection load: %w", err)
		}
	}

	return nil
}

// Store adds the provided documents to the Milvus collection.
// It returns the list of IDs for the stored documents or an error.
func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	co := indexer.GetCommonOptions(&indexer.Options{
		Embedding: i.config.Embedding,
	}, opts...)
	io := indexer.GetImplSpecificOptions(&ImplOptions{
		Partition: i.config.PartitionName,
	}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{
		Docs: docs,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	vectors, err := i.embedDocuments(ctx, co.Embedding, docs)
	if err != nil {
		return nil, err
	}

	upsertResult, err := i.upsertDocuments(ctx, docs, vectors, io.Partition)
	if err != nil {
		return nil, err
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{
		IDs: upsertResult,
	})

	return upsertResult, nil
}

func (i *Indexer) embedDocuments(ctx context.Context, emb embedding.Embedder, docs []*schema.Document) ([][]float64, error) {
	if emb == nil {
		return nil, nil // Return nil vectors if no embedder
	}

	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.Content)
	}

	vectors, err := emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to embed documents: %w", err)
	}
	if len(vectors) != len(docs) {
		return nil, fmt.Errorf("[Indexer.Store] embedding result length mismatch: need %d, got %d", len(docs), len(vectors))
	}
	return vectors, nil
}

func (i *Indexer) upsertDocuments(ctx context.Context, docs []*schema.Document, vectors [][]float64, partition string) ([]string, error) {
	columns, err := i.config.DocumentConverter(ctx, docs, vectors)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to convert documents: %w", err)
	}

	insertOpt := milvusclient.NewColumnBasedInsertOption(i.config.Collection)
	if partition != "" {
		insertOpt = insertOpt.WithPartition(partition)
	}
	for _, col := range columns {
		insertOpt = insertOpt.WithColumns(col)
	}

	result, err := i.client.Upsert(ctx, insertOpt)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to upsert documents: %w", err)
	}

	ids := make([]string, 0, result.IDs.Len())
	for idx := 0; idx < result.IDs.Len(); idx++ {
		idStr, err := result.IDs.GetAsString(idx)
		if err != nil {
			return nil, fmt.Errorf("[Indexer.Store] failed to get id: %w", err)
		}
		ids = append(ids, idStr)
	}

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

	// Ensure at least one vector config is present
	if c.Vector == nil && c.Sparse == nil {
		return fmt.Errorf("[NewIndexer] at least one vector field (dense or sparse) is required")
	}

	if c.Collection == "" {
		c.Collection = defaultCollection
	}
	if c.Description == "" {
		c.Description = defaultDescription
	}

	// Dense vector defaults
	if c.Vector != nil {
		if c.Vector.MetricType == "" {
			c.Vector.MetricType = L2
		}
		if c.Vector.VectorField == "" {
			c.Vector.VectorField = defaultVectorField
		}
	}

	// Sparse vector defaults
	if c.Sparse != nil {
		if c.Sparse.VectorField == "" {
			c.Sparse.VectorField = defaultSparseVectorField
		}
		if c.Sparse.MetricType == "" {
			c.Sparse.MetricType = BM25
		}

		if c.Sparse.Method == "" {
			// Auto uses Milvus server-side functions (e.g. BM25).
			// Precomputed requires the user to provide the sparse vector in the document.
			// Currently, Milvus only provides built-in Auto support for BM25.
			// For other metrics (e.g. IP), default to Precomputed.
			if c.Sparse.MetricType == BM25 {
				c.Sparse.Method = SparseMethodAuto
			} else {
				c.Sparse.Method = SparseMethodPrecomputed
			}
		}

		c.addDefaultBM25Function()
	}

	if c.DocumentConverter == nil {
		c.DocumentConverter = defaultDocumentConverter(c.Vector, c.Sparse)
	}
	return nil
}

func (c *IndexerConfig) addDefaultBM25Function() {
	if c.Sparse != nil && c.Sparse.Method == SparseMethodAuto {
		hasSparseFunc := false
		for _, fn := range c.Functions {
			for _, output := range fn.OutputFieldNames {
				if output == c.Sparse.VectorField {
					hasSparseFunc = true
					break
				}
			}
			if hasSparseFunc {
				break
			}
		}

		if !hasSparseFunc {
			bm25Fn := entity.NewFunction().
				WithName("bm25_auto").
				WithType(entity.FunctionTypeBM25).
				WithInputFields(defaultContentField).
				WithOutputFields(c.Sparse.VectorField)
			c.Functions = append(c.Functions, bm25Fn)
		}
	}
}

// createCollection creates a new Milvus collection with the default schema.
func createCollection(ctx context.Context, cli *milvusclient.Client, conf *IndexerConfig) error {
	sch, err := buildSchema(conf)
	if err != nil {
		return err
	}

	createOpt := milvusclient.NewCreateCollectionOption(conf.Collection, sch)
	if conf.ConsistencyLevel != ConsistencyLevelDefault {
		createOpt = createOpt.WithConsistencyLevel(conf.ConsistencyLevel.ToEntity())
	}

	if err := cli.CreateCollection(ctx, createOpt); err != nil {
		return fmt.Errorf("[NewIndexer] failed to create collection: %w", err)
	}

	return nil
}

func buildSchema(conf *IndexerConfig) (*entity.Schema, error) {
	// Helper to apply field params
	applyParams := func(f *entity.Field, name string) {
		if params, ok := conf.FieldParams[name]; ok {
			for k, v := range params {
				f.WithTypeParams(k, v)
			}
		}
	}

	idField := entity.NewField().
		WithName(defaultIDField).
		WithDataType(entity.FieldTypeVarChar).
		WithMaxLength(defaultMaxIDLen).
		WithIsPrimaryKey(true)
	applyParams(idField, defaultIDField)

	contentField := entity.NewField().
		WithName(defaultContentField).
		WithDataType(entity.FieldTypeVarChar).
		WithMaxLength(defaultMaxContentLen)
	applyParams(contentField, defaultContentField)

	metadataField := entity.NewField().
		WithName(defaultMetadataField).
		WithDataType(entity.FieldTypeJSON)
	applyParams(metadataField, defaultMetadataField)

	sch := entity.NewSchema().
		WithField(idField).
		WithField(contentField).
		WithField(metadataField).
		WithDynamicFieldEnabled(conf.EnableDynamicSchema)

	if conf.Vector != nil {
		vecField := entity.NewField().
			WithName(conf.Vector.VectorField).
			WithDataType(entity.FieldTypeFloatVector).
			WithDim(conf.Vector.Dimension)
		applyParams(vecField, conf.Vector.VectorField)
		sch.WithField(vecField)
	} else if conf.Sparse == nil {
		// Should not happen if validation passed, but safety check: at least one vector field required
		return nil, fmt.Errorf("[NewIndexer] at least one vector field (dense or sparse) is required")
	}

	if conf.Sparse != nil {
		sparseField := entity.NewField().
			WithName(conf.Sparse.VectorField).
			WithDataType(entity.FieldTypeSparseVector)
		applyParams(sparseField, conf.Sparse.VectorField)
		sch.WithField(sparseField)
	}

	// Add functions to schema
	for _, fn := range conf.Functions {
		sch.WithFunction(fn)
	}

	return sch, nil
}

// createIndex creates indexes on fields if they don't exist.
func createIndex(ctx context.Context, cli *milvusclient.Client, conf *IndexerConfig) error {
	if conf.Vector != nil {
		if err := createVectorIndex(ctx, cli, conf.Vector.VectorField, conf.Vector, conf.Collection); err != nil {
			return err
		}
	}

	if conf.Sparse != nil {
		if err := createSparseIndex(ctx, cli, conf.Sparse, conf.Collection); err != nil {
			return err
		}
	}

	return nil
}

func createVectorIndex(ctx context.Context, cli *milvusclient.Client, vectorField string, vectorConf *VectorConfig, collection string) error {
	var idx index.Index
	if vectorConf.IndexBuilder != nil {
		idx = vectorConf.IndexBuilder.Build(vectorConf.MetricType)
	} else {
		idx = index.NewAutoIndex(vectorConf.MetricType.toEntity())
	}

	descOpts := milvusclient.NewDescribeIndexOption(collection, vectorField)
	_, err := cli.DescribeIndex(ctx, descOpts)
	if err == nil {
		log.Printf("[NewIndexer] vector index for field %s already exists, skipping creation", vectorField)
		return nil
	}

	createIndexOpt := milvusclient.NewCreateIndexOption(collection, vectorField, idx)

	createTask, err := cli.CreateIndex(ctx, createIndexOpt)
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to create index: %w", err)
	}

	if err := createTask.Await(ctx); err != nil {
		return fmt.Errorf("[NewIndexer] failed to await index creation: %w", err)
	}
	return nil
}

func createSparseIndex(ctx context.Context, cli *milvusclient.Client, sparseConf *SparseVectorConfig, collection string) error {
	var sparseIdx index.Index
	if sparseConf.IndexBuilder != nil {
		sparseIdx = sparseConf.IndexBuilder.Build(sparseConf.MetricType)
	} else {
		sparseIdx = NewSparseInvertedIndexBuilder().Build(sparseConf.MetricType)
	}

	descOpts := milvusclient.NewDescribeIndexOption(collection, sparseConf.VectorField)
	_, err := cli.DescribeIndex(ctx, descOpts)
	if err == nil {
		log.Printf("[NewIndexer] sparse index for field %s already exists, skipping creation", sparseConf.VectorField)
		return nil
	}

	createSparseIndexOpt := milvusclient.NewCreateIndexOption(collection, sparseConf.VectorField, sparseIdx)

	createTask, err := cli.CreateIndex(ctx, createSparseIndexOpt)
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to create sparse index: %w", err)
	}

	if err := createTask.Await(ctx); err != nil {
		return fmt.Errorf("[NewIndexer] failed to await sparse index creation: %w", err)
	}
	return nil
}

// defaultDocumentConverter returns the default document to column converter.
func defaultDocumentConverter(vector *VectorConfig, sparse *SparseVectorConfig) func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
	return func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
		ids := make([]string, 0, len(docs))
		contents := make([]string, 0, len(docs))
		vecs := make([][]float32, 0, len(docs))
		metadatas := make([][]byte, 0, len(docs))
		sparseVecs := make([]entity.SparseEmbedding, 0, len(docs))

		// Determine if we need to handle sparse vectors
		sparseVectorField := ""
		if sparse != nil && sparse.Method == SparseMethodPrecomputed {
			sparseVectorField = sparse.VectorField
		}

		// Determine if we need to handle dense vectors
		denseVectorField := ""
		if vector != nil {
			denseVectorField = vector.VectorField
		}

		for idx, doc := range docs {
			ids = append(ids, doc.ID)
			contents = append(contents, doc.Content)

			var sourceVec []float64
			if len(vectors) == len(docs) {
				sourceVec = vectors[idx]
			} else {
				sourceVec = doc.DenseVector()
			}

			// Dense vector is required when vectorField is set (dense-only or hybrid mode).
			if denseVectorField != "" {
				if len(sourceVec) == 0 {
					return nil, fmt.Errorf("vector data missing for document %d (id: %s)", idx, doc.ID)
				}
				vec := make([]float32, len(sourceVec))
				for i, v := range sourceVec {
					vec[i] = float32(v)
				}
				vecs = append(vecs, vec)
			}

			if sparseVectorField != "" {
				sv := doc.SparseVector()
				se, err := toMilvusSparseEmbedding(sv)
				if err != nil {
					return nil, fmt.Errorf("failed to convert sparse vector for document %d: %w", idx, err)
				}
				sparseVecs = append(sparseVecs, se)
			}

			metadata, err := sonic.Marshal(doc.MetaData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal metadata: %w", err)
			}
			metadatas = append(metadatas, metadata)
		}

		columns := []column.Column{
			column.NewColumnVarChar(defaultIDField, ids),
			column.NewColumnVarChar(defaultContentField, contents),
			column.NewColumnJSONBytes(defaultMetadataField, metadatas),
		}

		if denseVectorField != "" {
			dim := 0
			if len(vecs) > 0 {
				dim = len(vecs[0])
			}
			columns = append(columns, column.NewColumnFloatVector(denseVectorField, dim, vecs))
		}

		if sparseVectorField != "" {
			// SparseFloatVector column does not typically require specific dimension argument in insert
			columns = append(columns, column.NewColumnSparseVectors(sparseVectorField, sparseVecs))
		}

		return columns, nil
	}
}

func toMilvusSparseEmbedding(sv map[int]float64) (entity.SparseEmbedding, error) {
	if len(sv) == 0 {
		return entity.NewSliceSparseEmbedding([]uint32{}, []float32{})
	}

	indices := make([]int, 0, len(sv))
	for k := range sv {
		indices = append(indices, k)
	}

	sort.Ints(indices)

	uint32Indices := make([]uint32, len(indices))
	values := make([]float32, len(indices))

	for i, idx := range indices {
		if idx < 0 {
			return nil, fmt.Errorf("negative sparse index: %d", idx)
		}
		uint32Indices[i] = uint32(idx)
		values[i] = float32(sv[idx])
	}

	return entity.NewSliceSparseEmbedding(uint32Indices, values)
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
