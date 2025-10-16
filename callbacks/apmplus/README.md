# Volcengine APMPlus Callbacks

English | [简体中文](README_zh.md)

A Volcengine APMPlus callback implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Handler` interface. This enables seamless integration with Eino's application for enhanced observability.

## Features

- Implements `github.com/cloudwego/eino/internel/callbacks.Handler` interface
- Implements session functionality to associate multiple requests in a single session
- Easy integration with Eino's application

## Installation

```bash
go get github.com/cloudwego/eino-ext/callbacks/apmplus
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/callbacks/apmplus"
	"github.com/cloudwego/eino/callbacks"
)

func main() {
	ctx := context.Background()
    // Create apmplus handler
	cbh, showdown, err := apmplus.NewApmplusHandler(&apmplus.Config{
		Host: "apmplus-cn-beijing.volces.com:4317",
		AppKey:      "appkey-xxx",
		ServiceName: "eino-app",
		Release:     "release/v0.0.1",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Set apmplus as a global callback
	callbacks.AppendGlobalHandlers(cbh)
	
	g := NewGraph[string,string]()
	/*
	 * compose and run graph
	 */
	runner, _ := g.Compile(ctx)
	// To set session information, use apmplus.SetSession method
	ctx = apmplus.SetSession(ctx, apmplus.WithSessionID("your_session_id"), apmplus.WithUserID("your_user_id"))
	// Execute the runner
	result, _ := runner.Invoke(ctx, "input")
	/*
	 * Process the result
	 */
	 
	// Exit after all trace and metrics reporting is complete
	showdown(ctx)
}
```

## Configuration

The callback can be configured using the `Config` struct:

```go
type Config struct {
    // Host is the Apmplus server URL (Required)
    // Example: "https://apmplus-cn-beijing.volces.com:4317"
    Host string
    
    // AppKey is the key for authentication (Required)
    // Example: "abc..."
    AppKey string
    
    // ServiceName is the name of service (Required)
    // Example: "my-app"
    ServiceName string
    
    // Release is the version or release identifier (Optional)
    // Default: ""
    // Example: "v1.2.3"
    Release string
}
```

## For More Details

- [Volcengine APMPlus Documentation](https://www.volcengine.com/docs/6431/69092)
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
