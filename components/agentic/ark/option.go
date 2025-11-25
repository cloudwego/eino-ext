package ark

import (
	"github.com/cloudwego/eino/components/agency"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

type arkOptions struct {
	reasoning *responses.ResponsesReasoning
	thinking  *responses.ResponsesThinking

	customHeaders map[string]string
	cache         *CacheOption
}

func WithReasoning(reasoning *responses.ResponsesReasoning) agency.Option {
	return agency.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.reasoning = reasoning
	})
}

func WithThinking(thinking *responses.ResponsesThinking) agency.Option {
	return agency.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.thinking = thinking
	})
}

func WithCustomHeaders(headers map[string]string) agency.Option {
	return agency.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.customHeaders = headers
	})
}

type CacheOption struct {
	// HeadPreviousResponseID is a response ID from a previous ResponsesAPI call.
	// This ID links the current request to a previous conversation context, enabling
	// features like conversation continuation and prefix caching.
	// The referenced response must be cached before use.
	// Only applicable for ResponsesAPI.
	// Optional.
	HeadPreviousResponseID *string

	// SessionCache is the configuration of ResponsesAPI session cache.
	// Optional.
	SessionCache *SessionCacheConfig
}

func WithCacheOption(option CacheOption) agency.Option {
	return agency.WrapImplSpecificOptFn(func(o *arkOptions) {
		o.cache = &option
	})
}
