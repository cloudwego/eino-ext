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

// Package milvus provides an Indexer implementation using Milvus vector database.
//
// Deprecated: This package uses milvus-sdk-go v2.4.x which only supports Milvus 2.4.
// Use github.com/cloudwego/eino-ext/components/indexer/milvus2 instead.
package milvus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type IndexerConfig struct {
	// Client is the milvus client to be called
	// It requires the milvus-sdk-go client of version 2.4.x
	// Required
	Client client.Client

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

	// DocumentConverter is the function to convert the schema.Document to the row data
	// Optional, and the default value is defaultDocumentConverter
	DocumentConverter func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error)

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
	ok, err := conf.Client.HasCollection(ctx, conf.Collection)
	if err != nil {
		if errors.Is(err, client.ErrClientNotReady) {
			return nil, fmt.Errorf("[NewIndexer] milvus client not ready: %w", err)
		}
		if errors.Is(err, client.ErrStatusNil) {
			return nil, fmt.Errorf("[NewIndexer] milvus client status is nil: %w", err)
		}
		return nil, fmt.Errorf("[NewIndexer] failed to check collection: %w", err)
	}
	if !ok {
		// create the collection
		if errToCreate := conf.Client.CreateCollection(
			ctx,
			conf.getSchema(conf.Collection, conf.Description, conf.Fields),
			conf.SharedNum,
			client.WithConsistencyLevel(
				conf.ConsistencyLevel.getConsistencyLevel(),
			),
			client.WithEnableDynamicSchema(conf.EnableDynamicSchema),
			client.WithPartitionNum(conf.PartitionNum),
		); errToCreate != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to create collection: %w", errToCreate)
		}
	}

	// load collection info
	collection, err := conf.Client.DescribeCollection(ctx, conf.Collection)
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to describe collection: %w", err)
	}
	// check collection schema
	if !conf.checkCollectionSchema(collection.Schema, conf.Fields) {
		return nil, fmt.Errorf("[NewIndexer] collection schema not match")
	}
	// check the collection load state
	if !collection.Loaded {
		// load collection
		if err := conf.loadCollection(ctx); err != nil {
			return nil, err
		}
	}

	if conf.PartitionNum == 0 && conf.PartitionName != "" {
		ok, err = conf.Client.HasPartition(ctx, conf.Collection, conf.PartitionName)
		if err != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to check partition: %w", err)
		}
		if !ok {
			err := conf.Client.CreatePartition(ctx, conf.Collection, conf.PartitionName)
			if err != nil {
				return nil, fmt.Errorf("[NewIndexer] failed to create partition: %w", err)
			}
		}
		if err = conf.Client.LoadPartitions(ctx, conf.Collection, []string{conf.PartitionName}, false); err != nil {
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

	// load documents content
	rows, err := i.config.DocumentConverter(ctx, docs, vectors)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to convert documents: %w", err)
	}

	// store documents into milvus
	results, err := i.config.Client.InsertRows(ctx, i.config.Collection, io.Partition, rows)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to insert rows: %w", err)
	}

	// flush collection to make sure the data is visible
	if err := i.config.Client.Flush(ctx, i.config.Collection, false); err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to flush collection: %w", err)
	}

	// callback info on end
	ids = make([]string, results.Len())
	for idx := 0; idx < results.Len(); idx++ {
		ids[idx], err = results.GetAsString(idx)
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

// getDefaultSchema returns the default schema
func (i *IndexerConfig) getSchema(collection, description string, fields []*entity.Field) *entity.Schema {
	s := entity.NewSchema().
		WithName(collection).
		WithDescription(description)
	for _, field := range fields {
		s.WithField(field)
	}
	return s
}

func (i *IndexerConfig) getDefaultDocumentConvert() func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
	return func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
		em := make([]defaultSchema, 0, len(docs))
		texts := make([]string, 0, len(docs))
		rows := make([]interface{}, 0, len(docs))

		for _, doc := range docs {
			metadata, err := sonic.Marshal(doc.MetaData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal metadata: %w", err)
			}
			em = append(em, defaultSchema{
				ID:       doc.ID,
				Content:  doc.Content,
				Vector:   nil,
				Metadata: metadata,
			})
			texts = append(texts, doc.Content)
		}

		// build embedding documents for storing
		for idx, vec := range vectors {
			em[idx].Vector = vector2Bytes(vec)
			rows = append(rows, &em[idx])
		}
		return rows, nil
	}
}

// createdDefaultIndex creates the default index
func (i *IndexerConfig) createdDefaultIndex(ctx context.Context, async bool) error {
	index, err := entity.NewIndexAUTOINDEX(i.MetricType.getMetricType())
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to create index: %w", err)
	}
	if err := i.Client.CreateIndex(ctx, i.Collection, defaultIndexField, index, async); err != nil {
		return fmt.Errorf("[NewIndexer] failed to create index: %w", err)
	}
	return nil
}

// checkCollectionSchema checks the collection schema
func (i *IndexerConfig) checkCollectionSchema(schema *entity.Schema, field []*entity.Field) bool {
	var count int
	if len(schema.Fields) != len(field) {
		return false
	}
	for _, f := range schema.Fields {
		for _, e := range field {
			if f.Name == e.Name && f.DataType == e.DataType {
				count++
			}
		}
	}
	if count != len(field) {
		return false
	}
	return true
}

// getCollectionDim gets the collection dimension
func (i *IndexerConfig) loadCollection(ctx context.Context) error {
	loadState, err := i.Client.GetLoadState(ctx, i.Collection, nil)
	if err != nil {
		return fmt.Errorf("[NewIndexer] failed to get load state: %w", err)
	}
	switch loadState {
	case entity.LoadStateNotExist:
		return fmt.Errorf("[NewIndexer] collection not exist")
	case entity.LoadStateNotLoad:
		index, err := i.Client.DescribeIndex(ctx, i.Collection, "vector")
		if errors.Is(err, client.ErrClientNotReady) {
			return fmt.Errorf("[NewIndexer] milvus client not ready: %w", err)
		}
		if len(index) == 0 {
			if err := i.createdDefaultIndex(ctx, false); err != nil {
				return err
			}
		}
		if err := i.Client.LoadCollection(ctx, i.Collection, true); err != nil {
			return err
		}
		return nil
	case entity.LoadStateLoading:
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				loadingProgress, err := i.Client.GetLoadingProgress(ctx, i.Collection, nil)
				if err != nil {
					return err
				}
				if loadingProgress == 100 {
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
	if i.Client == nil {
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
