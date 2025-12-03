# Ollama 模型

一个针对 [Eino](https://github.com/cloudwego/eino) 的 Ollama 模型实现，实现了 `ToolCallingChatModel` 接口。这使得能够与 Eino 的 LLM 功能无缝集成，以增强自然语言处理和生成能力。

## 特性

- 实现了 `github.com/cloudwego/eino/components/model.Model`
- 轻松与 Eino 的模型系统集成
- 可配置的模型参数
- 支持聊天补全
- 支持流式响应
- 支持自定义响应解析
- 灵活的模型配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/ollama@latest
```

## 快速开始

以下是如何使用 Ollama 模型的快速示例：

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ollama"
)

func main() {
	ctx := context.Background()
	modelName := os.Getenv("MODEL_NAME")
	
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: "http://localhost:11434",
		Model:   modelName,
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v\n", err)
		return
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})
	if err != nil {
		log.Printf("Generate failed, err=%v\n", err)
		return
	}

	log.Printf("output: \n%v\n", resp)
}

```

## 配置

可以使用 `ollama.ChatModelConfig` 结构体配置模型：


```go
type ChatModelConfig struct {
    BaseURL string        `json:"base_url"`
    Timeout time.Duration `json:"timeout"` // request timeout for http client
    
    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`
    
    Model     string          `json:"model"`
    Format    json.RawMessage `json:"format"`
    KeepAlive *time.Duration  `json:"keep_alive"`
    
    Options *Options `json:"options"`
    
    Thinking *ThinkValue `json:"thinking"`
}


type Options struct {
    Runner
    
    // NumKeep specifies the number of tokens from the prompt to retain when the context size is exceeded and tokens need to be trimmed.
    NumKeep int `json:"num_keep,omitempty"`
    // Seed sets the random number seed for the model. Using the same seed with the same parameters will produce the same output.
    Seed int `json:"seed,omitempty"`
    // NumPredict sets the maximum number of tokens to generate.
    NumPredict int `json:"num_predict,omitempty"`
    // TopK controls the diversity of the generated text by limiting the selection of tokens to the top k most likely tokens.
    TopK int `json:"top_k,omitempty"`
    // TopP, also known as nucleus sampling, is another way to control the diversity of the generated text. It filters out the least likely tokens whose cumulative probability is below a certain threshold.
    TopP float32 `json:"top_p,omitempty"`
    // MinP is a parameter that works with TopP to ensure that the generated text is not too constrained. It sets a minimum probability for a token to be considered.
    MinP float32 `json:"min_p,omitempty"`
    // TypicalP is a parameter that helps to generate more "typical" or expected text by sampling from a reduced set of tokens that are considered typical.
    TypicalP float32 `json:"typical_p,omitempty"`
    // RepeatLastN specifies how many of the last N tokens to consider for penalizing repetition.
    RepeatLastN int `json:"repeat_last_n,omitempty"`
    // Temperature controls the randomness of the generated text. A higher temperature results in more random and creative output, while a lower temperature produces more predictable and conservative text.
    Temperature float32 `json:"temperature,omitempty"`
    // RepeatPenalty is used to penalize the model for repeating tokens that have already appeared in the generated text.
    RepeatPenalty float32 `json:"repeat_penalty,omitempty"`
    // PresencePenalty is used to penalize the model for introducing new tokens that were not present in the prompt.
    PresencePenalty float32 `json:"presence_penalty,omitempty"`
    // FrequencyPenalty is used to penalize the model for using tokens that appear frequently in the training data.
    FrequencyPenalty float32 `json:"frequency_penalty,omitempty"`
    // Stop is a list of strings that will cause the generation to stop if they are encountered.
    Stop []string `json:"stop,omitempty"`
}

type ThinkValue struct {
    // Value can be a bool or string
    Value interface{}
}

```


## 示例

### 文本生成

```go

package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ollama"
)

func main() {
	ctx := context.Background()
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: "http://localhost:11434",
		Model:   modelName,
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v\n", err)
		return
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})
	if err != nil {
		log.Printf("Generate failed, err=%v\n", err)
		return
	}

	log.Printf("output: \n%v\n", resp)
}

```

### 多模态支持(图片理解)

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ollama"
)

func main() {
	ctx := context.Background()
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: "http://localhost:11434",
		Model:   modelName,
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v\n", err)
		return
	}

	multiModalMsg := &schema.Message{
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "this picture is a landscape photo, what's the picture's content",
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						URL: of("https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcT11qEDxU4X_MVKYQVU5qiAVFidA58f8GG0bQ&s"),
					},
					Detail: schema.ImageURLDetailAuto,
				},
			},
		},
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		multiModalMsg,
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)
}

func of[T any](a T) *T {
	return &a
}

```

### 流式生成

```go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ollama"
)

func main() {
	ctx := context.Background()
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: "http://localhost:11434",
		Model:   modelName,
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v\n", err)
		return
	}

	streamMsgs, err := chatModel.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})

	if err != nil {
		log.Printf("Generate failed, err=%v", err)
		return
	}

	defer streamMsgs.Close()

	log.Println("typewriter output:")
	for {
		msg, err := streamMsgs.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("\nstream.Recv failed, err=%v", err)
			return
		}
		fmt.Print(msg.Content)
	}

	fmt.Print("\n")
}

```

### 工具调用

```go

package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ollama"
)

func main() {

	ctx := context.Background()
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: "http://localhost:11434",
		Model:   modelName,
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v", err)
		return
	}

	err = chatModel.BindTools([]*schema.ToolInfo{
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
		log.Printf("BindForcedTools failed, err=%v", err)
		return
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
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
		log.Printf("Generate failed, err=%v", err)
		return
	}

	log.Printf("output: \n%+v", resp)
}

```

### 开启Thinking模式

```go

package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"
	ollamaapi "github.com/eino-contrib/ollama/api"

	"github.com/cloudwego/eino-ext/components/model/ollama"
)

func main() {
	ctx := context.Background()
	modelName := os.Getenv("MODEL_NAME")
	thinking := ollamaapi.ThinkValue{Value: true}
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  "http://localhost:11434",
		Model:    modelName,
		Thinking: &thinking,
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v\n", err)
		return
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})
	if err != nil {
		log.Printf("Generate failed, err=%v\n", err)
		return
	}

	log.Printf("output thinking: \n%v\n", resp.ReasoningContent)
	log.Printf("output content: \n%v\n", resp.Content)
}

```

## 更多信息

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Ollama Documentation](https://ollama.readthedocs.io/api/#generate-a-chat-completion)
