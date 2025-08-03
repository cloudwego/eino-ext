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
	"fmt"
	
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// Indexer is a Milvus indexer that implements the indexer.Indexer interface.
type Indexer struct {
	conf *IndexerConfig
}

// Store stores the given documents in the Milvus collection.
func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	// Get common options and implementation-specific options
	co := indexer.GetCommonOptions(&indexer.Options{
		SubIndexes: nil,
		Embedding:  i.conf.Embedding,
	}, opts...)
	
	io := indexer.GetImplSpecificOptions(&ImplOptions{}, opts...)
	if io.Partition == "" {
		io.Partition = i.conf.Partition
	}
	
	// Ensure the context has the necessary run info
	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	callbacks.OnStart(ctx, &indexer.CallbackInput{
		Docs: docs,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()
	
	// Get the embedding from the common options
	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[Indexer.Store] embedding is not set")
	}
	
	// Embed the documents
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.Content)
	}
	
	vectors, err := emb.EmbedStrings(makeEmbeddingCtx(ctx, emb), texts)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to embed documents: %w", err)
	}
	
	if len(vectors) != len(docs) {
		return nil, fmt.Errorf("[Indexer.Store] converter returned %d rows, expected %d", len(vectors), len(docs))
	}
	
	// Convert documents to Milvus schema
	rows, err := i.conf.DocumentConverter(docs, int(i.conf.Dim), vectors)
	if err != nil {
		return nil, fmt.Errorf("[Indexer.Store] failed to convert documents: %w", err)
	}
	// Persistent vectors
	insertOpt := milvusclient.NewColumnBasedInsertOption(i.conf.Collection, rows...)
	
	var res column.Column
	if io.Upsert {
		result, err := i.conf.Client.Upsert(ctx, insertOpt)
		if err != nil {
			return nil, fmt.Errorf("[Indexer.Store] failed to upsert documents: %w", err)
		}
		res = result.IDs
	} else {
		result, err := i.conf.Client.Insert(ctx, insertOpt)
		if err != nil {
			return nil, fmt.Errorf("[Indexer.Store] failed to insert documents: %w", err)
		}
		res = result.IDs
	}
	
	// Convert IDs to string
	ids = make([]string, 0, res.Len())
	for _, doc := range docs {
		ids = append(ids, doc.ID)
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

// NewIndexer creates a new Milvus indexer with the given configuration.
func NewIndexer(ctx context.Context, conf *IndexerConfig) (*Indexer, error) {
	// validate the configuration
	if err := conf.validate(); err != nil {
		return nil, fmt.Errorf("[Indexer.NewIndexer] invalid indexer config: %w", err)
	}
	
	if err := conf.ensureCollation(ctx); err != nil {
		return nil, fmt.Errorf("[Indexer.NewIndexer] failed to ensure collection: %w", err)
	}
	
	return &Indexer{conf: conf}, nil
}
