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
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// RetrieverConfig holds the configuration for the pgvector retriever.
type RetrieverConfig struct {
	// Conn is a pgx connection or pool for PostgreSQL database.
	Conn PgxConn
	// TableName is the table name for storing documents and vectors.
	// Default DefaultTableName.
	TableName string
	// DistanceFunction is the distance function for similarity search.
	// Default DistanceCosine.
	DistanceFunction DistanceFunction
	// TopK is the maximum number of documents to retrieve.
	// Default 5.
	TopK int
	// Embedding is the vectorization method for queries.
	Embedding embedding.Embedder
	// ScoreThreshold is the optional maximum distance for filtering results.
	// If set, only results with distance <= threshold are returned.
	ScoreThreshold *float64
}

// PgxConn is an interface for pgx connection operations.
type PgxConn interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Ping(ctx context.Context) error
}

// Retriever is the pgvector implementation of eino Retriever.
type Retriever struct {
	config *RetrieverConfig
}

// NewRetriever creates a new pgvector Retriever.
func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
	if config.Embedding == nil {
		return nil, fmt.Errorf("[NewRetriever] embedding not provided for pgvector retriever")
	}

	if config.Conn == nil {
		return nil, fmt.Errorf("[NewRetriever] database connection not provided")
	}

	if config.TableName == "" {
		config.TableName = DefaultTableName
	}

	if config.TopK == 0 {
		config.TopK = 5
	}

	if config.DistanceFunction == "" {
		config.DistanceFunction = DistanceCosine
	}

	if err := config.DistanceFunction.Validate(); err != nil {
		return nil, fmt.Errorf("[NewRetriever] invalid distance function: %w", err)
	}

	// Validate table name to prevent SQL injection
	if err := validateIdentifier(config.TableName); err != nil {
		return nil, fmt.Errorf("[NewRetriever] invalid table name: %w", err)
	}

	// Test connection
	if err := config.Conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("[NewRetriever] failed to ping database: %w", err)
	}

	return &Retriever{
		config: config,
	}, nil
}

// Retrieve retrieves similar documents based on the query.
func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK:           &r.config.TopK,
		ScoreThreshold: r.config.ScoreThreshold,
		Embedding:      r.config.Embedding,
	}, opts...)

	io := retriever.GetImplSpecificOptions(&implOptions{
		DistanceFunction: r.config.DistanceFunction,
	}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *co.TopK,
		ScoreThreshold: co.ScoreThreshold,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[pgvector retriever] embedding not provided")
	}

	// Embed the query
	vectors, err := emb.EmbedStrings(r.makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, fmt.Errorf("[Retrieve] failed to embed query: %w", err)
	}

	if len(vectors) != 1 {
		return nil, fmt.Errorf("[Retrieve] invalid vector length, expected=1, got=%d", len(vectors))
	}

	queryVector := vectors[0]

	// Convert float64 to float32 for pgvector
	vec32 := make([]float32, len(queryVector))
	for i, v := range queryVector {
		vec32[i] = float32(v)
	}
	pgVec := pgvector.NewVector(vec32)

	// Build and execute the search query
	searchQuery := r.buildSearchQuery(io.WhereClause, co.ScoreThreshold)

	args := []any{
		pgVec,
		*co.TopK,
	}

	rows, err := r.config.Conn.Query(ctx, searchQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("[Retrieve] query failed: %w", err)
	}
	defer rows.Close()

	// Parse results
	docs = make([]*schema.Document, 0)
	for rows.Next() {
		var (
			id       string
			content  string
			metadata map[string]any
			distance float64
		)

		err = rows.Scan(&id, &content, &metadata, &distance)
		if err != nil {
			return nil, fmt.Errorf("[Retrieve] failed to scan row: %w", err)
		}

		doc := &schema.Document{
			ID:       id,
			Content:  content,
			MetaData: metadata,
		}

		// Calculate score (1 - distance for cosine, normalized distance for others)
		score := r.calculateScore(distance)
		doc.WithScore(score)
		doc.WithDenseVector(queryVector)

		docs = append(docs, doc)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("[Retrieve] rows error: %w", rows.Err())
	}

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})

	return docs, nil
}

func (r *Retriever) buildSearchQuery(whereClause string, scoreThreshold *float64) string {
	distanceOp := r.config.DistanceFunction.Operator()

	query := fmt.Sprintf(`
		SELECT id, content, metadata, (embedding %s $1) AS distance
		FROM %s`,
		distanceOp,
		quoteIdentifier(r.config.TableName),
	)

	// Add WHERE clause
	conditions := []string{}
	if whereClause != "" {
		conditions = append(conditions, whereClause)
	}
	if scoreThreshold != nil {
		// For cosine: distance < 1 - threshold
		// For L2/IP: distance < threshold
		thresholdCondition := fmt.Sprintf("(embedding %s $1) < %f",
			distanceOp,
			r.calculateThresholdDistance(*scoreThreshold))
		conditions = append(conditions, thresholdCondition)
	}

	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}

	query += " ORDER BY distance ASC LIMIT $2"

	return query
}

func (r *Retriever) calculateScore(distance float64) float64 {
	switch r.config.DistanceFunction {
	case DistanceCosine:
		// For cosine distance: score = 1 - distance
		// Cosine similarity = 1 - cosine distance
		return 1 - distance
	case DistanceL2, DistanceIP:
		// For L2 and IP: use inverse as score
		if distance == 0 {
			return 1.0
		}
		return 1.0 / (1.0 + distance)
	default:
		return 1 - distance
	}
}

func (r *Retriever) calculateThresholdDistance(scoreThreshold float64) float64 {
	switch r.config.DistanceFunction {
	case DistanceCosine:
		// For cosine: distance = 1 - score
		return 1 - scoreThreshold
	case DistanceL2, DistanceIP:
		// For L2 and IP: distance is already in the right scale
		return scoreThreshold
	default:
		return scoreThreshold
	}
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

// GetType returns the type of the retriever.
func (r *Retriever) GetType() string {
	return "PGVector"
}

// IsCallbacksEnabled returns true if callbacks are enabled.
func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

// Ensure Retriever implements retriever.Retriever
var _ retriever.Retriever = (*Retriever)(nil)

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
