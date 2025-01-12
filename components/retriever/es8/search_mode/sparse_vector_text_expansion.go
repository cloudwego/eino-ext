package search_mode

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/cloudwego/eino-ext/components/retriever/es8/field_mapping"
	"github.com/cloudwego/eino/components/retriever"
)

// SearchModeSparseVectorTextExpansion convert the query text into a list of token-weight pairs,
// which are then used in a query against a sparse vector
// see: https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-text-expansion-query.html
func SearchModeSparseVectorTextExpansion(modelID string) SearchMode {
	return &sparseVectorTextExpansion{modelID}
}

type SparseVectorTextExpansionQuery struct {
	FieldKV field_mapping.FieldKV `json:"field_kv"`
	Filters []types.Query         `json:"filters,omitempty"`
}

// ToRetrieverQuery convert approximate query to string query
func (s *SparseVectorTextExpansionQuery) ToRetrieverQuery() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("[ToRetrieverQuery] convert query failed, %w", err)
	}

	return string(b), nil
}

type sparseVectorTextExpansion struct {
	modelID string
}

func (s sparseVectorTextExpansion) BuildRequest(ctx context.Context, query string, options *retriever.Options) (*search.Request, error) {
	var sq SparseVectorTextExpansionQuery
	if err := json.Unmarshal([]byte(query), &sq); err != nil {
		return nil, fmt.Errorf("[BuildRequest][SearchModeSparseVectorTextExpansion] parse query failed, %w", err)
	}

	name := fmt.Sprintf("%s.tokens", sq.FieldKV.FieldNameVector)
	teq := types.TextExpansionQuery{
		ModelId:   s.modelID,
		ModelText: sq.FieldKV.Value,
	}

	q := &types.Query{
		Bool: &types.BoolQuery{
			Must: []types.Query{
				{TextExpansion: map[string]types.TextExpansionQuery{name: teq}},
			},
			Filter: sq.Filters,
		},
	}

	req := &search.Request{Query: q, Size: options.TopK}
	if options.ScoreThreshold != nil {
		req.MinScore = (*types.Float64)(of(*options.ScoreThreshold))
	}

	return req, nil
}
