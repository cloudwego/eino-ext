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

package es9

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// RetrieverConfig contains configuration for the ES9 retriever.
type RetrieverConfig struct {
	// Client is the Elasticsearch client used for retrieval.
	Client *elasticsearch.Client `json:"client"`

	// Index is the name of the Elasticsearch index.
	Index string `json:"index"`
	// TopK specifies the number of results to return.
	// Default is 10.
	TopK int `json:"top_k"`
	// ScoreThreshold filters results with a similarity score below this value.
	ScoreThreshold *float64 `json:"score_threshold"`

	// SearchMode defines the strategy for retrieval (e.g., dense vector, keyword).
	// use search_mode.SearchModeExactMatch with string query
	// use search_mode.SearchModeApproximate with search_mode.ApproximateQuery
	// use search_mode.SearchModeDenseVectorSimilarity with search_mode.DenseVectorSimilarityQuery
	// use search_mode.SearchModeSparseVectorTextExpansion with search_mode.SparseVectorTextExpansionQuery
	// use search_mode.SearchModeRawStringRequest with json search request
	SearchMode SearchMode `json:"search_mode"`
	// ResultParser parses Elasticsearch hits into Eino documents.
	// If ResultParser not provided, defaultResultParser will be used as default.
	ResultParser func(ctx context.Context, hit types.Hit) (doc *schema.Document, err error)
	// Embedding is the embedding model used for vectorization.
	// It is required when SearchMode needs it.
	Embedding embedding.Embedder
}

// SearchMode defines the interface for building Elasticsearch search requests.
type SearchMode interface {
	// BuildRequest generates the search request from configuration, query, and options.
	// Additionally, some specified options (like filters for query) will be provided in options,
	// and use retriever.GetImplSpecificOptions[options.ImplOptions] to get it.
	BuildRequest(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (*search.Request, error)
}

// Retriever implements the [retriever.Retriever] interface for Elasticsearch 9.x.
type Retriever struct {
	client *elasticsearch.Client
	config *RetrieverConfig
}

// NewRetriever creates a new ES9 retriever with the provided configuration.
func NewRetriever(_ context.Context, conf *RetrieverConfig) (*Retriever, error) {
	if conf.SearchMode == nil {
		return nil, fmt.Errorf("[NewRetriever] search mode not provided")
	}

	if conf.TopK == 0 {
		conf.TopK = defaultTopK
	}

	if conf.ResultParser == nil {
		conf.ResultParser = defaultResultParser
	}

	if conf.Client == nil {
		return nil, fmt.Errorf("[NewRetriever] es client not provided")
	}
	return &Retriever{
		client: conf.Client,
		config: conf,
	}, nil
}

// Retrieve searches for documents in Elasticsearch matching the given query.
func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	options := retriever.GetCommonOptions(&retriever.Options{
		Index:          &r.config.Index,
		TopK:           &r.config.TopK,
		ScoreThreshold: r.config.ScoreThreshold,
		Embedding:      r.config.Embedding,
	}, opts...)

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *options.TopK,
		ScoreThreshold: options.ScoreThreshold,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	req, err := r.config.SearchMode.BuildRequest(ctx, r.config, query, opts...)
	if err != nil {
		return nil, err
	}

	resp, err := search.NewSearchFunc(r.client)().
		Index(r.config.Index).
		Request(req).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	docs, err = r.parseSearchResult(ctx, resp)
	if err != nil {
		return nil, err
	}

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})

	return docs, nil
}

func (r *Retriever) parseSearchResult(ctx context.Context, resp *search.Response) (docs []*schema.Document, err error) {
	if len(resp.Hits.Hits) == 0 {
		return []*schema.Document{}, nil
	}
	docs = make([]*schema.Document, 0, len(resp.Hits.Hits))

	for _, hit := range resp.Hits.Hits {
		doc, err := r.config.ResultParser(ctx, hit)
		if err != nil {
			return nil, err
		}

		docs = append(docs, doc)
	}

	return docs, nil
}

// GetType returns the type of the retriever.
func (r *Retriever) GetType() string {
	return typ
}

// IsCallbacksEnabled checks if callbacks are enabled for this retriever.
func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

func defaultResultParser(ctx context.Context, hit types.Hit) (*schema.Document, error) {
	if hit.Id_ == nil {
		return nil, fmt.Errorf("defaultResultParser: field '_id' not found in hit")
	}
	id := *hit.Id_

	score := 0.0
	if hit.Score_ != nil {
		score = float64(*hit.Score_)
	}

	if hit.Source_ == nil {
		return nil, fmt.Errorf("defaultResultParser: field '_source' not found in document %s", id)
	}

	var source map[string]any
	if err := json.Unmarshal(hit.Source_, &source); err != nil {
		return nil, fmt.Errorf("defaultResultParser: unmarshal document content failed: %v", err)
	}

	val, ok := source["content"]
	if !ok {
		return nil, fmt.Errorf("defaultResultParser: field 'content' not found in document %s; please use a custom ResultParser or ensure index mapping has 'content' field", id)
	}

	content, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("defaultResultParser: field 'content' in document %s is not a string", id)
	}

	// Remove content from metadata to avoid duplication if it's large
	meta := make(map[string]any, len(source)+1)
	for k, v := range source {
		if k != "content" {
			meta[k] = v
		}
	}
	meta["score"] = score

	doc := &schema.Document{
		ID:       id,
		Content:  content,
		MetaData: meta,
	}
	return doc.WithScore(score), nil
}
