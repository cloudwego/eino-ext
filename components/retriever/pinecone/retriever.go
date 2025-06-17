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
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	topK              = 5
	defaultContentKey = "content"
)

type RetrieverConfig struct {
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

	// Retrieve parameters
	TopK int // default 5
	// ScoredVectorToDocument converts pinecone retrieve result to eino document
	// If ScoredVectorToDocument is not set, will use defaultScoredVectorToDocument as default.
	ScoredVectorToDocument func(ctx context.Context, sv *pinecone.ScoredVector) (*schema.Document, error)
	// Embedding vectorization method when dense vector not provided in document extra
	Embedding embedding.Embedder
}

type Retriever struct {
	conf    *RetrieverConfig
	idxConn *pinecone.IndexConnection
}

func NewRetriever(ctx context.Context, config *RetrieverConfig) (*Retriever, error) {
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

	if config.TopK == 0 {
		config.TopK = topK
	}

	if config.ScoredVectorToDocument == nil {
		config.ScoredVectorToDocument = defaultScoredVectorToDocument
	}

	return &Retriever{
		conf:    config,
		idxConn: idxConn,
	}, nil
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (
	docs []*schema.Document, err error) {

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	co := retriever.GetCommonOptions(&retriever.Options{
		Index:     &r.conf.IndexName,
		TopK:      &r.conf.TopK,
		Embedding: r.conf.Embedding,
	}, opts...)
	io := retriever.GetImplSpecificOptions(&options{}, opts...)

	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:  query,
		TopK:   *co.TopK,
		Filter: marshalStringNoErr(io.MetadataFilter),
	})

	req, err := r.makeQueryRequest(ctx, query, co, io)
	if err != nil {
		return nil, err
	}

	resp, err := r.idxConn.QueryByVectorValues(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, match := range resp.Matches {
		doc, err := r.conf.ScoredVectorToDocument(ctx, match)
		if err != nil {
			return nil, fmt.Errorf("[Retrieve] pinecone retriever ScoredVectorToDocument failed, %w", err)
		}

		docs = append(docs, doc)
	}

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})

	return docs, nil
}

func (r *Retriever) makeQueryRequest(ctx context.Context, query string, co *retriever.Options, io *options) (
	*pinecone.QueryByVectorValuesRequest, error) {

	req := &pinecone.QueryByVectorValuesRequest{
		Vector:          nil,
		TopK:            uint32(*co.TopK),
		MetadataFilter:  nil,
		IncludeValues:   true,
		IncludeMetadata: true,
		SparseValues:    toPineconeSparseVector(io.SparseVector),
	}

	if io.DenseVector == nil {
		emb := co.Embedding
		if emb == nil {
			return nil, fmt.Errorf("[makeQueryRequest] embedding method in config must not be nil when query not contains dense vector")
		}

		vectors, err := emb.EmbedStrings(r.makeEmbeddingCtx(ctx, emb), []string{query})
		if err != nil {
			return nil, fmt.Errorf("[makeQueryRequest] embed failed, %w", err)
		}

		if len(vectors) != 1 {
			return nil, fmt.Errorf("[makeQueryRequest] invalid return length of vector, got=%d, expected=1", len(vectors))
		}

		req.Vector = f64To32(vectors[0])
	} else {
		req.Vector = f64To32(io.DenseVector)
	}

	if io.MetadataFilter != nil {
		filter, err := structpb.NewStruct(io.MetadataFilter)
		if err != nil {
			return nil, err
		}

		req.MetadataFilter = filter
	}

	return req, nil
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

func defaultScoredVectorToDocument(ctx context.Context, sv *pinecone.ScoredVector) (*schema.Document, error) {
	metadata := sv.Vector.Metadata.AsMap()

	content, ok := metadata[defaultContentKey].(string)
	if !ok {
		return nil, fmt.Errorf("[defaultScoredVectorToDocument] pinecone retrieve content not found in metadata, key=%s", defaultContentKey)
	}

	doc := &schema.Document{
		ID:       sv.Vector.Id,
		Content:  content,
		MetaData: metadata,
	}

	doc.WithScore(float64(sv.Score)).WithDenseVector(f32To64(sv.Vector.Values))

	if sv.Vector.SparseValues != nil {
		doc.WithSparseVector(fromPineconeSparseVector(sv.Vector.SparseValues))
	}

	return doc, nil
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

func fromPineconeSparseVector(values *pinecone.SparseValues) map[int]float64 {
	if values == nil {
		return nil
	}

	sparse := make(map[int]float64)
	for i := range values.Indices {
		indices := values.Indices[i]
		vector := values.Values[i]

		sparse[int(indices)] = float64(vector)
	}

	return sparse
}
