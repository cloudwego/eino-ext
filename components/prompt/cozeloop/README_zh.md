# CozeLoop Component for Eino

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 CozeLoop 组件，集成了 [CozeLoop](https://github.com/coze-dev/cozeloop-go)。该组件实现了 `ChatTemplate` 接口，允许您从 CozeLoop 的提示词管理服务获取和格式化提示词。

## 特性

- 实现 `github.com/cloudwego/eino/components/prompt.ChatTemplate` 接口
- 与 CozeLoop SDK 集成，用于提示词管理
- 支持提示词版本控制
- 自动提示词格式化及变量替换
- 支持回调以实现可观测性和监控

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/prompt/cozeloop
```

## 前置条件

在使用此组件之前，您需要：

1. 设置 CozeLoop 账户和工作空间
2. 在 CozeLoop 提示词管理系统中创建提示词
3. 获取身份验证凭据（API token 或 JWT OAuth 凭据）

## 配置

设置以下环境变量：

```bash
# 必需
export COZELOOP_WORKSPACE_ID=your-workspace-id

# 选项 1：使用 API Token（仅推荐用于测试）
export COZELOOP_API_TOKEN=your-api-token

# 选项 2：使用 JWT OAuth（推荐用于生产环境）
export COZELOOP_JWT_OAUTH_CLIENT_ID=your-client-id
export COZELOOP_JWT_OAUTH_PRIVATE_KEY=your-private-key
export COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID=your-public-key-id
```

## 使用方法

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/prompt/cozeloop"
	cozeloopgo "github.com/coze-dev/cozeloop-go"
)

func main() {
	ctx := context.Background()

	// 初始化 CozeLoop 客户端
	client, err := cozeloopgo.NewClient()
	if err != nil {
		log.Fatalf("Failed to create CozeLoop client: %v", err)
	}
	defer client.Close(ctx)

	// 创建 PromptHub 组件
	ph, err := cozeloop.NewPromptHub(ctx, &cozeloop.Config{
		Key:            "your.prompt.key", // 从 CozeLoop 获取的提示词 key
		Version:        "",                // 空字符串表示使用最新版本
		CozeLoopClient: client,
	})
	if err != nil {
		log.Fatalf("Failed to create PromptHub: %v", err)
	}

	// 使用变量格式化提示词
	variables := map[string]any{
		"user_name": "John Doe",
		"topic":     "example topic",
	}

	messages, err := ph.Format(ctx, variables)
	if err != nil {
		log.Fatalf("Failed to format prompt: %v", err)
	}

	// 使用格式化后的消息
	for _, msg := range messages {
		fmt.Printf("Role: %s, Content: %s\n", msg.Role, msg.Content)
	}
}
```

### 多模态内容支持

该组件支持提示词变量中的多模态内容（文本 + 图像）。您可以传递 `[]*entity.ContentPart` 作为变量值：

```go
import "github.com/coze-dev/cozeloop-go/entity"

// 用于创建字符串指针的辅助函数
func strPtr(s string) *string {
	return &s
}

// 创建包含文本和图像的多模态内容
multiModalContent := []*entity.ContentPart{
	{
		Type: entity.ContentTypeText,
		Text: strPtr("Describe this image: "),
	},
	{
		Type:     entity.ContentTypeImageURL,
		ImageURL: strPtr("https://example.com/image.png"),
	},
	{
		Type: entity.ContentTypeText,
		Text: strPtr(" in detail."),
	},
}

variables := map[string]any{
	"user_input": multiModalContent,
}

messages, err := ph.Format(ctx, variables)
// 格式化后的消息将包含 UserInputMultiContent 或 AssistantGenMultiContent
// 具体取决于提示词模板中的消息角色
```

**支持的内容类型：**
- `entity.ContentTypeText` - 纯文本内容
- `entity.ContentTypeImageURL` - 通过 URL 引用的图像
- `entity.ContentTypeBase64Data` - base64 编码的图像

**注意：** 多模态内容支持用户消息和助手消息（例如，用于提示词模板中的少样本示例）。

## API 参考

### NewPromptHub

创建一个新的 PromptHub 实例。

```go
func NewPromptHub(ctx context.Context, conf *Config) (prompt.ChatTemplate, error)
```

**Config 字段：**
- `Key`（string，必需）：从 CozeLoop 获取的提示词 key
- `Version`（string，可选）：提示词的特定版本。空字符串获取最新版本
- `CozeLoopClient`（cozeloop.Client，必需）：已初始化的 CozeLoop 客户端

### Format

使用提供的变量格式化提示词。

```go
func (p *promptHub) Format(ctx context.Context, vs map[string]any, opts ...prompt.Option) ([]*schema.Message, error)
```

**参数：**
- `ctx`：操作的上下文
- `vs`：用于提示词替换的变量名称到值的映射
- `opts`：可选的提示词格式化选项

**返回：**
- 格式化后的消息切片
- 如果格式化失败则返回错误

### GetType

返回组件类型标识符。

```go
func (p *promptHub) GetType() string
```

**返回：** "PromptHub"

## 与 Eino 集成

此组件可以无缝集成到 Eino 工作流中：

```go
import (
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino-ext/components/prompt/cozeloop"
)

// 在链中使用
chain := compose.NewChain[map[string]any, []*schema.Message]()
chain.AppendChatTemplate(ph)
```

## 错误处理

在以下情况下，组件将返回错误：
- 创建 CozeLoop 客户端失败
- 配置无效（配置或客户端为 nil）
- 提示词未找到或为空
- 提示词格式化失败
- 网络或 API 错误

## 可观测性

该组件支持 Eino 的回调系统以实现可观测性：

```go
import (
	"github.com/cloudwego/eino/callbacks"
	ccb "github.com/cloudwego/eino-ext/callbacks/cozeloop"
)

// 添加 CozeLoop 回调处理器
client, _ := cozeloop.NewClient()
handler := ccb.NewLoopHandler(client)
callbacks.AppendGlobalHandlers(handler)
```

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。

## 相关项目

- [Eino](https://github.com/cloudwego/eino) - Eino 主框架
- [CozeLoop Go SDK](https://github.com/coze-dev/cozeloop-go) - 官方 CozeLoop Go SDK
- [Eino Extensions](https://github.com/cloudwego/eino-ext) - Eino 扩展集合
