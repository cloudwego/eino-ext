package openairesponse

import "github.com/cloudwego/eino-ext/components/model/openai"

const (
	ChatCompletionResponseFormatTypeJSONObject = openai.ChatCompletionResponseFormatTypeJSONObject
	ChatCompletionResponseFormatTypeJSONSchema = openai.ChatCompletionResponseFormatTypeJSONSchema
	ChatCompletionResponseFormatTypeText       = openai.ChatCompletionResponseFormatTypeText
)

type ChatCompletionResponseFormat = openai.ChatCompletionResponseFormat