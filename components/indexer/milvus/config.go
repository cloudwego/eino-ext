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
	
	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// IndexerConfig is the configuration for the Milvus indexer.
type IndexerConfig struct {
	// Client is the Milvus client to connect to the Milvus server.
	// It is required to create a new indexer.
	Client *milvusclient.Client
	
	// Collection is the name of the collection to store the documents.
	Collection string
	// Partition is the name of the partition to store the documents.
	Partition string
	// Schema is the schema of the collection.
	Schema *entity.Schema
	// Dim is the dimension of the vector to store in the collection.
	// It is required to create a new indexer.
	Dim int64
	// CollectionOpt is the option to create the collection.
	// It is used to create the collection if it does not exist.
	// If it is nil, the default options will be used.
	CollectionOpt milvusclient.CreateCollectionOption
	
	// DocumentConverter is the function to convert documents to Milvus schema.
	// It takes a slice of documents and a slice of vectors, and returns a slice of
	// columns that can be inserted into the Milvus collection.
	DocumentConverter DocumentConverter
	// Embedding is the embedder to embed the documents.
	// It is used to convert the document content into vectors.
	Embedding embedding.Embedder
}

// validate validates the IndexerConfig.
// It checks if the Milvus client is not nil, the dimension is greater than 0
func (i *IndexerConfig) validate() error {
	if i.Client == nil {
		return fmt.Errorf("[IndexerConfig.validate] the milvus client is nil")
	}
	if i.Dim <= 0 {
		return fmt.Errorf("[IndexerConfig.validate] the dimension of the vector must be greater than 0")
	}
	if i.DocumentConverter == nil {
		i.DocumentConverter = defaultDocumentConverter
	}
	if i.Collection == "" {
		i.Collection = defaultCollection
	}
	if i.Partition == "" {
		i.Partition = defaultPartition
	}
	return nil
}

// ensureCollation ensures that the collection exists in the Milvus server.
func (i *IndexerConfig) ensureCollation(ctx context.Context) error {
	ok, err := i.Client.HasCollection(ctx, milvusclient.NewHasCollectionOption(i.Collection))
	if err != nil {
		return fmt.Errorf("[Indexer.NewIndexer] failed to validate collection: %w", err)
	}
	
	if !ok {
		createOpt := i.getCreateCollectionOption()
		if err := i.Client.CreateCollection(ctx, createOpt); err != nil {
			return fmt.Errorf("[Indexer.NewIndexer] failed to create collection: %w", err)
		}
		loadTask, err := i.Client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(i.Collection))
		if err != nil {
			return fmt.Errorf("[Indexer.NewIndexer] failed to load collection: %w", err)
		}
		if err := loadTask.Await(ctx); err != nil {
			return fmt.Errorf("[Indexer.NewIndexer] failed to await the load collection: %w", err)
		}
	}
	return nil
}

// getCreateCollectionOption returns the create collection option.
// If CollectionOpt is nil, it uses the default options with the collection name and dimension.
// Otherwise, it returns the CollectionOpt as is.
func (i *IndexerConfig) getCreateCollectionOption() milvusclient.CreateCollectionOption {
	if i.CollectionOpt == nil {
		i.CollectionOpt = milvusclient.SimpleCreateCollectionOptions(i.Collection, i.Dim)
	}
	return i.CollectionOpt
}
