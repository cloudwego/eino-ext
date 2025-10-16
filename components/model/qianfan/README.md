# Qianfan Model

A Qianfan model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

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
	"os"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"
	modelName := os.Getenv("MODEL_NAME")
	cm, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               modelName,
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
		Seed:                of(0),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	ir, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})

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

The model can be configured using the `qianfan.ChatModelConfig` struct:


```go
// ChatModelConfig config for qianfan chat completion
// see: https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Wm3fhy2vb
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
	ResponseFormat *ResponseFormat
}

```








## examples

### generate

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"
	modelName := os.Getenv("MODEL_NAME")
	cm, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               modelName,
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
		Seed:                of(0),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	ir, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})

	if err != nil {
		log.Fatalf("Generate of qianfan failed, err=%v", err)
	}

	fmt.Println(ir)
}

func of[T any](t T) *T {
	return &t
}

```

### generate_with_image

```go

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               modelName,
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
		Seed:                of(0),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	image, err := os.ReadFile("./examples/generate_with_image/test.jpg")
	if err != nil {
		log.Fatalf("os.ReadFile failed, err=%v\n", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "What do you see in this image?",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: of(base64.StdEncoding.EncodeToString(image)),
							MIMEType:   "image/jpeg",
						},
						Detail: schema.ImageURLDetailAuto,
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}
	fmt.Printf("Assistant: %s\n", resp.Content)
}

func of[T any](t T) *T {
	return &t
}

```

### stream

```go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"
	modelName := os.Getenv("MODEL_NAME")
	cm, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               modelName,
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
	})
	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	sr, err := cm.Stream(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})

	if err != nil {
		log.Fatalf("Stream of qianfan failed, err=%v", err)
	}

	var ms []*schema.Message
	for {
		m, err := sr.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Fatalf("Stream of qianfan failed, err=%v", err)
		}

		fmt.Println(m)
		ms = append(ms, m)
	}
	sm, err := schema.ConcatMessages(ms)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}

	fmt.Println(sm)
}

func of[T any](t T) *T {
	return &t
}

```

### tool

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"

	modelName := os.Getenv("MODEL_NAME")
	cm, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               modelName,
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
	})
	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	err = cm.BindTools([]*schema.ToolInfo{
		{
			Name: "user_company",
			Desc: "Query the user's company and position information based on their name and email",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "The user's name",
					},
					"email": {
						Type: "string",
						Desc: "The user's email",
					},
				}),
		},
		{
			Name: "user_salary",
			Desc: "Query the user's salary information based on their name and email",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "The user's name",
					},
					"email": {
						Type: "string",
						Desc: "The user's email",
					},
				}),
		},
	})
	if err != nil {
		log.Fatalf("BindTools of qianfan failed, err=%v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a real estate agent. Use the user_company and user_salary APIs to provide relevant property information based on the user's salary and job. Email is required",
		},
		{
			Role:    schema.User,
			Content: "My name is zhangsan, and my email is zhangsan@bytedance.com. Please recommend some suitable houses for me.",
		},
	})

	if err != nil {
		log.Fatalf("Generate of qianfan failed, err=%v", err)
	}

	fmt.Println(resp)
}

func of[T any](t T) *T {
	return &t
}

```



## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Qianfan Documentation](https://cloud.baidu.com/doc/qianfan-api/s/rm7u7qdiq)
