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
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

// Config contains the configuration for the SQLiteVec retriever.
type Config struct {
	// DB is an opened SQLite database handle. The retriever does not close it.
	DB *sql.DB
	// DocumentTable stores document IDs, content, and JSON metadata.
	DocumentTable string
	// VectorTable stores dense vectors in a sqlite-vec vec0 virtual table.
	VectorTable string
	// VectorDim is the fixed embedding dimension.
	VectorDim int
	// TopK is the default maximum number of retrieved documents.
	TopK int
	// Embedding generates dense vectors for queries.
	Embedding embedding.Embedder
}

// Retriever retrieves documents from SQLiteVec by vector similarity.
type Retriever struct {
	db            *sql.DB
	documentTable string
	vectorTable   string
	vectorDim     int
	topK          int
	embedding     embedding.Embedder
}

// NewRetriever creates a SQLiteVec retriever.
func NewRetriever(ctx context.Context, config *Config) (*Retriever, error) {
	if config == nil {
		return nil, fmt.Errorf("[NewRetriever] config is nil")
	}
	if config.DB == nil {
		return nil, fmt.Errorf("[NewRetriever] db not provided")
	}
	if config.Embedding == nil {
		return nil, fmt.Errorf("[NewRetriever] embedding not provided for sqlitevec retriever")
	}
	if config.VectorDim <= 0 {
		return nil, fmt.Errorf("[NewRetriever] vector dim must be positive")
	}

	documentTable, err := normalizeTableName(config.DocumentTable, defaultDocumentTable)
	if err != nil {
		return nil, fmt.Errorf("[NewRetriever] invalid document table: %w", err)
	}
	vectorTable, err := normalizeTableName(config.VectorTable, defaultVectorTable)
	if err != nil {
		return nil, fmt.Errorf("[NewRetriever] invalid vector table: %w", err)
	}

	topK := config.TopK
	if topK == 0 {
		topK = defaultTopK
	}
	if topK < 0 {
		return nil, fmt.Errorf("[NewRetriever] top k must be non-negative")
	}

	return &Retriever{
		db:            config.DB,
		documentTable: documentTable,
		vectorTable:   vectorTable,
		vectorDim:     config.VectorDim,
		topK:          topK,
		embedding:     config.Embedding,
	}, nil
}

// Retrieve retrieves documents by query vector similarity.
func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK:      &r.topK,
		Embedding: r.embedding,
	}, opts...)
	io := retriever.GetImplSpecificOptions(&implOptions{}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           dereferenceOrZero(co.TopK),
		ScoreThreshold: co.ScoreThreshold,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if co.Embedding == nil {
		return nil, fmt.Errorf("[Retrieve] embedding not provided")
	}
	if co.TopK == nil || *co.TopK <= 0 {
		return nil, fmt.Errorf("[Retrieve] top k must be positive")
	}

	vectors, err := co.Embedding.EmbedStrings(r.makeEmbeddingCtx(ctx, co.Embedding), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[Retrieve] embedding failed: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("[Retrieve] invalid embedding result length, expected=1, got=%d", len(vectors))
	}

	queryVector, err := vectorToJSON(vectors[0], r.vectorDim)
	if err != nil {
		return nil, fmt.Errorf("[Retrieve] %w", err)
	}

	docs, err = r.search(ctx, queryVector, *co.TopK, co.ScoreThreshold, io.MaxDistance)
	if err != nil {
		return nil, err
	}

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})
	return docs, nil
}

func (r *Retriever) search(ctx context.Context, queryVector string, topK int, scoreThreshold, maxDistance *float64) ([]*schema.Document, error) {
	querySQL := fmt.Sprintf(`SELECT
	d.doc_id,
	d.content,
	d.metadata_json,
	v.distance
FROM %s AS v
JOIN %s AS d ON d.id = v.rowid
WHERE v.embedding MATCH ? AND v.k = ?
ORDER BY v.distance`, r.vectorTable, r.documentTable)

	rows, err := r.db.QueryContext(ctx, querySQL, queryVector, topK)
	if err != nil {
		return nil, fmt.Errorf("[Retrieve] sqlitevec search failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	docs := make([]*schema.Document, 0, topK)
	for rows.Next() {
		var (
			id           string
			content      string
			metadataJSON string
			distance     float64
		)
		if err := rows.Scan(&id, &content, &metadataJSON, &distance); err != nil {
			return nil, fmt.Errorf("[Retrieve] scan row failed: %w", err)
		}

		if maxDistance != nil && distance > *maxDistance {
			continue
		}

		score := distanceToScore(distance)
		if scoreThreshold != nil && score < *scoreThreshold {
			continue
		}

		metadata, err := metadataFromJSON(metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("[Retrieve] unmarshal metadata failed: %w", err)
		}
		metadata[metadataDistanceKey] = distance

		doc := &schema.Document{
			ID:       id,
			Content:  content,
			MetaData: metadata,
		}
		doc.WithScore(score)
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("[Retrieve] iterate rows failed: %w", err)
	}

	return docs, nil
}

func distanceToScore(distance float64) float64 {
	return 1 / (1 + distance)
}

func dereferenceOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func (r *Retriever) makeEmbeddingCtx(ctx context.Context, emb embedding.Embedder) context.Context {
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
func (r *Retriever) GetType() string {
	return typ
}

// IsCallbacksEnabled returns whether callbacks are enabled.
func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

var _ retriever.Retriever = (*Retriever)(nil)
