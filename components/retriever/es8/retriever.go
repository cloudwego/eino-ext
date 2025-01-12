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
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/retriever/es8/field_mapping"
	"github.com/cloudwego/eino-ext/components/retriever/es8/internal"
)

type RetrieverConfig struct {
	ESConfig elasticsearch.Config `json:"es_config"`

	Index          string   `json:"index"`
	TopK           int      `json:"top_k"`
	ScoreThreshold *float64 `json:"score_threshold"`

	// SearchMode retrieve strategy, see prepared impls in search_mode package:
	// use search_mode.SearchModeExactMatch with string query
	// use search_mode.SearchModeApproximate with search_mode.ApproximateQuery
	// use search_mode.SearchModeDenseVectorSimilarity with search_mode.DenseVectorSimilarityQuery
	// use search_mode.SearchModeSparseVectorTextExpansion with search_mode.SparseVectorTextExpansionQuery
	// use search_mode.SearchModeRawStringRequest with json search request
	SearchMode SearchMode `json:"search_mode"`
	// Embedding vectorization method, must provide when SearchMode needed
	Embedding embedding.Embedder
}

type SearchMode interface {
	BuildRequest(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (*search.Request, error)
}

type Retriever struct {
	client *elasticsearch.TypedClient
	config *RetrieverConfig
}

func NewRetriever(_ context.Context, conf *RetrieverConfig) (*Retriever, error) {
	if conf.SearchMode == nil {
		return nil, fmt.Errorf("[NewRetriever] search mode not provided")
	}

	client, err := elasticsearch.NewTypedClient(conf.ESConfig)
	if err != nil {
		return nil, fmt.Errorf("[NewRetriever] new es client failed, %w", err)
	}

	return &Retriever{
		client: client,
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

	resp, err := r.client.Search().
		Index(r.config.Index).
		Request(req).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	docs, err = r.parseSearchResult(resp)
	if err != nil {
		return nil, err
	}

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})

	return docs, nil
}

func (r *Retriever) parseSearchResult(resp *search.Response) (docs []*schema.Document, err error) {
	docs = make([]*schema.Document, 0, len(resp.Hits.Hits))

	for _, hit := range resp.Hits.Hits {
		var raw map[string]any
		if err = json.Unmarshal(hit.Source_, &raw); err != nil {
			return nil, fmt.Errorf("[parseSearchResult] unexpected hit source type, source=%v", string(hit.Source_))
		}

		var id string
		if hit.Id_ != nil {
			id = *hit.Id_
		}

		content, ok := raw[field_mapping.DocFieldNameContent].(string)
		if !ok {
			return nil, fmt.Errorf("[parseSearchResult] content type not string, raw=%v", raw)
		}

		expMap := make(map[string]any, len(raw)-1)
		for k, v := range raw {
			if k != internal.DocExtraKeyEsFields {
				expMap[k] = v
			}
		}

		doc := &schema.Document{
			ID:       id,
			Content:  content,
			MetaData: map[string]any{internal.DocExtraKeyEsFields: expMap},
		}

		if hit.Score_ != nil {
			doc.WithScore(float64(*hit.Score_))
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
