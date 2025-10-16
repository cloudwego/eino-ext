# Qianfan Model

A Qianfan model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Model` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/model.Model`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for chat completion
- Support for streaming responses
- Custom response parsing support
- Flexible model configuration

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/qianfan@latest
```

## Quick Start

Here's a quick example of how to use the Qianfan model:

```go

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"

	cm, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               "ernie-3.5-8k",
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	ir, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("你好"),
	})
	/***
	Alternatively, you can use the following calling methods

	ir, err = cm.Generate(ctx, []*schema.Message{
		{Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "你好"},
			}},
	})

	If the model supports multimodal scenarios, you can use the following call methods

	ir, err = cm.Generate(ctx, []*schema.Message{
		{Role: schema.User,UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "介绍下如下图片内容"},
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{URL: of("https://img0.baidu.com/it/u=4078387433,1356951957&fm=253&fmt=auto&app=138&f=JPEG?w=800&h=1034"),
				}}},
			}},
	})

	***/

	if err != nil {
		log.Fatalf("Generate of qianfan failed, err=%v", err)
	}

	fmt.Println(ir)
	// assistant: 你好！我是文心一言，很高兴与你交流。请问你有什么想问我的吗？无论是关于知识、创作还是其他任何问题，我都会尽力回答你。
}

func of[T any](t T) *T {
	return &t
}
```

## Configuration

The model can be configured using the `qwen.ChatModelConfig` struct:

```go
type ChatModelConfig struct {
// Model is the model to use for the chat completion.
Model string

// LLMRetryCount is the number of times to retry a failed request.
LLMRetryCount *int

// LLMRetryTimeout is the timeout for each retry attempt.
LLMRetryTimeout *float32

// LLMRetryBackoffFactor is the backoff factor for retries.
LLMRetryBackoffFactor *float32

// Temperature controls the randomness of the output. A higher value makes the output more random, while a lower value makes it more focused and deterministic. Default is 0.95, range (0, 1.0].
Temperature *float32

// TopP controls the diversity of the output. A higher value increases the diversity of the generated text. Default is 0.7, range [0, 1.0].
TopP *float32

// PenaltyScore reduces the generation of repetitive tokens by adding a penalty. A higher value means a larger penalty. Range: [1.0, 2.0].
PenaltyScore *float64

// MaxCompletionTokens is the maximum number of tokens to generate in the completion. Range [2, 2048].
MaxCompletionTokens *int

// Seed is the random seed for generation. Range (0, 2147483647).
Seed *int

// Stop is a list of strings that will stop the generation when the model generates a token that is a suffix of one of the strings.
Stop []string

// User is a unique identifier representing the end-user.
User *string

// FrequencyPenalty specifies the frequency penalty to control the repetition of generated text. Range [-2.0, 2.0].
FrequencyPenalty *float64

// PresencePenalty specifies the presence penalty to control the repetition of generated text. Range [-2.0, 2.0].
PresencePenalty *float64

// ParallelToolCalls specifies whether to call tools in parallel. Defaults to true.
ParallelToolCalls *bool

// ResponseFormat specifies the format of the response.
ResponseFormat *qianfan.ResponseFormat
}
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Qianfan Documentation](https://cloud.baidu.com/doc/qianfan-api/s/3m7of64lb)
