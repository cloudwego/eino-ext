package ark

type caching string

const (
	cachingEnabled  caching = "enabled"
	cachingDisabled caching = "disabled"
)

const (
	callbackExtraKeyThinking      = "ark-thinking"
	callbackExtraKeyPreResponseID = "ark-previous-response-id"
)
