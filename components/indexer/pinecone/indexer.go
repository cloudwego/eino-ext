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

package pinecone

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	batchSize         = 200
	defaultContentKey = "content"
)

type IndexerConfig struct {
	// Client parameters
	ApiKey     string            // required
	Headers    map[string]string // optional
	Host       string            // optional
	RestClient *http.Client      // optional
	SourceTag  string            // optional

	// Index Connection parameters
	IndexName          string            // required
	Namespace          string            // optional - if not provided the default namespace of "" will be used
	AdditionalMetadata map[string]string // optional

	// Store parameters
	// BatchSize max size for pinecone UpsertVectors and Embedding.
	// Default is 200.
	BatchSize int
	// DocumentToMetadata converts eino document to pinecone Metadata.
	// Metadata payloads must be key-value pairs in a JSON object.
	// Keys must be strings, and values can be one of the following data types:
	// 1. String
	// 2. Number (integer or floating point, gets converted to a 64 bit floating point)
	// 3. Booleans (true, false)
	// 4. List of strings
	// If DocumentToMetadata is not set, will use defaultDocumentToMetadata as default.
	DocumentToMetadata func(ctx context.Context, doc *schema.Document) (map[string]any, error)
	// Embedding vectorization method when dense vector not provided in document extra
	Embedding embedding.Embedder
}

type Indexer struct {
	conf    *IndexerConfig
	idxConn *pinecone.IndexConnection
}

func NewIndexer(ctx context.Context, config *IndexerConfig) (*Indexer, error) {
	clientParams := pinecone.NewClientParams{
		ApiKey:     config.ApiKey,
		Headers:    config.Headers,
		Host:       config.Host,
		RestClient: config.RestClient,
		SourceTag:  config.SourceTag,
	}

	pc, err := pinecone.NewClient(clientParams)
	if err != nil {
		return nil, fmt.Errorf("pinecone: Failed to create Client: %w", err)
	}

	idx, err := pc.DescribeIndex(ctx, config.IndexName)
	if err != nil {
		return nil, fmt.Errorf("pinecone: Failed to describe index %v: %w", config.IndexName, err)
	}

	idxConn, err := pc.Index(pinecone.NewIndexConnParams{
		Host:               idx.Host,
		Namespace:          config.Namespace,
		AdditionalMetadata: config.AdditionalMetadata,
	})
	if err != nil {
		return nil, fmt.Errorf("pinecone: Failed to create IndexConnection for Host: %v: %w", idx.Host, err)
	}

	if config.BatchSize == 0 {
		config.BatchSize = batchSize
	}

	if config.DocumentToMetadata == nil {
		config.DocumentToMetadata = defaultDocumentToMetadata
	}

	return &Indexer{
		conf:    config,
		idxConn: idxConn,
	}, nil
}

func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	options := indexer.GetCommonOptions(&indexer.Options{Embedding: i.conf.Embedding}, opts...)

	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{Docs: docs})

	for _, batch := range chunk(docs, i.conf.BatchSize) {
		in, err := i.makeBatchRequest(ctx, batch, options)
		if err != nil {
			return nil, err
		}

		_, err = i.idxConn.UpsertVectors(ctx, in)
		if err != nil {
			return nil, err
		}

		ids = append(ids, iter(batch, func(t *schema.Document) string { return t.ID })...)
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: ids})

	return ids, nil
}

func (i *Indexer) makeBatchRequest(ctx context.Context, batch []*schema.Document, option *indexer.Options) (
	pvs []*pinecone.Vector, err error) {

	emb := option.Embedding

	var (
		indices []int
		texts   []string
	)

	for idx, doc := range batch {
		dense := doc.DenseVector()
		if dense == nil {
			indices = append(indices, idx)
			texts = append(texts, doc.Content)
		}

		pv := &pinecone.Vector{
			Id:           doc.ID,
			Values:       f64To32(dense),
			SparseValues: toPineconeSparseVector(doc.SparseVector()),
		}

		metadata, err := i.conf.DocumentToMetadata(ctx, doc)
		if err != nil {
			return nil, fmt.Errorf("[makeBatchRequest] DocumentToMetadata failed, %w", err)
		}

		md, err := structpb.NewStruct(metadata)
		if err != nil {
			return nil, err
		}

		pv.Metadata = md
		pvs = append(pvs, pv)
	}

	if len(texts) > 0 {
		if emb == nil {
			return nil, fmt.Errorf("[makeBatchRequest] embedding not provided from config")
		}

		vectors, err := emb.EmbedStrings(i.makeEmbeddingCtx(ctx, emb), texts)
		if err != nil {
			return nil, fmt.Errorf("[makeBatchRequest] embed error, %w", err)
		}

		if len(vectors) != len(indices) {
			return nil, fmt.Errorf("[makeBatchRequest] invalid return length of vector, got=%d, expected=%d",
				len(vectors), len(indices))
		}

		for j, idx := range indices {
			pvs[idx].Values = f64To32(vectors[j])
		}
	}

	return pvs, nil
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

const typ = "Pinecone"

func (i *Indexer) GetType() string {
	return typ
}

func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}

func defaultDocumentToMetadata(ctx context.Context, doc *schema.Document) (map[string]any, error) {
	r := make(map[string]interface{})

	for k := range doc.MetaData {
		v := doc.MetaData[k]
		if isValidType(v) {
			r[k] = v
		}
	}

	r[defaultContentKey] = doc.Content

	return r, nil
}

func toPineconeSparseVector(sparse map[int]float64) *pinecone.SparseValues {
	if sparse == nil {
		return nil
	}
	sv := &pinecone.SparseValues{
		Indices: make([]uint32, 0, len(sparse)),
		Values:  make([]float32, 0, len(sparse)),
	}

	for indices, vector := range sparse {
		sv.Indices = append(sv.Indices, uint32(indices))
		sv.Values = append(sv.Values, float32(vector))
	}

	return sv
}

func isValidType(value interface{}) bool {
	switch value.(type) {
	case string:
		return true
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	case bool:
		return true
	case []string:
		return true
	default:
		return false
	}
}
