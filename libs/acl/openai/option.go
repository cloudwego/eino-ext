package openai

import "github.com/cloudwego/eino/components/model"

type openaiOptions struct {
	ExtraFields map[string]any
}

func WithExtraFields(extraFields map[string]any) model.Option {
	return model.WrapImplSpecificOptFn(func(o *openaiOptions) {
		o.ExtraFields = extraFields
	})
}
