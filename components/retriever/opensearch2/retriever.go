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

package opensearch2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	opensearch "github.com/opensearch-project/opensearch-go/v2"
)

// RetrieverConfig contains configuration for the OpenSearch retriever.
type RetrieverConfig struct {
	// Client is the OpenSearch client used for retrieval.
	Client *opensearch.Client `json:"client"`

	// Index is the name of the OpenSearch index.
	Index string `json:"index"`
	// TopK specifies the number of results to return.
	// Default is 10.
	TopK int `json:"top_k"`
	// ScoreThreshold filters results with a similarity score below this value.
	ScoreThreshold *float64 `json:"score_threshold"`

	// SearchMode defines the strategy for retrieval (e.g., dense vector, keyword).
	SearchMode SearchMode `json:"search_mode"`
	// ResultParser parses OpenSearch hits into Eino documents.
	// If ResultParser not provided, defaultResultParser will be used as default.
	ResultParser func(ctx context.Context, hit map[string]any) (doc *schema.Document, err error)
	// Embedding is the embedding model used for vectorization.
	// It is required when SearchMode needs it.
	Embedding embedding.Embedder
}

// SearchMode defines the interface for building OpenSearch search requests.
type SearchMode interface {
	// BuildRequest generates the search request body based on the configuration and query.
	BuildRequest(ctx context.Context, conf *RetrieverConfig, query string, opts ...retriever.Option) (map[string]any, error)
}

// Retriever implements the [retriever.Retriever] interface for OpenSearch.
type Retriever struct {
	client *opensearch.Client
	config *RetrieverConfig
}

// NewRetriever creates a new OpenSearch retriever with the provided configuration.
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
		return nil, fmt.Errorf("[NewRetriever] opensearch client not provided")
	}
	return &Retriever{
		client: conf.Client,
		config: conf,
	}, nil
}

// Retrieve searches for documents in OpenSearch matching the given query.
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

	reqBody, err := r.config.SearchMode.BuildRequest(ctx, r.config, query, opts...)
	if err != nil {
		return nil, err
	}

	// Add size to request body if not present or override it
	// Actually better to let SearchMode handle it or merge it here.
	// But SearchMode might produce a complex query.
	// Usually strict replacement of 'size' is fine.
	reqBody["size"] = *options.TopK
	if options.ScoreThreshold != nil {
		reqBody["min_score"] = *options.ScoreThreshold
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("[Retrieve] marshal request body failed: %w", err)
	}

	resp, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex(*options.Index),
		r.client.Search.WithBody(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("[Retrieve] search failed: %s", resp.String())
	}

	docs, err = r.parseSearchResult(ctx, resp.Body)
	if err != nil {
		return nil, err
	}

	callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs})

	return docs, nil
}

func (r *Retriever) parseSearchResult(ctx context.Context, body io.Reader) (docs []*schema.Document, err error) {
	var response map[string]any
	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return nil, fmt.Errorf("[parseSearchResult] decode response failed: %w", err)
	}

	hitsWrapper, ok := response["hits"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("[parseSearchResult] response hits field missing or invalid")
	}

	hits, ok := hitsWrapper["hits"].([]any)
	if !ok {
		// Empty hits or invalid format
		return []*schema.Document{}, nil
	}

	docs = make([]*schema.Document, 0, len(hits))

	for _, h := range hits {
		hit, ok := h.(map[string]any)
		if !ok {
			continue
		}
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

func defaultResultParser(ctx context.Context, hit map[string]any) (*schema.Document, error) {
	idVal, ok := hit["_id"]
	if !ok {
		return nil, fmt.Errorf("defaultResultParser: field '_id' not found in hit")
	}
	id, ok := idVal.(string)
	if !ok {
		return nil, fmt.Errorf("defaultResultParser: field '_id' is not a string in hit")
	}

	score, _ := hit["_score"].(float64)

	source, ok := hit["_source"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("defaultResultParser: field '_source' not found in document %s", id)
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
