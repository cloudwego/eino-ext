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

package qdrant

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
)

type IndexerConfig struct {
	// Qdrant gRPC client
	Client *qdrant.Client
	// Collection name
	Collection string
	// Vector dimension
	VectorDim int
	// Distance metric
	Distance qdrant.Distance
	// BatchSize controls embedding texts size.
	BatchSize int `json:"batch_size"`
	// Embedder used to generate vector representations for documents.
	Embedding embedding.Embedder
	// DocumentToFields supports customize mapping from Document to Qdrant fields.
	DocumentToFields func(ctx context.Context, doc *schema.Document) (*Fields, error)
}

type Fields struct {
	ID       string
	Content  string
	Vector   []float64
	Metadata map[string]interface{}
}

type Indexer struct {
	config *IndexerConfig
}

func NewIndexer(ctx context.Context, config *IndexerConfig) (*Indexer, error) {
	if config.Embedding == nil {
		return nil, fmt.Errorf("[NewIndexer] embedding not provided for qdrant indexer")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("[NewIndexer] qdrant client not provided")
	}
	if config.Collection == "" {
		config.Collection = defaultCollection
	}
	if config.DocumentToFields == nil {
		config.DocumentToFields = defaultDocumentToFields
	}
	if config.BatchSize == 0 {
		config.BatchSize = 10
	}
	return &Indexer{config: config}, nil
}

func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	options := indexer.GetCommonOptions(&indexer.Options{
		Embedding: i.config.Embedding,
	}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{Docs: docs})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if err = i.batchUpsert(ctx, docs, options); err != nil {
		return nil, err
	}

	ids = make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})
	return ids, nil
}

func (i *Indexer) batchUpsert(ctx context.Context, docs []*schema.Document, options *indexer.Options) error {
	emb := options.Embedding
	batchSize := i.config.BatchSize

	if err := i.ensureCollection(ctx); err != nil {
		return err
	}
	for start := 0; start < len(docs); start += batchSize {
		end := start + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[start:end]
		var (
			points []*qdrant.PointStruct
			texts  []string
		)
		for _, doc := range batch {
			fields, err := i.config.DocumentToFields(ctx, doc)
			if err != nil {
				return err
			}
			texts = append(texts, fields.Content)
		}
		vectors, err := emb.EmbedStrings(ctx, texts)
		if err != nil {
			return fmt.Errorf("[batchUpsert] embedding failed, %w", err)
		}
		if len(vectors) != len(batch) {
			return fmt.Errorf("[batchUpsert] invalid vector length, expected=%d, got=%d", len(batch), len(vectors))
		}
		for idx, doc := range batch {
			fields, _ := i.config.DocumentToFields(ctx, doc)
			point := &qdrant.PointStruct{
				Id:      qdrant.NewID(fields.ID),
				Vectors: qdrant.NewVectors(float64SliceToFloat32(vectors[idx])...),
				Payload: qdrant.NewValueMap(map[string]any{
					defaultContentKey:  fields.Content,
					defaultMetadataKey: fields.Metadata,
				}),
			}
			points = append(points, point)
		}
		_, err = i.config.Client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: i.config.Collection,
			Points:         points,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Indexer) ensureCollection(ctx context.Context) error {
	exists, err := i.config.Client.CollectionExists(ctx, i.config.Collection)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	err = i.config.Client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: i.config.Collection,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(i.config.VectorDim),
			Distance: i.config.Distance,
		}),
	})
	return err
}

func (i *Indexer) GetType() string {
	return typ
}

func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

func defaultDocumentToFields(ctx context.Context, doc *schema.Document) (*Fields, error) {
	if doc.ID == "" {
		return nil, fmt.Errorf("[defaultDocumentToFields] doc id not set")
	}
	return &Fields{
		ID:       doc.ID,
		Content:  doc.Content,
		Metadata: doc.MetaData,
	}, nil
}

func float64SliceToFloat32(v []float64) []float32 {
	f := make([]float32, len(v))
	for i, x := range v {
		f[i] = float32(x)
	}
	return f
}
