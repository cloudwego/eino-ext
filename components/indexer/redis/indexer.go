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

package redis

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

type IndexerConfig struct {
	// Client is a Redis client representing a pool of zero or more underlying connections.
	// It's safe for concurrent use by multiple goroutines, which means is okay to pass
	// an existed Client to create a new Indexer component.
	Client *redis.Client
	// KeyPrefix prefix for each key, hset key would be KeyPrefix+Hashes.Key.
	// If not set, make sure each key from DocumentToHashes contains same prefix, for ft.Create requires.
	// see: https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#create-a-vector-index
	KeyPrefix string
	// DocumentToHashes supports customize key, field and value for redis hash.
	// field2EmbeddingValue is field - text pairs, which text will be embedded, then field and embedding will join field2Value.
	// field2Value is field - value pairs for hset.
	// key is hash key, is okay to use document ID if it's unique.
	// Eventually, command will look like: hset $(KeyPrefix+key) field_1 val_1 field_2 val_2 ...
	// Default defaultDocumentToFields.
	DocumentToHashes func(ctx context.Context, doc *schema.Document) (*Hashes, error)
	// BatchSize controls embedding texts size.
	// Default 10.
	BatchSize int `json:"batch_size"`
	// Embedding vectorization method for values need to be embedded from FieldValue.
	Embedding embedding.Embedder
}

type Hashes struct {
	// Key redis hashes key
	Key string
	// Key redis hashes field - val pairs
	Field2Value map[string]FieldValue
}

type FieldValue struct {
	// Value original Value
	Value any
	// EmbedKey if set, Value will be vectorized and saved to es.
	// If Stringify method is provided, Embedding input text will be Stringify(Value).
	// If Stringify method not set, retriever will try to assert Value as string.
	EmbedKey string
	// Stringify converts Value to string
	Stringify func(val any) (string, error)
}

type Indexer struct {
	config *IndexerConfig
}

func NewIndexer(ctx context.Context, config *IndexerConfig) (*Indexer, error) {
	if config.Embedding == nil {
		return nil, fmt.Errorf("[NewIndexer] embedding not provided for redis indexer")
	}

	if config.Client == nil {
		return nil, fmt.Errorf("[NewIndexer] redis client not provided")
	}

	if config.DocumentToHashes == nil {
		config.DocumentToHashes = defaultDocumentToFields
	}

	if config.BatchSize == 0 {
		config.BatchSize = 10
	}

	return &Indexer{
		config: config,
	}, nil
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

	if err = i.pipelineHSet(ctx, docs, options); err != nil {
		return nil, err
	}

	ids = make([]string, 0, len(docs))
	for _, doc := range docs {
		// If you need hash key returned by FieldMapping, set doc.ID with key manually in DocumentToHashes.
		ids = append(ids, doc.ID)
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})

	return ids, nil
}

func (i *Indexer) pipelineHSet(ctx context.Context, docs []*schema.Document, options *indexer.Options) (err error) {
	emb := options.Embedding
	pipeline := i.config.Client.Pipeline()

	var (
		tuples []tuple
		texts  []string
	)

	embAndAdd := func() error {
		var vectors [][]float64

		if len(texts) > 0 {
			if emb == nil {
				return fmt.Errorf("[pipelineHSet] embedding method not provided")
			}

			vectors, err = emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
			if err != nil {
				return fmt.Errorf("[pipelineHSet] embedding failed, %w", err)
			}

			if len(vectors) != len(texts) {
				return fmt.Errorf("[pipelineHSet] invalid vector length, expected=%d, got=%d", len(texts), len(vectors))
			}
		}

		for _, t := range tuples {
			fields := t.fields
			for k, idx := range t.key2Idx {
				fields[k] = vector2Bytes(vectors[idx])
			}

			pipeline.HSet(ctx, i.config.KeyPrefix+t.key, flatten(fields)...)
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

		key := hashes.Key
		field2Value := hashes.Field2Value
		fields := make(map[string]any, len(field2Value))
		embSize := 0
		for k, v := range field2Value {
			fields[k] = v.Value
			if v.EmbedKey != "" {
				embSize++
			}
		}

		if embSize > i.config.BatchSize {
			return fmt.Errorf("[pipelineHSet] embedding size over batch size, batch size=%d, got size=%d",
				i.config.BatchSize, embSize)
		}

		if len(texts)+embSize > i.config.BatchSize {
			if err = embAndAdd(); err != nil {
				return err
			}
		}

		key2Idx := make(map[string]int, embSize)
		for k, v := range field2Value {
			if v.EmbedKey != "" {
				if _, found := fields[v.EmbedKey]; found {
					return fmt.Errorf("[pipelineHSet] duplicate key for value and vector, field=%s", k)
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
						return fmt.Errorf("[pipelineHSet] assert value as string failed, key=%s, emb_key=%s", k, v.EmbedKey)
					}
				}

				key2Idx[v.EmbedKey] = len(texts)
				texts = append(texts, text)
			}
		}

		tuples = append(tuples, tuple{
			key:     key,
			fields:  fields,
			key2Idx: key2Idx,
		})
	}

	if len(tuples) > 0 {
		if err = embAndAdd(); err != nil {
			return err
		}
	}

	if _, err = pipeline.Exec(ctx); err != nil {
		return err
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

const typ = "Redis"

func (i *Indexer) GetType() string {
	return typ
}

func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

func defaultDocumentToFields(ctx context.Context, doc *schema.Document) (*Hashes, error) {
	if doc.ID == "" {
		return nil, fmt.Errorf("[defaultFieldMapping] doc id not set")
	}

	field2Value := map[string]FieldValue{
		defaultReturnFieldContent: {
			Value:     doc.Content,
			EmbedKey:  defaultReturnFieldVectorContent,
			Stringify: nil,
		},
	}
	for k := range doc.MetaData {
		field2Value[k] = FieldValue{
			Value: doc.MetaData[k],
		}
	}

	return &Hashes{
		Key:         doc.ID,
		Field2Value: field2Value,
	}, nil
}

type tuple struct {
	key     string
	fields  map[string]any
	key2Idx map[string]int
}

func flatten(fields map[string]any) []any {
	r := make([]any, 0, len(fields)*2)
	for k := range fields {
		r = append(r, k, fields[k])
	}
	return r
}
