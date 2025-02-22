# Volcengine APMPlus Callbacks

English | [简体中文](README_zh.md)

A Volcengine APMPlus callback implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Handler` interface. This enables seamless integration with Eino's application for enhanced observability.

## Features

- Implements `github.com/cloudwego/eino/internel/callbacks.Handler`
- Easy integration with Eino's application

## Installation

```bash
go get github.com/cloudwego/eino-ext/callbacks/apmplus
```

## Quick Start

```go
package main

import (
	"github.com/cloudwego/eino-ext/callbacks/apmplus"
	"github.com/cloudwego/eino/callbacks"
)

func main() {
    // Create apmplus handler
	cbh, showdown := apmplus.NewApmplusHandler(&apmplus.Config{
		Host: "apmplus-cn-beijing.volces.com:4317",
		AppKey:      "appkey-xxx",
		ServiceName: "app",
		Release:     "release/v0.0.1",
	})

	// Set apmplus as a global callback
	callbacks.InitCallbackHandlers([]callbacks.Handler{cbh})
	
	g := NewGraph[string,string]()
	/*
	 * compose and run graph
	 */
	
	// Exit after all trace and metrics reporting is complete
	showdown()
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
- [Eino Documentation](https://github.com/cloudwego/eino)
