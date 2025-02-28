# DDGSearch

[English](README.md) | 简体中文

一个用于 DuckDuckGo 搜索功能的原生 Go 库。该库提供了一种简单高效的方式来使用 DuckDuckGo 搜索引擎进行搜索。

## 为什么选择 DuckDuckGo？

DuckDuckGo 提供了以下优势：
- **无需认证**：与其他搜索引擎不同，DuckDuckGo 的 API 无需任何 API 密钥或认证
- 注重隐私的搜索结果
- 合理使用范围内无速率限制
- 支持多个地区和语言
- 干净且相关的搜索结果

## 特性

- 简洁且地道的 Go 实现
- 全面的错误处理
- 可配置的搜索参数
- 带 TTL 的内存缓存
- 支持：
  - 多个地区（us-en、uk-en、de-de 等）
  - 安全搜索级别（严格、适中、关闭）
  - 基于时间的过滤（天、周、月、年）
  - 结果分页
  - 自定义 HTTP 头
  - 代理配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/duckduckgo
```

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/cloudwego/eino-ext/components/tool/duckduckgo/ddgsearch"
)

func main() {
    // 创建带配置的新客户端
    cfg := &ddgsearch.Config{
        Timeout:    30 * time.Second,
        MaxRetries: 3,
        Cache:      true,
    }
    client, err := ddgsearch.New(cfg)
    if err != nil {
        log.Fatalf("New of ddgsearch failed, err=%v", err)
    }

    // 配置搜索参数
    params := &ddgsearch.SearchParams{
        Query:      "what is golang",
        Region:     ddgsearch.RegionUSEN,
        SafeSearch: ddgsearch.SafeSearchModerate,
        TimeRange:  ddgsearch.TimeRangeMonth,
        MaxResults: 10,
    }

    // 执行搜索
    response, err := client.Search(context.Background(), params)
    if err != nil {
        log.Fatalf("Search of ddgsearch failed, err=%v", err)
    }

    // 打印结果
    for i, result := range response.Results {
        fmt.Printf("%d. %s\n   URL: %s\n   Description: %s\n\n", 
            i+1, result.Title, result.URL, result.Description)
    }
}
```

## 高级用法

### 配置

```go
// 使用自定义配置创建客户端
cfg := &ddgsearch.Config{
    Timeout:    20 * time.Second,
    MaxRetries: 3,
    Proxy:      "http://proxy:8080",
    Cache:      true,
    Headers: map[string]string{
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
    },
}
client, err := ddgsearch.New(cfg)
```

### 搜索参���

```go
params := &ddgsearch.SearchParams{
    Query:      "golang tutorial",       // 搜索查询
    Region:     ddgsearch.RegionUS,      // 结果地区（us-en、uk-en 等）
    SafeSearch: ddgsearch.SafeSearchModerate, // 安全搜索级别
    TimeRange:  ddgsearch.TimeRangeWeek, // 时间过滤器
    MaxResults: 10,                      // 返回的最大结果数
}
```

可用地区：
- RegionUS（美国）
- RegionUK（英国）
- RegionDE（德国）
- RegionFR（法国）
- RegionJP（日本）
- RegionCN（中国）
- RegionRU（俄罗斯）

安全搜索级别：
- SafeSearchStrict（严格）
- SafeSearchModerate（适中）
- SafeSearchOff（关闭）

时间范围选项：
- TimeRangeDay（天）
- TimeRangeWeek（周）
- TimeRangeMonth（月）
- TimeRangeYear（年）

### 代理支持

```go
// HTTP 代理
cfg := &ddgsearch.Config{
    Proxy: "http://proxy:8080",
}
client, err := ddgsearch.New(cfg)

// SOCKS5 代理
cfg := &ddgsearch.Config{
    Proxy: "socks5://proxy:1080",
}
client, err := ddgsearch.New(cfg)
``` 