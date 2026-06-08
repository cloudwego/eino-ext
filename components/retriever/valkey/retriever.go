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

package valkey

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/valkey-io/valkey-glide/go/v2/models"
	"github.com/valkey-io/valkey-glide/go/v2/options"
	"github.com/valkey-io/valkey-glide/go/v2/servermodules/glideft"
)

// SearchClient is the interface for Valkey clients that support FT.SEARCH.
// Satisfied by *glide.Client.
type SearchClient interface {
	CustomCommand(ctx context.Context, args []string) (any, error)
}

// RetrieverConfig configures the Valkey retriever.
type RetrieverConfig struct {
	// Client is a Valkey GLIDE client that supports FT.SEARCH via CustomCommand.
	Client SearchClient
	// Index is the name of the Valkey Search index to query.
	Index string
	// VectorField is the vector field name in the index schema.
	// Default: "vector_content"
	VectorField string
	// DistanceThreshold enables vector range queries when set.
	// If nil, KNN vector search is used instead.
	// NOTE: VECTOR_RANGE requires Valkey Search 1.3/2.0+ which is not yet released.
	// Using this option with current Valkey Search versions will result in an error.
	DistanceThreshold *float64
	// Dialect is the query dialect version. Default: 2.
	Dialect int
	// ReturnFields limits which fields are returned from search results.
	// Default: []string{"content", "vector_content"}
	ReturnFields []string
	// DocumentConverter converts a search result document to an eino Document.
	// Default: built-in parser using ReturnFields.
	DocumentConverter func(ctx context.Context, doc models.FtSearchDocument) (*schema.Document, error)
	// TopK limits the number of results. Default: 5.
	TopK int
	// Embedding is the embedder used to vectorize the query string.
	Embedding embedding.Embedder
}

// Retriever implements retriever.Retriever using Valkey Search.
type Retriever struct {
	config *RetrieverConfig
}

// NewRetriever creates a new Valkey retriever.
func NewRetriever(_ context.Context, config *RetrieverConfig) (*Retriever, error) {
	if config.Embedding == nil {
		return nil, fmt.Errorf("[NewRetriever] embedding not provided for valkey retriever")
	}
	if config.Index == "" {
		return nil, fmt.Errorf("[NewRetriever] valkey index not provided")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("[NewRetriever] valkey client not provided")
	}
	if config.Dialect < 2 {
		config.Dialect = 2
	}
	if config.TopK == 0 {
		config.TopK = 5
	}
	if config.VectorField == "" {
		config.VectorField = defaultReturnFieldVectorContent
	}
	if len(config.ReturnFields) == 0 {
		config.ReturnFields = []string{
			defaultReturnFieldContent,
			defaultReturnFieldVectorContent,
		}
	}
	if config.DocumentConverter == nil {
		config.DocumentConverter = defaultResultParser(config.ReturnFields)
	}
	return &Retriever{config: config}, nil
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	co := retriever.GetCommonOptions(&retriever.Options{
		Index:          &r.config.Index,
		TopK:           &r.config.TopK,
		ScoreThreshold: r.config.DistanceThreshold,
		Embedding:      r.config.Embedding,
	}, opts...)
	io := retriever.GetImplSpecificOptions(&implOptions{}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *co.TopK,
		Filter:         io.FilterQuery,
		ScoreThreshold: co.ScoreThreshold,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	emb := co.Embedding
	if emb == nil {
		return nil, fmt.Errorf("[valkey retriever] embedding not provided")
	}

	vectors, err := emb.EmbedStrings(r.makeEmbeddingCtx(ctx, emb), []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("[valkey retriever] invalid return length of vector, got=%d, expected=1", len(vectors))
	}

	vecBytes := vector2Bytes(vectors[0])

	var searchQuery string
	params := []options.FtSearchParam{
		{Key: paramVector, Value: string(vecBytes)},
	}

	if r.config.DistanceThreshold != nil {
		params = append(params, options.FtSearchParam{
			Key:   paramDistanceThreshold,
			Value: fmt.Sprintf("%f", *r.config.DistanceThreshold),
		})
		baseQuery := fmt.Sprintf("@%s:[VECTOR_RANGE $%s $%s]", r.config.VectorField, paramDistanceThreshold, paramVector)
		if io.FilterQuery != "" {
			baseQuery = io.FilterQuery + " " + baseQuery
		}
		searchQuery = fmt.Sprintf("%s=>{$yield_distance_as: %s}", baseQuery, SortByDistanceAttributeName)
	} else {
		filter := "*"
		if io.FilterQuery != "" {
			filter = io.FilterQuery
		}
		searchQuery = fmt.Sprintf("(%s)=>[KNN %d @%s $%s AS %s]",
			filter, *co.TopK, r.config.VectorField, paramVector, SortByDistanceAttributeName)
	}

	returnFields := make([]options.FtSearchReturnField, 0, len(r.config.ReturnFields))
	for _, field := range r.config.ReturnFields {
		returnFields = append(returnFields, options.FtSearchReturnField{FieldIdentifier: field})
	}

	dialect := r.config.Dialect
	searchOpts := &options.FtSearchOptions{
		Params:       params,
		ReturnFields: returnFields,
		Limit:        &options.FtSearchLimit{Offset: 0, Count: *co.TopK},
		Dialect:      &dialect,
	}

	result, err := glideft.FtSearch(ctx, r.config.Client, *co.Index, searchQuery, searchOpts)
	if err != nil {
		return nil, err
	}

	for _, raw := range result.Documents {
		doc, convErr := r.config.DocumentConverter(ctx, raw)
		if convErr != nil {
			return nil, convErr
		}
		docs = append(docs, doc)
	}

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})
	return docs, nil
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

const typ = "Valkey"

func (r *Retriever) GetType() string {
	return typ
}

func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

func defaultResultParser(returnFields []string) func(ctx context.Context, doc models.FtSearchDocument) (*schema.Document, error) {
	return func(ctx context.Context, doc models.FtSearchDocument) (*schema.Document, error) {
		resp := &schema.Document{
			ID:       doc.Key,
			Content:  "",
			MetaData: map[string]any{},
		}
		for _, field := range returnFields {
			val, found := doc.Fields[field]
			if !found {
				return nil, fmt.Errorf("[defaultResultParser] field=%s not found in doc, doc=%v", field, doc)
			}
			strVal, _ := val.(string)
			if field == defaultReturnFieldContent {
				resp.Content = strVal
			} else if field == defaultReturnFieldVectorContent {
				resp.WithDenseVector(bytes2Vector([]byte(strVal)))
			} else {
				resp.MetaData[field] = val
			}
		}
		return resp, nil
	}
}

func vector2Bytes(vector []float64) []byte {
	bytes := make([]byte, len(vector)*4)
	for i, v := range vector {
		binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(float32(v)))
	}
	return bytes
}

func bytes2Vector(b []byte) []float64 {
	n := len(b) / 4
	vector := make([]float64, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(b[i*4 : (i+1)*4])
		vector[i] = float64(math.Float32frombits(bits))
	}
	return vector
}
