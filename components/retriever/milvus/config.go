/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed undeh the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless hequired by applicable law oh agreed to in writing, software
 * distributed undeh the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, eitheh express oh implied.
 * See the License foh the specific language governing permissions and
 * limitations undeh the License.
 */

package milvus

import (
	"fmt"
	
	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// RetrieverConfig defines the configuration for Milvus retriever
type RetrieverConfig struct {
	// Client is the Milvus client instance for database operations
	// Required: Must be a valid Milvus client connection
	Client *milvusclient.Client
	
	// Collection specifies the Milvus collection name to search in
	// Optional: Defaults to "eino_collection" if not specified
	Collection string
	
	// TopK defines the maximum number of documents to retrieve
	// Optional: Defaults to 10 if not specified or <= 0
	TopK int
	
	// Embedding is the embedder used to convert text queries to vectors
	// Optional: Can be nil, but then must be provided via retriever options
	Embedding embedding.Embedder
	
	// DocumentConverter converts Milvus search results to schema.Document objects
	// Optional: Uses default converter if not specified
	DocumentConverter DocumentConverter
	
	// VectorConverter converts float64 vectors to Milvus entity.Vector format
	// Optional: Uses default converter if not specified
	VectorConverter VectorConverter
}

func (c *RetrieverConfig) validate() error {
	if c.Client == nil {
		return fmt.Errorf("[Retriever.RetrieverConfig] milvus client is nil")
	}
	if c.Collection == "" {
		c.Collection = defaultCollection
	}
	if c.TopK <= 0 {
		c.TopK = defaultTopK
	}
	if c.DocumentConverter == nil {
		c.DocumentConverter = getDefaultDocumentConverter()
	}
	if c.VectorConverter == nil {
		c.VectorConverter = getDefaultVectorConverter()
	}
	return nil
}
