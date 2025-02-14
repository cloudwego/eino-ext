package ark

import (
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	keyOfRequestID = "ark-request-id"
)

type arkRequestID string

func init() {
	compose.RegisterStreamChunkConcatFunc(func(chunks []arkRequestID) (final arkRequestID, err error) {
		if len(chunks) == 0 {
			return "", nil
		}

		return chunks[len(chunks)-1], nil
	})
}

func GetArkRequestID(msg *schema.Message) string {
	reqID, ok := msg.Extra[keyOfRequestID].(arkRequestID)
	if !ok {
		return ""
	}
	return string(reqID)
}
