package search_mode

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/cloudwego/eino-ext/components/retriever/es8/field_mapping"
	"github.com/cloudwego/eino/components/retriever"
)

func SearchModeExactMatch() SearchMode {
	return &exactMatch{}
}

type exactMatch struct{}

func (e exactMatch) BuildRequest(ctx context.Context, query string, options *retriever.Options) (*search.Request, error) {
	q := &types.Query{
		Match: map[string]types.MatchQuery{
			field_mapping.DocFieldNameContent: {Query: query},
		},
	}

	req := &search.Request{Query: q, Size: options.TopK}
	if options.ScoreThreshold != nil {
		req.MinScore = (*types.Float64)(of(*options.ScoreThreshold))
	}

	return req, nil
}
