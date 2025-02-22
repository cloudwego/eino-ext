# 火山引擎 APMPlus 回调

[English](README.md) | 简体中文

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 火山引擎 APMPlus 回调。该工具实现了 `Handler` 接口，可以与 Eino 的应用无缝集成以提供增强的可观测能力。

## 特性

- 实现了 `github.com/cloudwego/eino/internel/callbacks.Handler` 接口
- 易于与 Eino 应用集成

## 安装

```bash
go get github.com/cloudwego/eino-ext/callbacks/apmplus
```

## 快速开始

```go
package main

import (
	"github.com/cloudwego/eino-ext/callbacks/apmplus"
	"github.com/cloudwego/eino/callbacks"
)

func main() {
	// 创建apmplus handler
	cbh, showdown := apmplus.NewApmplusHandler(&apmplus.Config{
		Host: "apmplus-cn-beijing.volces.com:4317",
		AppKey:      "appkey-xxx",
		ServiceName: "app",
		Release:     "release/v0.0.1",
	})

	// 设置apmplus为全局callback
	callbacks.InitCallbackHandlers([]callbacks.Handler{cbh})

	g := NewGraph[string,string]()
	/*
	 * compose and run graph
	 */

	// 等待所有trace和metrics上报完成后退出
	showdown()
}
```

## 配置

回调可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // 上报地址，用于观测指标上报，可从apmplus产品页面/文档获取 (必填)
    // 例子: "https://apmplus-cn-beijing.volces.com:4317"
    Host string
    
    // 认证信息，可从apmplus产品页面获取 (必填)
    // 例子: "abc..."
    AppKey string
    
    // 服务名称 (必填)
    // 例子: "my-app"
    ServiceName string
    
    // 版本信息 (选填)
    // 默认值: ""
    // 例子: "v1.2.3"
    Release string
}
```

## 更多详情

- [火山引擎 APMPlus 文档](https://www.volcengine.com/docs/6431/69092)
- [Eino 文档](https://github.com/cloudwego/eino) 