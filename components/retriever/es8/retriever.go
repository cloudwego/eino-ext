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

package es8

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type RetrieverConfig struct {
	Client *elasticsearch.Client `json:"client"`

	Index string `json:"index"`
	// TopK number of result to return as top hits.
	// Default is 10
	TopK           int      `json:"top_k"`
	ScoreThreshold *float64 `json:"score_threshold"`

	// SearchMode retrieve strategy, see prepared impls in search_mode package:
	// use search_mode.SearchModeExactMatch with string query
	// use search_mode.SearchModeApproximate with search_mode.ApproximateQuery
	// use search_mode.SearchModeDenseVectorSimilarity with search_mode.DenseVectorSimilarityQuery
	// use search_mode.SearchModeSparseVectorTextExpansion with search_mode.SparseVectorTextExpansionQuery
	// use search_mode.SearchModeRawStringRequest with json search request
	SearchMode SearchMode `json:"search_mode"`
	// ResultParser parse document from es search hits.
	// If ResultParser not provided, defaultResultParser will be used as default
	ResultParser func(ctx context.Context, hit types.Hit) (doc *schema.Document, err error)
	// Embedding vectorization method, must provide when SearchMode needed
	Embedding embedding.Embedder
}

type SearchMode interface {
	// BuildRequest generate search request from config, query and options.
	// Additionally, some specified options (like filters for query) will be provided in options,
	// and use retriever.GetImplSpecificOptions[options.ImplOptions] to get it.
	BuildRequest(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (*search.Request, error)
}

type Retriever struct {
	client *elasticsearch.Client
	config *RetrieverConfig
}

func NewRetriever(_ context.Context, conf *RetrieverConfig) (*Retriever, error) {
	if conf.SearchMode == nil {
		return nil, fmt.Errorf("[NewRetriever] search mode not provided")
	}

	if conf.TopK == 0 {
		conf.TopK = defaultTopK
	}

	if conf.ResultParser == nil {
		return nil, fmt.Errorf("[NewRetriever] result parser not provided")
	}

	if conf.Client == nil {
		return nil, fmt.Errorf("[NewRetriever] es client not provided")
	}
	return &Retriever{
		client: conf.Client,
		config: conf,
	}, nil
}

func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	options := retriever.GetCommonOptions(&retriever.Options{
		Index:          &r.config.Index,
		TopK:           &r.config.TopK,
		ScoreThreshold: r.config.ScoreThreshold,
		Embedding:      r.config.Embedding,
	}, opts...)

	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           *options.TopK,
		ScoreThreshold: options.ScoreThreshold,
	})

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

func (r *Retriever) GetType() string {
	return typ
}

func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}
