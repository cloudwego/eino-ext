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

package pinecone

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/pinecone-io/go-pinecone/v3/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

type IndexerConfig struct {
	// Client is the Pinecone client to be called.
	// It requires the go-pinecone client.
	// Required.
	Client *pinecone.Client

	// IndexName is the name of the Pinecone index.
	// Optional. Default is "eino-index".
	IndexName string
	// Cloud specifies the cloud provider where the index is hosted.
	// Optional. Default is "aws".
	Cloud pinecone.Cloud
	// Region specifies the region where the index is hosted.
	// Optional. Default is "us-east-1".
	Region string
	// Metric defines the distance metric used for similarity search in the index.
	// e.g., "cosine", "euclidean", "dotproduct".
	// Optional. Default is "cosine".
	Metric pinecone.IndexMetric
	// Dimension is the dimensionality of the vectors to be stored in the index.
	// Optional. Default is 2560.
	Dimension int32
	// VectorType specifies the type of vectors stored in the index.
	// Optional. Default is "float32". Other types might be available based on Pinecone features.
	VectorType string
	// Namespace is the namespace within the Pinecone index where data will be stored.
	// Optional. If not specified, the default namespace is used.
	Namespace string
	// Tags are metadata tags to be associated with the index.
	// Optional.
	Tags *pinecone.IndexTags
	// DeletionProtection specifies if deletion protection is enabled for the index.
	// Optional. Default is typically "disabled".
	DeletionProtection pinecone.DeletionProtection

	// DocumentConverter is a function to convert schema.Document and their embeddings
	// into Pinecone-specific vector format.
	// Optional. If not provided, a default converter will be used.
	DocumentConverter func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]*pinecone.Vector, error)

	// BatchSize is the number of vectors to include in each batch when upserting data.
	// Optional. Default is 100.
	BatchSize int
	// MaxConcurrency is the maximum number of concurrent goroutines for upserting data.
	// Optional. Default is 10.
	MaxConcurrency int

	// Embedding is the vectorization method for values that need to be embedded
	// from schema.Document's content.
	// Required.
	Embedding embedding.Embedder
}

type Indexer struct {
	config *IndexerConfig
}

// NewIndexer creates a new indexer
func NewIndexer(ctx context.Context, conf *IndexerConfig) (*Indexer, error) {
	if err := conf.check(); err != nil {
		return nil, err
	}

	// Create index if it doesn't exist
	indexes, err := conf.Client.ListIndexes(ctx)
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to list indexes: %w", err)
	}
	exists := false
	for _, index := range indexes {
		if index.Name == conf.IndexName {
			exists = true
			break
		}
	}

	if !exists {
		if _, err := conf.Client.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
			Name:               conf.IndexName,
			Cloud:              conf.Cloud,
			Region:             conf.Region,
			Metric:             &conf.Metric,
			DeletionProtection: &conf.DeletionProtection,
			Dimension:          &conf.Dimension,
			VectorType:         &conf.VectorType,
			Tags:               conf.Tags,
		}); err != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to create index: %w", err)
		}
	}

	// load index info
	index, err := conf.Client.DescribeIndex(ctx, conf.IndexName)
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to describe index: %w", err)
	}

	// check index schema
	if err := validateIndexSchema(index, conf); err != nil {
		return nil, fmt.Errorf("[NewIndexer] index schema validation failed: %w", err)
	}

	return &Indexer{
		config: conf,
	}, nil
}

// validateIndexSchema checks if the index schema matches the config
func validateIndexSchema(index *pinecone.Index, conf *IndexerConfig) error {
	// Check dimension
	if index.Dimension != nil && *index.Dimension != conf.Dimension {
		return fmt.Errorf("index dimension mismatch: expected %d, got %d", conf.Dimension, *index.Dimension)
	}

	// Check metric
	if index.Metric != conf.Metric {
		return fmt.Errorf("index metric mismatch: expected %s, got %s", conf.Metric, index.Metric)
	}

	// Check deletion protection
	if index.DeletionProtection != conf.DeletionProtection {
		return fmt.Errorf("index deletion protection mismatch: expected %s, got %s", conf.DeletionProtection, index.DeletionProtection)
	}

	// Compare tags if provided
	if conf.Tags != nil {
		for k, v := range *conf.Tags {
			if index.Tags != nil && (*index.Tags)[k] != v {
				return fmt.Errorf("index tag mismatch for key %s: expected %v, got %v", k, v, (*index.Tags)[k])
			}
		}
	}

	return nil
}

// GetType returns the type of the indexer
func (i *Indexer) GetType() string {
	return typ
}

// IsCallbacksEnabled returns whether callbacks are enabled
func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

// Store stores documents into Pinecone index
func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{Docs: docs})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	options := indexer.GetCommonOptions(&indexer.Options{
		Embedding: i.config.Embedding,
	}, opts...)

	if options.Embedding == nil {
		return nil, fmt.Errorf("[Store] embedding not provided")
	}

	// insert documents to index
	if err := i.insert(ctx, docs, options); err != nil {
		return nil, fmt.Errorf("[Store] failed to insert document: %w", err)
	}

	ids = make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})

	return ids, nil
}

func (i *Indexer) insert(ctx context.Context, docs []*schema.Document, options *indexer.Options) error {
	// load docs content
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.Content)
	}

	// embedding docs
	embedder := options.Embedding
	if embedder == nil {
		return fmt.Errorf("[insert] embedding not provided")
	}

	vectors, err := embedder.EmbedStrings(makeEmbeddingCtx(ctx, embedder), texts)
	if err != nil {
		return err
	}

	if len(vectors) != len(docs) {
		return fmt.Errorf("[insert] number of documents mismatch: expected %d, got %d", len(docs), len(vectors))
	}

	// prepare pinecone vectors
	pcVectors, err := i.config.DocumentConverter(ctx, docs, vectors)
	if err != nil {
		return err
	}

	// parallel insert pinecone vectors
	if err := i.parallelInsert(ctx, pcVectors); err != nil {
		return err
	}

	return nil
}

func (i *Indexer) parallelInsert(ctx context.Context, vectors []*pinecone.Vector) error {
	// get index connection
	index, err := i.config.Client.DescribeIndex(ctx, i.config.IndexName)
	if err != nil {
		return fmt.Errorf("[Parallel] failed to describe index: %w", err)
	}
	indexConn, err := i.config.Client.Index(pinecone.NewIndexConnParams{
		Host:      index.Host,
		Namespace: i.config.Namespace,
	})
	if err != nil {
		return fmt.Errorf("[Parallel] failed to connect to index: %w", err)
	}

	batchSize := i.config.BatchSize
	batchNum := (len(vectors)-1)/batchSize + 1

	// make vectors into batches
	batches := make([][]*pinecone.Vector, 0, batchNum)
	for batchId := 0; batchId < batchNum; batchId++ {
		start := batchId * i.config.BatchSize
		end := start + batchSize
		if end > len(vectors) {
			end = len(vectors)
		}

		batch := vectors[start:end]
		batches = append(batches, batch)
	}

	// parallel add
	errChan := make(chan error, len(vectors))
	semaphore := make(chan struct{}, i.config.MaxConcurrency)
	var wg sync.WaitGroup

	for i, batch := range batches {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(batch []*pinecone.Vector) {
			defer func() {
				<-semaphore
				wg.Done()
			}()

			_, err := indexConn.UpsertVectors(ctx, batch)
			if err != nil {
				errChan <- fmt.Errorf("batch %d failed: %v", i, err)
			}
		}(batch)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return fmt.Errorf("[Parallel] failed to insert documents: %w", err)
		}
	}
	return nil
}

func (ic *IndexerConfig) getDefaultDocumentConvert() func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]*pinecone.Vector, error) {
	return func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]*pinecone.Vector, error) {
		if len(docs) != len(vectors) {
			return nil, fmt.Errorf("docs count mismatch: expected %d, got %d", len(vectors), len(docs))
		}

		pcVectors := make([]*pinecone.Vector, 0, len(docs))
		for i, doc := range docs {
			// check vector dimension
			if len(vectors[i]) != int(ic.Dimension) {
				return nil, fmt.Errorf("vector dimension mismatch: expected %d, got %d", ic.Dimension, len(vectors[i]))
			}

			// convert float64 to float32
			values := make([]float32, 0, len(vectors[i]))
			for _, v := range vectors[i] {
				values = append(values, float32(v))
			}

			// convert document metadata to pinecone metadata
			metadata, err := structpb.NewStruct(doc.MetaData)
			if err != nil {
				return nil, fmt.Errorf("[getDefaultDocumentConvert] failed to convert metadata: %w", err)
			}

			pcVector := &pinecone.Vector{
				Id:       doc.ID,
				Values:   &values,
				Metadata: metadata,
			}

			pcVectors = append(pcVectors, pcVector)
		}

		return pcVectors, nil
	}
}

// check the indexer config
func (ic *IndexerConfig) check() error {
	if ic.Client == nil {
		return fmt.Errorf("[NewIndexer] pinecone client not provided")
	}
	if ic.Embedding == nil {
		return fmt.Errorf("[NewIndexer] embedding not provided")
	}
	if ic.Dimension < 0 {
		return fmt.Errorf("[NewIndexer] dimension must be positive")
	}
	if ic.IndexName == "" {
		ic.IndexName = defaultIndexName
	}
	if ic.Cloud == "" {
		ic.Cloud = defaultCloud
	}
	if ic.Region == "" {
		ic.Region = defaultRegion
	}
	if ic.VectorType == "" {
		ic.VectorType = defaultVectorType
	}
	if ic.Dimension == 0 {
		ic.Dimension = defaultDimension
	}
	if ic.Metric == "" {
		ic.Metric = defaultMetric
	}
	if ic.DeletionProtection == "" {
		ic.DeletionProtection = defaultDeletionProtection
	}
	if ic.MaxConcurrency <= 0 {
		ic.MaxConcurrency = defaultMaxConcurrency
	}
	if ic.BatchSize <= 0 {
		ic.BatchSize = defaultBatchSize
	}
	if ic.DocumentConverter == nil {
		ic.DocumentConverter = ic.getDefaultDocumentConvert()
	}
	return nil
}
