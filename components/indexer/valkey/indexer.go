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

package valkey

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

// DocumentType controls how documents are stored in Valkey.
type DocumentType int

const (
	// DocumentTypeHash stores documents as Valkey Hashes (HSET).
	// Index must be created with: FT.CREATE ... ON HASH
	DocumentTypeHash DocumentType = iota
	// DocumentTypeJSON stores documents as JSON (JSON.SET).
	// Requires the Valkey JSON module.
	// Index must be created with: FT.CREATE ... ON JSON
	DocumentTypeJSON
)

// BatchClient is the interface for Valkey clients that support pipeline batch execution.
// Each command is a slice of strings (e.g., ["HSET", key, field1, val1, ...]).
// The caller retains ownership and is responsible for closing the client
// after the Indexer is no longer in use.
type BatchClient interface {
	Exec(ctx context.Context, commands [][]string) ([]any, error)
}

// IndexerConfig configures the Valkey indexer.
type IndexerConfig struct {
	// Client is a Valkey GLIDE client that supports batch Exec.
	// The caller retains ownership and is responsible for closing the client
	// after the Indexer is no longer in use.
	Client BatchClient
	// KeyPrefix is prepended to each document key.
	// Ensure this matches the prefix used in FT.CREATE for the search index.
	KeyPrefix string
	// DocumentType controls storage format. Default: DocumentTypeHash.
	DocumentType DocumentType
	// DocumentToHashes converts a document into a Hashes struct for Hash storage.
	// Only used when DocumentType is DocumentTypeHash.
	// Default: stores doc.Content in "content" field with embedding in "vector_content".
	DocumentToHashes func(ctx context.Context, doc *schema.Document) (*Hashes, error)
	// DocumentToJSON converts a document into a JSON map for JSON storage.
	// Only used when DocumentType is DocumentTypeJSON.
	// Default: stores content, vector, and metadata fields.
	DocumentToJSON func(ctx context.Context, doc *schema.Document, vector []float64) (map[string]any, error)
	// BatchSize controls how many texts are embedded per batch call. Default: 10.
	BatchSize int
	// Embedding is the embedder used to vectorize document content.
	Embedding embedding.Embedder
}

// Hashes represents the key and field-value pairs for a Valkey hash.
type Hashes struct {
	// Key is the hash key (without prefix).
	Key string
	// Field2Value maps field names to their values and embedding configuration.
	Field2Value map[string]FieldValue
}

// FieldValue represents a single field value with optional embedding.
type FieldValue struct {
	// Value is the original field value.
	Value any
	// EmbedKey, if set, indicates Value should be vectorized and stored under this field name.
	EmbedKey string
	// Stringify converts Value to a string for embedding. If nil, Value is asserted as string.
	Stringify func(val any) (string, error)
}

// Indexer implements indexer.Indexer using Valkey hashes or JSON.
type Indexer struct {
	config *IndexerConfig
}

// NewIndexer creates a new Valkey indexer.
func NewIndexer(_ context.Context, config *IndexerConfig) (*Indexer, error) {
	if config.Embedding == nil {
		return nil, fmt.Errorf("[NewIndexer] embedding not provided for valkey indexer")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("[NewIndexer] valkey client not provided")
	}
	cfg := *config // shallow copy to avoid mutating caller's config
	if cfg.DocumentToHashes == nil {
		cfg.DocumentToHashes = defaultDocumentToFields
	}
	if cfg.DocumentToJSON == nil {
		cfg.DocumentToJSON = defaultDocumentToJSON
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	return &Indexer{config: &cfg}, nil
}

// Store writes documents to Valkey in batches. If a batch fails partway through,
// previously successful batches are NOT rolled back. Callers should treat Store
// as at-least-once and ensure document IDs are deterministic for idempotent writes.
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

	switch i.config.DocumentType {
	case DocumentTypeJSON:
		err = i.batchJSONSet(ctx, docs, options)
	default:
		err = i.batchHSet(ctx, docs, options)
	}
	if err != nil {
		return nil, err
	}

	ids = make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})
	return ids, nil
}

type tuple struct {
	key     string
	fields  map[string]string
	key2Idx map[string]int
}

func (i *Indexer) batchHSet(ctx context.Context, docs []*schema.Document, options *indexer.Options) error {
	emb := options.Embedding

	var (
		tuples []tuple
		texts  []string
	)

	embAndStore := func() error {
		var vectors [][]float64
		if len(texts) > 0 {
			if emb == nil {
				return fmt.Errorf("[batchHSet] embedding method not provided")
			}
			var err error
			vectors, err = emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
			if err != nil {
				return fmt.Errorf("[batchHSet] embedding failed, %w", err)
			}
			if len(vectors) != len(texts) {
				return fmt.Errorf("[batchHSet] invalid vector length, expected=%d, got=%d", len(texts), len(vectors))
			}
		}

		commands := make([][]string, 0, len(tuples))
		for _, t := range tuples {
			for k, idx := range t.key2Idx {
				t.fields[k] = string(vector2Bytes(vectors[idx]))
			}
			cmd := []string{"HSET", i.config.KeyPrefix + t.key}
			for k, v := range t.fields {
				cmd = append(cmd, k, v)
			}
			commands = append(commands, cmd)
		}

		if _, err := i.config.Client.Exec(ctx, commands); err != nil {
			return err
		}

		tuples = tuples[:0]
		texts = texts[:0]
		return nil
	}

	for _, doc := range docs {
		hashes, err := i.config.DocumentToHashes(ctx, doc)
		if err != nil {
			return err
		}

		fields := make(map[string]string, len(hashes.Field2Value))
		embSize := 0
		for k, v := range hashes.Field2Value {
			fields[k] = fmt.Sprintf("%v", v.Value)
			if v.EmbedKey != "" {
				embSize++
			}
		}

		if embSize > i.config.BatchSize {
			return fmt.Errorf("[batchHSet] embedding size over batch size, batch size=%d, got size=%d",
				i.config.BatchSize, embSize)
		}

		if len(texts)+embSize > i.config.BatchSize {
			if err = embAndStore(); err != nil {
				return err
			}
		}

		key2Idx := make(map[string]int, embSize)
		for k, v := range hashes.Field2Value {
			if v.EmbedKey != "" {
				if _, found := fields[v.EmbedKey]; found {
					return fmt.Errorf("[batchHSet] duplicate key for value and vector, field=%s", k)
				}
				var text string
				if v.Stringify != nil {
					text, err = v.Stringify(v.Value)
					if err != nil {
						return err
					}
				} else {
					var ok bool
					text, ok = v.Value.(string)
					if !ok {
						return fmt.Errorf("[batchHSet] assert value as string failed, key=%s, emb_key=%s", k, v.EmbedKey)
					}
				}
				key2Idx[v.EmbedKey] = len(texts)
				texts = append(texts, text)
			}
		}

		tuples = append(tuples, tuple{
			key:     hashes.Key,
			fields:  fields,
			key2Idx: key2Idx,
		})
	}

	if len(tuples) > 0 {
		if err := embAndStore(); err != nil {
			return err
		}
	}

	return nil
}

type jsonDoc struct {
	doc  *schema.Document
	text string
}

func (i *Indexer) batchJSONSet(ctx context.Context, docs []*schema.Document, options *indexer.Options) error {
	emb := options.Embedding
	if emb == nil {
		return fmt.Errorf("[batchJSONSet] embedding method not provided")
	}

	// Collect texts for embedding in batches
	var (
		pending []jsonDoc
		texts   []string
	)

	flushBatch := func() error {
		if len(texts) == 0 {
			return nil
		}

		vectors, err := emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
		if err != nil {
			return fmt.Errorf("[batchJSONSet] embedding failed, %w", err)
		}
		if len(vectors) != len(texts) {
			return fmt.Errorf("[batchJSONSet] invalid vector length, expected=%d, got=%d", len(texts), len(vectors))
		}

		commands := make([][]string, 0, len(pending))
		for idx, p := range pending {
			jsonMap, err := i.config.DocumentToJSON(ctx, p.doc, vectors[idx])
			if err != nil {
				return fmt.Errorf("[batchJSONSet] DocumentToJSON failed for key=%s: %w", p.doc.ID, err)
			}

			jsonBytes, err := json.Marshal(jsonMap)
			if err != nil {
				return fmt.Errorf("[batchJSONSet] json marshal failed, %w", err)
			}

			commands = append(commands, []string{"JSON.SET", i.config.KeyPrefix + p.doc.ID, "$", string(jsonBytes)})
		}

		if _, err := i.config.Client.Exec(ctx, commands); err != nil {
			return err
		}

		pending = pending[:0]
		texts = texts[:0]
		return nil
	}

	for _, doc := range docs {
		if doc.ID == "" {
			return fmt.Errorf("[batchJSONSet] document ID must not be empty")
		}
		if len(texts) >= i.config.BatchSize {
			if err := flushBatch(); err != nil {
				return err
			}
		}
		pending = append(pending, jsonDoc{doc: doc, text: doc.Content})
		texts = append(texts, doc.Content)
	}

	return flushBatch()
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

const typ = "Valkey"

func (i *Indexer) GetType() string {
	return typ
}

func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

func defaultDocumentToFields(_ context.Context, doc *schema.Document) (*Hashes, error) {
	if doc.ID == "" {
		return nil, fmt.Errorf("[defaultDocumentToFields] doc id not set")
	}
	field2Value := map[string]FieldValue{
		defaultReturnFieldContent: {
			Value:    doc.Content,
			EmbedKey: defaultReturnFieldVectorContent,
		},
	}
	for k, v := range doc.MetaData {
		if k == defaultReturnFieldContent || k == defaultReturnFieldVectorContent {
			return nil, fmt.Errorf("[defaultDocumentToFields] metadata key %q conflicts with reserved field", k)
		}
		field2Value[k] = FieldValue{Value: v}
	}
	return &Hashes{Key: doc.ID, Field2Value: field2Value}, nil
}

func defaultDocumentToJSON(_ context.Context, doc *schema.Document, vector []float64) (map[string]any, error) {
	if doc.ID == "" {
		return nil, fmt.Errorf("[defaultDocumentToJSON] doc id not set")
	}
	m := map[string]any{
		defaultReturnFieldContent:       doc.Content,
		defaultReturnFieldVectorContent: vector,
	}
	for k, v := range doc.MetaData {
		if k == defaultReturnFieldContent || k == defaultReturnFieldVectorContent {
			return nil, fmt.Errorf("[defaultDocumentToJSON] metadata key %q conflicts with reserved field", k)
		}
		m[k] = v
	}
	return m, nil
}

func vector2Bytes(vector []float64) []byte {
	bytes := make([]byte, len(vector)*4)
	for i, v := range vector {
		binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(float32(v)))
	}
	return bytes
}
