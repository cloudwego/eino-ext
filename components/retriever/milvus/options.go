package milvus

import (
	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

type ImplOptions struct {
	// Filter is the filter for the search
	// Optional, and the default value is empty
	// It's means the milvus search required param, and refer to https://milvus.io/docs/boolean.md
	Filter string

	// SearchQueryOptFn is the function to set the search query option
	// Optional, and the default value is nil
	// It's means the milvus search extra search options, and refer to client.SearchQueryOptionFunc
	SearchQueryOptFn func(option *client.SearchQueryOption)
}

func (i *ImplOptions) WithFilter(filter string) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Filter = filter
	})
}

func (i *ImplOptions) WithSearchQueryOptFn(f func(option *client.SearchQueryOption)) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.SearchQueryOptFn = f
	})
}
