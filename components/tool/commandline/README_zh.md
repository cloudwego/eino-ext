# CommandLine Tool

为 [Eino](https://github.com/cloudwego/eino) 实现的 CommandLine Tools 组件，实现了 `Tool` 接口。这使得能够无缝集成 Eino 的 LLM 功能，以增强自然语言处理和生成能力。
> **注意**：此实现受到 [OpenManus](https://github.com/mannaandpoem/OpenManus) 项目的启发和参考。

## 特性

- 实现 `github.com/cloudwego/eino/components/tool.InvokableTool`
- 易于与 Eino 的工具系统集成
- 支持在 Docker 容器中执行命令行指令

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/commandline@latest
```

## 快速开始

以下是如何使用 commandline 工具的快速示例：

Editor:
```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/tool/commandline"
	"github.com/cloudwego/eino-ext/components/tool/commandline/sandbox"
)

func main() {
	ctx := context.Background()

	op, err := sandbox.NewDockerSandbox(ctx, &sandbox.Config{})
	if err != nil {
		log.Fatal(err)
	}
	// 在创建 docker 容器之前，您应该确保 docker 已经启动
	err = op.Create(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer op.Cleanup(ctx)

	sre, err := commandline.NewStrReplaceEditor(ctx, &commandline.Config{Operator: op})
	if err != nil {
		log.Fatal(err)
	}

	info, err := sre.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tool name: %s, tool desc: %s", info.Name, info.Desc)

	content := "hello world"

	log.Println("create file[test.txt]...")
	result, err := sre.Execute(ctx, &commandline.StrReplaceEditorParams{
		Command:  commandline.CreateCommand,
		Path:     "./test.txt",
		FileText: &content,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("create file result: ", result)

	log.Println("view file[test.txt]...")
	result, err = sre.Execute(ctx, &commandline.StrReplaceEditorParams{
		Command: commandline.ViewCommand,
		Path:    "./test.txt",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("view file result: ", result)
}
```
PyExecutor:
```go
/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/tool/commandline"
	"github.com/cloudwego/eino-ext/components/tool/commandline/sandbox"
)

func main() {
	ctx := context.Background()
	op, err := sandbox.NewDockerSandbox(ctx, &sandbox.Config{})
	if err != nil {
		log.Fatal(err)
	}
	// 在创建 docker 容器之前，您应该确保 docker 已经启动
	err = op.Create(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer op.Cleanup(ctx)

	exec, err := commandline.NewPyExecutor(ctx, &commandline.PyExecutorConfig{Operator: op}) // 默认使用 python3
	if err != nil {
		log.Fatal(err)
	}

	info, err := exec.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tool name: %s,  tool desc: %s", info.Name, info.Desc)

	code := "print(\"hello world\")"
	log.Printf("execute code:\n%s", code)
	result, err := exec.Execute(ctx, &commandline.Input{Code: code})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("result:\n", result)
}
```


## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
## 示例

查看以下示例了解更多用法：

- [代码编辑器](./examples/editor/)
- [Python 执行器](./examples/pyexecutor/)

