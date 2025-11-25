package ark

import (
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	keyOfRequestID             = "ark-request-id"
	keyOfReasoningContent      = "ark-reasoning-content"
	keyOfModelName             = "ark-model-name"
	videoURLFPS                = "ark-model-video-url-fps"
	keyOfContextID             = "ark-context-id"
	keyOfResponseID            = "ark-response-id"
	keyOfResponseCacheExpireAt = "ark-response-cache-expire-at"
	keyOfServiceTier           = "ark-service-tier"
	ImageSizeKey               = "seedream-image-size"
)

type arkRequestID string
type arkModelName string
type arkServiceTier string
type arkResponseID string
type arkContextID string
type arkResponseCacheExpireAt int64

func init() {
	compose.RegisterStreamChunkConcatFunc(func(chunks []arkRequestID) (final arkRequestID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[arkRequestID]("_eino_ext_ark_request_id")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkModelName) (final arkModelName, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[arkModelName]("_eino_ext_ark_model_name")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkServiceTier) (final arkServiceTier, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[arkServiceTier]("_eino_ext_ark_service_tier")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkContextID) (final arkContextID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		// Some chunks may not contain a contextID, so it is more reliable to take the first non-empty contextID.
		for _, chunk := range chunks {
			if chunk != "" {
				return chunk, nil
			}
		}
		return "", nil
	})
	schema.RegisterName[arkContextID]("_eino_ext_ark_context_id")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkResponseID) (final arkResponseID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}
		// Some chunks may not contain a responseID, so it is more reliable to take the first non-empty responseID.
		for _, chunk := range chunks {
			if chunk != "" {
				return chunk, nil
			}
		}
		return "", nil
	})
	schema.RegisterName[arkResponseID]("_eino_ext_ark_response_id")

	compose.RegisterStreamChunkConcatFunc(func(chunks []arkResponseCacheExpireAt) (final arkResponseCacheExpireAt, err error) {
		if len(chunks) == 0 {
			return 0, nil
		}
		return chunks[len(chunks)-1], nil
	})
	schema.RegisterName[arkResponseCacheExpireAt]("_eino_ext_ark_response_cache_expire_at")
}

// GetResponseID returns the response ID from the message.
// Available only for ResponsesAPI responses.
func GetResponseID(msg *schema.AgenticMessage) (string, bool) {
	responseID_, ok := getMsgExtraValue[arkResponseID](msg, keyOfResponseID)
	if ok {
		return string(responseID_), true
	}
	// When the user serializes and deserializes the message,
	// the type will be lost and compatibility with the string type is required.
	responseIDStr, ok := getMsgExtraValue[string](msg, keyOfResponseID)
	if !ok {
		return "", false
	}
	return responseIDStr, true
}

func setResponseID(msg *schema.AgenticMessage, responseID string) {
	setMsgExtra(msg, keyOfResponseID, arkResponseID(responseID))
}

// getCacheExpiration returns the cache expiration time in seconds.
// Only available for ResponsesAPI responses.
func getCacheExpiration(msg *schema.Message) (expireAtSec int64, ok bool) {
	expireAtSec_, ok := getMsgExtraValue[arkResponseCacheExpireAt](msg, keyOfResponseCacheExpireAt)
	if ok {
		return int64(expireAtSec_), true
	}
	expireAtSec, ok = getMsgExtraValue[int64](msg, keyOfResponseCacheExpireAt)
	if ok {
		return expireAtSec, true
	}
	return 0, false
}

func setResponseCacheExpireAt(msg *schema.AgenticMessage, expireAt arkResponseCacheExpireAt) {
	setMsgExtra(msg, keyOfResponseCacheExpireAt, expireAt)
}

func getMsgExtraValue[T any](msg *schema.AgenticMessage, key string) (T, bool) {
	if msg == nil {
		var t T
		return t, false
	}
	val, ok := msg.Extra[key].(T)
	return val, ok
}

func setMsgExtra(msg *schema.AgenticMessage, key string, value any) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]any)
	}
	msg.Extra[key] = value
}
