/*
 * Copyright 2026 CloudWeGo Authors
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

package sqlitevec

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

// Config contains the configuration for the SQLiteVec indexer.
type Config struct {
	// DB is an opened SQLite database handle. The indexer does not close it.
	DB *sql.DB
	// DocumentTable stores document IDs, content, and JSON metadata.
	DocumentTable string
	// VectorTable stores dense vectors in a sqlite-vec vec0 virtual table.
	VectorTable string
	// VectorDim is the fixed embedding dimension.
	VectorDim int
	// BatchSize controls how many document contents are embedded at once.
	BatchSize int
	// Embedding generates dense vectors for documents.
	Embedding embedding.Embedder
	// DisableAutoCreate disables automatic table creation when set.
	DisableAutoCreate bool
}

// Indexer stores documents and their embeddings in SQLiteVec.
type Indexer struct {
	db            *sql.DB
	documentTable string
	vectorTable   string
	vectorDim     int
	batchSize     int
	embedding     embedding.Embedder
}

// NewIndexer creates a SQLiteVec indexer.
func NewIndexer(ctx context.Context, config *Config) (*Indexer, error) {
	if config == nil {
		return nil, fmt.Errorf("[NewIndexer] config is nil")
	}
	if config.DB == nil {
		return nil, fmt.Errorf("[NewIndexer] db not provided")
	}
	if config.Embedding == nil {
		return nil, fmt.Errorf("[NewIndexer] embedding not provided for sqlitevec indexer")
	}
	if config.VectorDim <= 0 {
		return nil, fmt.Errorf("[NewIndexer] vector dim must be positive")
	}

	documentTable, err := normalizeTableName(config.DocumentTable, defaultDocumentTable)
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] invalid document table: %w", err)
	}
	vectorTable, err := normalizeTableName(config.VectorTable, defaultVectorTable)
	if err != nil {
		return nil, fmt.Errorf("[NewIndexer] invalid vector table: %w", err)
	}

	batchSize := config.BatchSize
	if batchSize == 0 {
		batchSize = defaultBatchSize
	}
	if batchSize < 0 {
		return nil, fmt.Errorf("[NewIndexer] batch size must be non-negative")
	}

	idx := &Indexer{
		db:            config.DB,
		documentTable: documentTable,
		vectorTable:   vectorTable,
		vectorDim:     config.VectorDim,
		batchSize:     batchSize,
		embedding:     config.Embedding,
	}

	if !config.DisableAutoCreate {
		if err := idx.ensureSchema(ctx); err != nil {
			return nil, fmt.Errorf("[NewIndexer] failed to ensure schema: %w", err)
		}
	}

	return idx, nil
}

// Store stores documents and returns their IDs.
func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	options := indexer.GetCommonOptions(&indexer.Options{
		Embedding: i.embedding,
	}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{Docs: docs})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if len(docs) == 0 {
		ids = []string{}
		callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})
		return ids, nil
	}

	if options.Embedding == nil {
		return nil, fmt.Errorf("[Store] embedding not provided")
	}

	if err = i.storeBatches(ctx, docs, options.Embedding); err != nil {
		return nil, err
	}

	ids = make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})
	return ids, nil
}

func (i *Indexer) ensureSchema(ctx context.Context) error {
	documentSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		doc_id TEXT NOT NULL UNIQUE,
		content TEXT NOT NULL,
		metadata_json TEXT NOT NULL DEFAULT '{}'
	)`, i.documentTable)
	if _, err := i.db.ExecContext(ctx, documentSQL); err != nil {
		return err
	}

	vectorSQL := fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS %s USING vec0(
		embedding float[%d]
	)`, i.vectorTable, i.vectorDim)
	_, err := i.db.ExecContext(ctx, vectorSQL)
	return err
}

func (i *Indexer) storeBatches(ctx context.Context, docs []*schema.Document, emb embedding.Embedder) error {
	for start := 0; start < len(docs); start += i.batchSize {
		end := start + i.batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[start:end]
		texts := make([]string, 0, len(batch))
		for _, doc := range batch {
			if doc == nil {
				return fmt.Errorf("[Store] document is nil")
			}
			if doc.ID == "" {
				return fmt.Errorf("[Store] document id is empty")
			}
			texts = append(texts, doc.Content)
		}

		vectors, err := emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
		if err != nil {
			return fmt.Errorf("[Store] embedding failed: %w", err)
		}
		if len(vectors) != len(batch) {
			return fmt.Errorf("[Store] invalid embedding result length, expected=%d, got=%d", len(batch), len(vectors))
		}

		if err := i.writeBatch(ctx, batch, vectors); err != nil {
			return err
		}
	}
	return nil
}

func (i *Indexer) writeBatch(ctx context.Context, docs []*schema.Document, vectors [][]float64) error {
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	upsertDocumentSQL := fmt.Sprintf(`INSERT INTO %s (doc_id, content, metadata_json)
VALUES (?, ?, ?)
ON CONFLICT(doc_id) DO UPDATE SET
	content = excluded.content,
	metadata_json = excluded.metadata_json`, i.documentTable)
	selectRowIDSQL := fmt.Sprintf(`SELECT id FROM %s WHERE doc_id = ?`, i.documentTable)
	deleteVectorSQL := fmt.Sprintf(`DELETE FROM %s WHERE rowid = ?`, i.vectorTable)
	insertVectorSQL := fmt.Sprintf(`INSERT INTO %s (rowid, embedding) VALUES (?, ?)`, i.vectorTable)

	for idx, doc := range docs {
		vectorJSON, err := vectorToJSON(vectors[idx], i.vectorDim)
		if err != nil {
			return fmt.Errorf("[Store] doc id=%s: %w", doc.ID, err)
		}
		metadataJSON, err := metadataToJSON(doc.MetaData)
		if err != nil {
			return fmt.Errorf("[Store] doc id=%s: marshal metadata failed: %w", doc.ID, err)
		}

		if _, err := tx.ExecContext(ctx, upsertDocumentSQL, doc.ID, doc.Content, metadataJSON); err != nil {
			return fmt.Errorf("[Store] upsert document failed: %w", err)
		}

		var rowID int64
		if err := tx.QueryRowContext(ctx, selectRowIDSQL, doc.ID).Scan(&rowID); err != nil {
			return fmt.Errorf("[Store] select document rowid failed: %w", err)
		}

		if _, err := tx.ExecContext(ctx, deleteVectorSQL, rowID); err != nil {
			return fmt.Errorf("[Store] delete old vector failed: %w", err)
		}

		if _, err := tx.ExecContext(ctx, insertVectorSQL, rowID, vectorJSON); err != nil {
			return fmt.Errorf("[Store] insert vector failed: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[Store] commit failed: %w", err)
	}
	committed = true
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

// GetType returns the component type.
func (i *Indexer) GetType() string {
	return typ
}

// IsCallbacksEnabled returns whether callbacks are enabled.
func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

var _ indexer.Indexer = (*Indexer)(nil)
