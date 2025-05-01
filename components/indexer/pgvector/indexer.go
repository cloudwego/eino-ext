/*
 * Copyright 2024 CloudWeGo Authors
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
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/lib/pq"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultBatchSize = 10
)

// VectorType represents the vector types supported by PGVector
type VectorType string

// VectorTypeVector standard vector type, supports up to 2000 dimensions
const VectorTypeVector VectorType = "vector"

// VectorTypeHalfvec half-precision vector type, supports up to 4000 dimensions
const VectorTypeHalfvec VectorType = "halfvec"

// VectorTypeBit bit vector type, supports up to 64000 dimensions
const VectorTypeBit VectorType = "bit"

// VectorTypeSparsevec sparse vector type, supports up to 1000 non-zero elements
const VectorTypeSparsevec VectorType = "sparsevec"

// IndexerConfig configures the PGVector indexer
type IndexerConfig struct {
	// PostgreSQL connection information
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"db_name"`
	SSLMode  string `json:"ssl_mode"`

	// Table name
	TableName string `json:"table_name"`

	// Vector dimension
	Dimension int `json:"dimension"`

	// Vector type
	VectorType VectorType `json:"vector_type"`

	// Batch insert size
	BatchSize int `json:"batch_size"`

	// Vectorization configuration
	Embedding embedding.Embedder `json:"embedding"`
}

// Indexer implements the PGVector indexer
type Indexer struct {
	config *IndexerConfig
	db     *sql.DB
}

// getSuitableVectorType automatically selects the appropriate vector type based on dimension
func getSuitableVectorType(dimension int) VectorType {
	switch {
	case dimension <= 2000:
		return VectorTypeVector
	case dimension <= 4000:
		return VectorTypeHalfvec
	case dimension <= 64000:
		return VectorTypeBit
	default:
		return VectorTypeSparsevec
	}
}

// NewIndexer creates a new PGVector indexer
func NewIndexer(ctx context.Context, config *IndexerConfig) (*Indexer, error) {
	if config.Embedding == nil {
		return nil, fmt.Errorf("[PGVectorIndexer] embedding is required")
	}

	if config.BatchSize == 0 {
		config.BatchSize = defaultBatchSize
	}

	// Set default vector type
	if config.VectorType == "" {
		config.VectorType = getSuitableVectorType(config.Dimension)
	} else {
		// If the specified vector type is not supported for the current dimension, automatically switch to a suitable type
		if err := validateVectorConfig(config.VectorType, config.Dimension); err != nil {
			newType := getSuitableVectorType(config.Dimension)
			fmt.Printf("[PGVectorIndexer] Warning: Automatically switching vector type from %s to %s to support %d dimensions\n",
				config.VectorType, newType, config.Dimension)
			config.VectorType = newType
		}
	}

	// Build connection string
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode)

	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("[PGVectorIndexer] failed to connect to database: %w", err)
	}

	// Test connection
	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("[PGVectorIndexer] failed to ping database: %w", err)
	}

	// Create indexer instance
	i := &Indexer{
		config: config,
		db:     db,
	}

	// Ensure table exists
	if err = i.ensureTable(ctx); err != nil {
		return nil, err
	}

	return i, nil
}

// validateVectorConfig checks if the vector configuration is valid
func validateVectorConfig(vectorType VectorType, dimension int) error {
	switch vectorType {
	case VectorTypeVector:
		if dimension > 2000 {
			return fmt.Errorf("[PGVectorIndexer] vector type 'vector' supports up to 2000 dimensions, got %d", dimension)
		}
	case VectorTypeHalfvec:
		if dimension > 4000 {
			return fmt.Errorf("[PGVectorIndexer] vector type 'halfvec' supports up to 4000 dimensions, got %d", dimension)
		}
	case VectorTypeBit:
		if dimension > 64000 {
			return fmt.Errorf("[PGVectorIndexer] vector type 'bit' supports up to 64000 dimensions, got %d", dimension)
		}
	case VectorTypeSparsevec:
		if dimension > 1000 {
			return fmt.Errorf("[PGVectorIndexer] vector type 'sparsevec' supports up to 1000 non-zero elements, got %d", dimension)
		}
	default:
		return fmt.Errorf("[PGVectorIndexer] unsupported vector type: %s", vectorType)
	}
	return nil
}

// ensureTable ensures that the vector table exists
func (i *Indexer) ensureTable(ctx context.Context) error {
	// Check if pgvector extension is installed
	_, err := i.db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector;")
	if err != nil {
		return fmt.Errorf("[PGVectorIndexer] failed to create vector extension: %w", err)
	}

	// Create table
	createTableSQL := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		%s TEXT PRIMARY KEY,
		%s TEXT NOT NULL,
		%s JSONB,
		%s %s(%d) NOT NULL
	);
	`,
		i.config.TableName,
		defaultFieldID,
		defaultFieldContent,
		defaultFieldMetadata,
		defaultFieldVector,
		i.config.VectorType,
		i.config.Dimension,
	)

	_, err = i.db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("[PGVectorIndexer] failed to create table: %w", err)
	}

	return nil
}

// Store stores documents to PGVector
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

	// Get embedding vectors
	emb := options.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[PGVectorIndexer] embedding not provided")
	}

	// Extract document content
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.Content)
	}

	// Generate vector embeddings
	vectors, err := emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
	if err != nil {
		return nil, fmt.Errorf("[PGVectorIndexer] embedding failed: %w", err)
	}

	if len(vectors) != len(docs) {
		return nil, fmt.Errorf("[PGVectorIndexer] embedding result length mismatch: need %d, got %d", len(docs), len(vectors))
	}

	// Batch insert documents
	ids = make([]string, 0, len(docs))
	for j := 0; j < len(docs); j += i.config.BatchSize {
		end := j + i.config.BatchSize
		if end > len(docs) {
			end = len(docs)
		}

		batchDocs := docs[j:end]
		batchVectors := vectors[j:end]

		batchIDs, err := i.batchInsert(ctx, batchDocs, batchVectors)
		if err != nil {
			return nil, err
		}

		ids = append(ids, batchIDs...)
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})

	return ids, nil
}

// batchInsert batch inserts documents to PGVector
func (i *Indexer) batchInsert(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]string, error) {
	if len(docs) == 0 {
		return []string{}, nil
	}

	// Begin transaction
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("[PGVectorIndexer] failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Build insert statement
	valueStrings := make([]string, 0, len(docs))
	valueArgs := make([]interface{}, 0, len(docs)*4)
	ids := make([]string, 0, len(docs))

	for idx, doc := range docs {
		ids = append(ids, doc.ID)

		// Process metadata
		metadata, err := json.Marshal(doc.MetaData)
		if err != nil {
			return nil, fmt.Errorf("[PGVectorIndexer] failed to marshal metadata: %w", err)
		}

		// Process vector
		vectorStr := i.formatVector(vectors[idx])

		placeholderOffset := idx * 4
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d)",
			placeholderOffset+1, placeholderOffset+2, placeholderOffset+3, placeholderOffset+4))

		valueArgs = append(valueArgs, doc.ID, doc.Content, string(metadata), vectorStr)
	}

	// Build complete SQL
	sql := fmt.Sprintf(
		"INSERT INTO %s (%s, %s, %s, %s) VALUES %s ON CONFLICT (%s) DO UPDATE SET %s = EXCLUDED.%s, %s = EXCLUDED.%s, %s = EXCLUDED.%s",
		i.config.TableName,
		defaultFieldID, defaultFieldContent, defaultFieldMetadata, defaultFieldVector,
		strings.Join(valueStrings, ", "),
		defaultFieldID,
		defaultFieldContent, defaultFieldContent,
		defaultFieldMetadata, defaultFieldMetadata,
		defaultFieldVector, defaultFieldVector,
	)

	// Execute insert
	_, err = tx.ExecContext(ctx, sql, valueArgs...)
	if err != nil {
		return nil, fmt.Errorf("[PGVectorIndexer] failed to insert documents: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("[PGVectorIndexer] failed to commit transaction: %w", err)
	}

	return ids, nil
}

// formatVector formats a float array as PGVector format
func (i *Indexer) formatVector(vector []float64) string {
	strValues := make([]string, len(vector))
	for i, v := range vector {
		strValues[i] = fmt.Sprintf("%f", v)
	}
	return fmt.Sprintf("[%s]", strings.Join(strValues, ","))
}

// makeEmbeddingCtx creates embedding context
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

// GetType returns the indexer type
func (i *Indexer) GetType() string {
	return typ
}

// IsCallbacksEnabled returns whether callbacks are enabled
func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

// Close closes the database connection
func (i *Indexer) Close() error {
	if i.db != nil {
		return i.db.Close()
	}
	return nil
}
