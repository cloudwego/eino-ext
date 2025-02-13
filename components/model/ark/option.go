package ark

import (
	"github.com/cloudwego/eino/components/model"
)

type arkOptions struct {
	customHeaders map[string]string
}

func WithCustomHeader(m map[string]string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.customHeaders = m
	})
}
