package search_mode

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"

	"github.com/cloudwego/eino/components/retriever"
)

func SearchModeRawStringRequest() SearchMode {
	return &rawString{}
}

type rawString struct{}

func (r rawString) BuildRequest(_ context.Context, query string, _ *retriever.Options) (*search.Request, error) {
	req, err := search.NewRequest().FromJSON(query)
	if err != nil {
		return nil, err
	}

	return req, nil
}
