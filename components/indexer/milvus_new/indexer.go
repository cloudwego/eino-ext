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
	"time"

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

type IndexerConfig struct {
	// Client is the milvus client to be called
	// It uses the new milvus/client/v2/milvusclient
	// Required
	Client milvusclient.Client

	// Default Collection config
	// Collection is the collection name in milvus database
	// Optional, and the default value is "eino_collection"
	Collection string
	// Description is the description for collection
	// Optional, and the default value is "the collection for eino"
	Description string
	// PartitionNum is the collection partition number
	// Optional, and the default value is 1(disable)
	// If the partition number is larger than 1, it means use partition and must have a partition key in Fields
	PartitionNum int64
	// PartitionName is the partition name in milvus database
	// Optional, and the default value is ""
	// If PartitionNum is larger than 1, it means use partition and not support manually specifying the partition names
	// give priority to using WithPartition
	PartitionName string
	// Fields is the collection fields
	// Optional, and the default value is the default fields
	Fields []*entity.Field
	// SharedNum is the milvus required param to create collection
	// Optional, and the default value is 1
	SharedNum int32
	// ConsistencyLevel is the milvus collection consistency tactics
	// Optional, and the default level is ClBounded(bounded consistency level with default tolerance of 5 seconds)
	ConsistencyLevel ConsistencyLevel
	// EnableDynamicSchema is means the collection is enabled to dynamic schema
	// Optional, and the default value is false
	// Enable to dynamic schema it could affect milvus performance
	EnableDynamicSchema bool

	// DocumentConverter is the function to convert the schema.Document to the row data (columns)
	// Optional, and the default value is defaultDocumentConverter
	DocumentConverter func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error)

	// Index config to the vector column
	// MetricType the metric type for vector
	// Optional and default type is HAMMING
	MetricType MetricType

	// Embedding vectorization method for values needs to be embedded from schema.Document's content.
	// Required
	Embedding embedding.Embedder
}

type Indexer struct {
	config IndexerConfig
}

// NewIndexer creates a new indexer.
func NewIndexer(ctx context.Context, conf *IndexerConfig) (*Indexer, error) {
	// conf check
	if err := conf.check(); err != nil {
		return nil, err
	}

	// check the collection whether to be created
	has, err := conf.Client.HasCollection(ctx, milvusclient.NewHasCollectionOption(conf.Collection))
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to check collection: %w", err)
	}

	if !has {
		// create the collection
		createOpt := milvusclient.NewCreateCollectionOption(conf.Collection, conf.getSchema(conf.Collection, conf.Description, conf.Fields)).
			WithShardNum(conf.SharedNum).
			WithConsistencyLevel(conf.ConsistencyLevel.getConsistencyLevel())

		if err := conf.Client.CreateCollection(ctx, createOpt); err != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to create collection: %w", err)
		}
	}

	// load collection info
	collection, err := conf.Client.DescribeCollection(ctx, milvusclient.NewDescribeCollectionOption(conf.Collection))
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to describe collection: %w", err)
	}

	// check collection schema
	if err := conf.checkCollectionSchema(collection.Schema, conf.Fields); err != nil {
		return nil, fmt.Errorf("[NewIndexer] collection schema not match: %w", err)
	}

	// check the collection load state
	if !collection.Loaded {
		// load collection
		if err := conf.loadCollection(ctx); err != nil {
			return nil, err
		}
	}

	if conf.PartitionNum == 0 && conf.PartitionName != "" {
		hasPartition, err := conf.Client.HasPartition(ctx, milvusclient.NewHasPartitionOption(conf.Collection, conf.PartitionName))
		if err != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to check partition: %w", err)
		}
		if !hasPartition {
			err := conf.Client.CreatePartition(ctx, milvusclient.NewCreatePartitionOption(conf.Collection, conf.PartitionName))
			if err != nil {
				return nil, fmt.Errorf("[NewIndexer] failed to create partition: %w", err)
			}
		}

		loadPartOpt := milvusclient.NewLoadPartitionsOption(conf.Collection, conf.PartitionName)
		_, err = conf.Client.LoadPartitions(ctx, loadPartOpt)
		if err != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to load partition: %w", err)
		}
	}

	// create indexer
	return &Indexer{
		config: *conf,
	}, nil
}

// Store stores the documents into the indexer.
func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	// get common options
	co := indexer.GetCommonOptions(&indexer.Options{
		SubIndexes: nil,
		Embedding:  i.config.Embedding,
	}, opts...)
	io := indexer.GetImplSpecificOptions(&ImplOptions{}, opts...)
	if io.Partition == "" {
		io.Partition = i.config.PartitionName
	}

	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	// callback info on start
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{
		Docs: docs,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[Indexer.Store] embedding not provided")
	}

	// load documents content
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.Content)
	}

	// embedding
	vectors, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), texts)
	if err != nil {
		return nil, err
	}

	if len(vectors) != len(docs) {
		return nil, fmt.Errorf("[Indexer.Store] embedding result length not match need: %d, got: %d", len(docs), len(vectors))
	}

	// convert documents to columns
	columns, err := i.config.DocumentConverter(ctx, docs, vectors)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to convert documents: %w", err)
	}

	// store documents into milvus
	insertOpt := milvusclient.NewColumnBasedInsertOption(i.config.Collection, columns...)
	if io.Partition != "" {
		insertOpt = insertOpt.WithPartition(io.Partition)
	}

	results, err := i.config.Client.Insert(ctx, insertOpt)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to insert rows: %w", err)
	}

	// flush collection to make sure the data is visible
	flushOpt := milvusclient.NewFlushOption(i.config.Collection)
	if _, err := i.config.Client.Flush(ctx, flushOpt); err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to flush collection: %w", err)
	}

	// callback info on end
	ids = make([]string, results.IDs.Len())
	for idx := 0; idx < results.IDs.Len(); idx++ {
		ids[idx], err = results.IDs.GetAsString(idx)
		if err != nil {
			return nil, fmt.Errorf("[Indexer.Store] failed to get id: %w", err)
		}
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{
		IDs: ids,
	})
	return ids, nil
}

func (i *Indexer) GetType() string {
	return typ
}

func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

// getSchema returns the schema
func (i *IndexerConfig) getSchema(collectionName, description string, fields []*entity.Field) *entity.Schema {
	return &entity.Schema{
		CollectionName: collectionName,
		Description:    description,
		Fields:         fields,
	}
}

func (i *IndexerConfig) getDefaultDocumentConvert() func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
	return func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
		ids := make([]string, len(docs))
		contents := make([]string, len(docs))
		vectorBytes := make([][]byte, len(docs))
		metadata := make([][]byte, len(docs))

		for idx, doc := range docs {
			ids[idx] = doc.ID
			contents[idx] = doc.Content
			vectorBytes[idx] = vector2Bytes(vectors[idx])

			metadataBytes, err := sonic.Marshal(doc.MetaData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal metadata: %w", err)
			}
			metadata[idx] = metadataBytes
		}

		// Create columns
		idColumn := column.NewColumnVarChar(defaultCollectionID, ids)
		contentColumn := column.NewColumnVarChar(defaultCollectionContent, contents)
		vectorColumn := column.NewColumnBinaryVector(defaultCollectionVector, defaultDim, vectorBytes)
		metadataColumn := column.NewColumnJSONBytes(defaultCollectionMetadata, metadata)

		return []column.Column{idColumn, contentColumn, vectorColumn, metadataColumn}, nil
	}
}

// createdDefaultIndex creates the default index
func (i *IndexerConfig) createdDefaultIndex(ctx context.Context, async bool) error {
	idx := index.NewAutoIndex(i.MetricType.getMetricType())

	createIndexOpt := milvusclient.NewCreateIndexOption(i.Collection, defaultIndexField, idx)

	_, err := i.Client.CreateIndex(ctx, createIndexOpt)
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to create index: %w", err)
	}
	return nil
}

// checkCollectionSchema checks the collection schema
func (i *IndexerConfig) checkCollectionSchema(schema *entity.Schema, fields []*entity.Field) error {
	// Check field count
	if len(schema.Fields) != len(fields) {
		return fmt.Errorf("field count mismatch: existing=%d, expected=%d. Existing fields: %v",
			len(schema.Fields), len(fields), getFieldNames(schema.Fields))
	}

	// Build a map of existing fields for easier lookup
	existingFields := make(map[string]*entity.Field)
	for _, f := range schema.Fields {
		existingFields[f.Name] = f
	}

	// Check each expected field
	var mismatches []string
	for _, expectedField := range fields {
		existingField, exists := existingFields[expectedField.Name]
		if !exists {
			mismatches = append(mismatches, fmt.Sprintf("field '%s' not found in existing schema", expectedField.Name))
			continue
		}
		if existingField.DataType != expectedField.DataType {
			mismatches = append(mismatches, fmt.Sprintf("field '%s' type mismatch: existing=%v, expected=%v",
				expectedField.Name, existingField.DataType, expectedField.DataType))
		}
	}

	if len(mismatches) > 0 {
		return fmt.Errorf("schema mismatches found: %v", mismatches)
	}

	return nil
}

func getFieldNames(fields []*entity.Field) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	return names
}

// loadCollection loads the collection
func (i *IndexerConfig) loadCollection(ctx context.Context) error {
	loadStateOpt := milvusclient.NewGetLoadStateOption(i.Collection)
	loadState, err := i.Client.GetLoadState(ctx, loadStateOpt)
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to get load state: %w", err)
	}

	switch loadState.State {
	case entity.LoadStateNotLoad:
		// Check if index exists
		descIndexOpt := milvusclient.NewDescribeIndexOption(i.Collection, defaultIndexField)
		_, err := i.Client.DescribeIndex(ctx, descIndexOpt)
		if err != nil {
			if err := i.createdDefaultIndex(ctx, false); err != nil {
				return err
			}
		}

		loadOpt := milvusclient.NewLoadCollectionOption(i.Collection)
		_, err = i.Client.LoadCollection(ctx, loadOpt)
		if err != nil {
			return err
		}
		return nil
	case entity.LoadStateLoading:
		// Wait for loading to complete
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				state, err := i.Client.GetLoadState(ctx, loadStateOpt)
				if err != nil {
					return err
				}
				if state.State == entity.LoadStateLoaded {
					return nil
				}
			}
		}
	default:
		return nil
	}
}

// check the indexer config
func (i *IndexerConfig) check() error {
	// Check if Client is nil using reflection safely
	clientVal := reflect.ValueOf(i.Client)
	if !clientVal.IsValid() {
		return fmt.Errorf("[NewIndexer] milvus client not provided")
	}
	// IsNil can only be called on certain kinds
	kind := clientVal.Kind()
	if (kind == reflect.Ptr || kind == reflect.Interface || kind == reflect.Slice ||
		kind == reflect.Map || kind == reflect.Chan || kind == reflect.Func) && clientVal.IsNil() {
		return fmt.Errorf("[NewIndexer] milvus client not provided")
	}
	if i.Embedding == nil {
		return fmt.Errorf("[NewIndexer] embedding not provided")
	}
	if i.PartitionNum > 1 && i.PartitionName != "" {
		return fmt.Errorf("[NewIndexer] not support manually specifying the partition names if partition key mode is used")
	}
	if i.Collection == "" {
		i.Collection = defaultCollection
	}
	if i.Description == "" {
		i.Description = defaultDescription
	}
	if i.SharedNum <= 0 {
		i.SharedNum = 1
	}
	if i.ConsistencyLevel <= 0 || i.ConsistencyLevel > 5 {
		i.ConsistencyLevel = defaultConsistencyLevel
	}
	if i.MetricType == "" {
		i.MetricType = defaultMetricType
	}
	if i.PartitionNum <= 1 {
		i.PartitionNum = 0
	}
	if i.Fields == nil {
		i.Fields = getDefaultFields()
	}
	if i.DocumentConverter == nil {
		i.DocumentConverter = i.getDefaultDocumentConvert()
	}
	return nil
}
