# Sequential Thinking Tool

**Sequential Thinking Tool**
是一个用于支持动态和反思性问题解决的工具，通过结构化的思考流程帮助用户逐步分析并解决问题。灵感来自于[@modelcontextprotocol
/sequentialthinking](https://github.com/modelcontextprotocol/servers/tree/HEAD/src/sequentialthinking)
,它通过一系列问题引导大语言模型逐步思考问题。

## 功能特性

- 指导性的逐步思考流程。
- 动态提问与自我反思。
- 提升问题解决能力。

## 使用场景

Sequential Thinking 工具旨在用于：

- 将复杂问题分解为步骤
- 规划和设计，留有修改空间
- 分析可能需要纠正的方向
- 问题可能在最初时无法完全清楚其范围
- 任务需要在多个步骤中保持上下文
- 需要过滤不相关信息的情况

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/sequentialthinking@latest
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	
	"github.com/bytedance/sonic"
	
	"github.com/cloudwego/eino-ext/components/tool/sequentialthinking"
)

func main() {
	ctx := context.Background()
	
	// Instantiate the tool
	tool, err := sequentialthinking.NewTool()
	if err != nil {
		panic(err)
	}
	
	args := &sequentialthinking.ThoughtRequest{
		Thought:           "This is a test thought",
		ThoughtNumber:     1,
		TotalThoughts:     3,
		NextThoughtNeeded: true,
	}
	
	argsStr, _ := sonic.Marshal(args)
	
	// Use the tool
	// (This is just a placeholder; actual usage will depend on the tool's functionality)
	result, err := tool.InvokableRun(ctx, string(argsStr))
	if err != nil {
		panic(err)
	}
	
	// Process the result
	// (This is just a placeholder; actual processing will depend on the tool's output)
	fmt.Println(result)
}
```