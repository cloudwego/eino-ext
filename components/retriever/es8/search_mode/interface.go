package search_mode

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"

	"github.com/cloudwego/eino/components/retriever"
)

type SearchMode interface { // nolint: byted_s_interface_name
	BuildRequest(ctx context.Context, query string, options *retriever.Options) (*search.Request, error)
}
