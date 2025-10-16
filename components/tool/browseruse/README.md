# BrowserUse Tool

A BrowserUse Tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Tool` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.
> **Note**: This implementation is inspired by and references the [OpenManus](https://github.com/mannaandpoem/OpenManus) project.

## Features

- Implements `github.com/cloudwego/eino/components/tool.BaseTool`
- Easy integration with Eino's tool system
- Support for executing browser actions

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/browseruse@latest
```

## Quick Start

Here's a quick example of how to use the browser-use tool:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/browseruse"
)

func main() {
	ctx := context.Background()
	but, err := browseruse.NewBrowserUseTool(ctx, &browseruse.Config{})
	if err != nil {
		log.Fatal(err)
	}

	url := "https://www.google.com"
	result, err := but.Execute(&browseruse.Param{
		Action: browseruse.ActionGoToURL,
		URL:    &url,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
	time.Sleep(10 * time.Second)
	but.Cleanup()
}

```

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
