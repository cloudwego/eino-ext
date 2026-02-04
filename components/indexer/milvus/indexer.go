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
	"time"

	"sync"

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

	// BatchSize is the number of documents to process in one batch
	// Optional, and the default value is 100
	BatchSize int

	// MaxConcurrency is the maximum number of concurrent batches
	// Optional, and the default value is 10
	MaxConcurrency int
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

	totalDocs := len(docs)
	batchSize := i.config.BatchSize
	concurrency := i.config.MaxConcurrency
	// collect all generated IDs
	allIDs := make([]string, totalDocs)
	// collect errors during concurrent processing
	errCh := make(chan error, totalDocs/batchSize+1)

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for idx := 0; idx < totalDocs; idx += batchSize {
		end := idx + batchSize
		if end > totalDocs {
			end = totalDocs
		}

		// split batch
		batchDocs := docs[idx:end]
		startIdx := idx

		wg.Add(1)
		go func(bDocs []*schema.Document, sIdx int) {
			defer wg.Done()
			// acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// 1. extract text
			texts := make([]string, len(bDocs))
			for k, doc := range bDocs {
				texts[k] = doc.Content
			}

			// 2. Embedding (note: perform embedding here instead of outside to save memory and utilize concurrency)
			vectors, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), texts)
			if err != nil {
				errCh <- fmt.Errorf("batch embedding failed: %w", err)
				return
			}

			if len(vectors) != len(bDocs) {
				errCh <- fmt.Errorf("embedding result length mismatch")
				return
			}

			// 3. convert data
			rows, err := i.config.DocumentConverter(ctx, bDocs, vectors)
			if err != nil {
				errCh <- fmt.Errorf("[Indexer.Store] failed to convert documents: %w", err)
				return
			}

			// 4. insert to Milvus
			// use partition from config
			results, err := i.config.Client.InsertRows(ctx, i.config.Collection, io.Partition, rows)
			if err != nil {
				errCh <- fmt.Errorf("[Indexer.Store] failed to insert rows: %w", err)
				return
			}

			// 5. collect IDs (maintain order)
			for k := 0; k < results.Len(); k++ {
				id, err := results.Get(k)
				if err != nil {
					errCh <- fmt.Errorf("get id failed: %w", err)
					return
				}
				// assume ID is string, convert if not
				strID, ok := id.(string)
				if !ok {
					// if ID is not string, handle according to actual situation, or simply use fmt.Sprint
					strID = fmt.Sprintf("%v", id)
				}
				if sIdx+k < len(allIDs) {
					allIDs[sIdx+k] = strID
				}
			}
		}(batchDocs, startIdx)
	}

	// wait for all batches to complete
	wg.Wait()
	close(errCh)

	// check if any error occurred
	if len(errCh) > 0 {
		// return the first error
		return nil, <-errCh
	}
	// finally: execute a global flush
	// flush collection to make sure the data is visible
	if err := i.config.Client.Flush(ctx, i.config.Collection, false); err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to flush collection: %w", err)
	}

	// load documents content
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.Content)
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{
		IDs: allIDs,
	})
	return allIDs, nil
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

	if i.BatchSize <= 0 {
		i.BatchSize = 10 // default 10 documents per batch
	}
	if i.MaxConcurrency <= 0 {
		i.MaxConcurrency = 10 // default 10 concurrent goroutines
	}
	return nil
}
