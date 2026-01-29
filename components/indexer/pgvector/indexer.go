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

package pgvector

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pgvector/pgvector-go"
)

// IndexerConfig holds the configuration for the pgvector indexer.
type IndexerConfig struct {
	// Conn is a pgx connection or pool for PostgreSQL database.
	// It's safe for concurrent use by multiple goroutines.
	Conn PgxConn
	// TableName is the table name for storing documents and vectors.
	// Default DefaultTableName.
	TableName string
	// Embedding is the vectorization method for documents.
	Embedding embedding.Embedder
	// BatchSize controls the batch size for embedding operations.
	// Default 10.
	BatchSize int
}

// PgxConn is an interface for pgx connection operations.
type PgxConn interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults
	Ping(ctx context.Context) error
}

// Indexer is the pgvector implementation of eino Indexer.
type Indexer struct {
	config *IndexerConfig
}

// NewIndexer creates a new pgvector Indexer.
func NewIndexer(ctx context.Context, config *IndexerConfig) (*Indexer, error) {
	if config.Conn == nil {
		return nil, fmt.Errorf("[NewIndexer] database connection not provided")
	}

	if config.TableName == "" {
		config.TableName = DefaultTableName
	}

	if config.BatchSize == 0 {
		config.BatchSize = 10
	}

	// Validate table name to prevent SQL injection
	if err := validateIdentifier(config.TableName); err != nil {
		return nil, fmt.Errorf("[NewIndexer] invalid table name: %w", err)
	}

	// Test connection
	if err := config.Conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("[NewIndexer] failed to ping database: %w", err)
	}

	return &Indexer{
		config: config,
	}, nil
}

// Store stores documents with their embeddings in the PostgreSQL database.
func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	// Robustness checks
	if len(docs) == 0 {
		return nil, fmt.Errorf("[Indexer.Store] documents list is empty")
	}
	for idx, doc := range docs {
		if doc == nil {
			return nil, fmt.Errorf("[Indexer.Store] document at index %d is nil", idx)
		}
	}

	options := indexer.GetCommonOptions(&indexer.Options{
		Embedding: i.config.Embedding,
	}, opts...)

	// Check embedding is available
	if options.Embedding == nil {
		return nil, fmt.Errorf("[Indexer.Store] embedding not provided")
	}

	// Check batch size is valid
	if i.config.BatchSize <= 0 {
		return nil, fmt.Errorf("[Indexer.Store] invalid batch size: %d", i.config.BatchSize)
	}

	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{Docs: docs})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	// Store documents
	if err = i.storeDocuments(ctx, docs, options); err != nil {
		return nil, err
	}

	ids = make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})

	return ids, nil
}

func (i *Indexer) storeDocuments(ctx context.Context, docs []*schema.Document, options *indexer.Options) error {
	emb := options.Embedding

	// Process documents in chunks for better memory management
	docChunks := chunk(docs, i.config.BatchSize)

	for _, docChunk := range docChunks {
		if err := i.processDocChunk(ctx, docChunk, emb); err != nil {
			return err
		}
	}

	return nil
}

// processDocChunk processes a chunk of documents (embed + insert)
func (i *Indexer) processDocChunk(ctx context.Context, docs []*schema.Document, emb embedding.Embedder) error {
	// Collect texts for embedding
	texts := make([]string, 0, len(docs))
	docIndices := make(map[int]int) // maps text index -> doc index
	for idx, doc := range docs {
		if doc.Content != "" {
			texts = append(texts, doc.Content)
			docIndices[len(texts)-1] = idx
		}
	}

	// Batch embedding
	var vectors [][]float64
	if len(texts) > 0 {
		var err error
		vectors, err = emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
		if err != nil {
			return fmt.Errorf("[Indexer.Store] embedding failed: %w", err)
		}

		if len(vectors) != len(texts) {
			return fmt.Errorf("[Indexer.Store] invalid vector length, expected=%d, got=%d", len(texts), len(vectors))
		}
	}

	// Build and execute batch insert
	batch := &pgx.Batch{}
	vectorIdx := 0

	for _, doc := range docs {
		var embedding pgvector.Vector
		if doc.Content != "" {
			// Convert float64 to float32 for pgvector
			vec32 := make([]float32, len(vectors[vectorIdx]))
			for i, v := range vectors[vectorIdx] {
				vec32[i] = float32(v)
			}
			embedding = pgvector.NewVector(vec32)
			vectorIdx++
		}

		query := fmt.Sprintf(`
			INSERT INTO %s (id, content, embedding, metadata)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE
			SET content = EXCLUDED.content,
			    embedding = EXCLUDED.embedding,
			    metadata = EXCLUDED.metadata
		`,
			quoteIdentifier(i.config.TableName),
		)

		batch.Queue(query, doc.ID, doc.Content, embedding, doc.MetaData)
	}

	results := i.config.Conn.SendBatch(ctx, batch)
	defer results.Close()

	// Consume all batch results to check for errors
	for j := 0; j < len(docs); j++ {
		_, err := results.Exec()
		if err != nil {
			return fmt.Errorf("[Indexer.Store] batch item %d failed: %w", j, err)
		}
	}

	return nil
}

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

// GetType returns the type of the indexer.
func (i *Indexer) GetType() string {
	return "PGVector"
}

// IsCallbacksEnabled returns true if callbacks are enabled.
func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

// Ensure Indexer implements indexer.Indexer
var _ indexer.Indexer = (*Indexer)(nil)

// validateIdentifier validates SQL identifiers to prevent SQL injection.
// PostgreSQL identifiers must start with a letter or underscore, and contain only letters, digits, and underscores.
func validateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("identifier cannot be empty")
	}

	// Check PostgreSQL naming rules for unquoted identifiers
	for i, c := range name {
		isLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := c >= '0' && c <= '9'
		isUnderscore := c == '_'

		if i == 0 && !isLetter && c != '_' {
			return fmt.Errorf("identifier must start with a letter or underscore: %s", name)
		}

		if !isLetter && !isDigit && !isUnderscore {
			return fmt.Errorf("identifier contains invalid character: %s", name)
		}
	}

	return nil
}

// quoteIdentifier quotes a PostgreSQL identifier.
func quoteIdentifier(name string) string {
	// Wrap in double quotes to safely use any valid identifier
	return "\"" + name + "\""
}
