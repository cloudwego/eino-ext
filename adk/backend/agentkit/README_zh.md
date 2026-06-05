# Ark Sandbox 后端

一个用于 EINO ADK 的安全文件系统后端，在火山引擎的隔离 Ark Sandbox 环境中执行操作。

## 快速开始

### 安装

```bash
go get github.com/cloudwego/eino-ext/adk/backend/agentkit
```

#### `MultiModalRead` 的 PDF 渲染

`MultiModalRead` 通过 [`klippa-app/go-pdfium`](https://github.com/klippa-app/go-pdfium)
的 **WebAssembly** 后端（由 [`wazero`](https://github.com/tetratelabs/wazero) 在进程内执行）
光栅化 PDF 页面。**无需 CGO 工具链，也无需 MuPDF/PDFium 等系统级原生库**，在 Linux、
macOS 和 Windows 上开箱即用。

行为说明：

- 进程内会在第一次分页 PDF 请求时延迟初始化一个全局 PDFium worker pool（首次约几百
  毫秒），后续调用复用。每个 WASM worker 占用约数十 MB 内存，默认 `MaxTotal=max(NumCPU, 2)`。
- pool 在整个进程内是单例；若第二个 backend 传入不同的 `PDFiumPool` sizing，第二份配置
  会被忽略并打印 `WARN` 日志。
- `agentkit` 与 `local` backend 分别属于独立 Go module，因此 **各自维护一份** 进程级 pool。
  同时引入两个 backend 的应用会运行两套 pdfium WASM runtime。
- 可通过 `MultiModalReadConfig.PDFiumPool` 调整 pool 大小（见下文）。

### 基本用法

```go
import (
    "context"
    "os"
    "time"
    
    "github.com/cloudwego/eino-ext/adk/backend/agentkit"
    "github.com/cloudwego/eino/adk/middlewares/filesystem"
)

// 使用凭证配置
config := &agentkit.Config{
    AccessKeyID:     os.Getenv("VOLC_ACCESS_KEY_ID"),
    SecretAccessKey: os.Getenv("VOLC_SECRET_ACCESS_KEY"),
    ToolID:          os.Getenv("VOLC_TOOL_ID"),
    UserSessionID:   "session-" + time.Now().Format("20060102-150405"),
    Region:          agentkit.RegionOfBeijing,
}

// 初始化后端
backend, err := agentkit.NewSandboxToolBackend(config)
if err != nil {
    panic(err)
}

// 使用后端
backend.Write(ctx, &filesystem.WriteRequest{
    FilePath: "/home/gem/file.txt",
    Content:  "Hello, Sandbox!",
})
```

## 功能特性

- **安全执行** - 所有操作在隔离的沙箱环境中运行
- **会话管理** - 支持会话隔离，可配置 TTL
- **完整后端实现** - 支持所有 `filesystem.Backend` 操作
- **请求签名** - 使用火山引擎自动进行 AK/SK 身份验证
- **远程执行** - 基于 Python 的沙箱操作，支持结果流式传输

## 配置

```go
type Config struct {
    // 必需：火山引擎凭证
    AccessKeyID     string
    SecretAccessKey string
    ToolID          string  // 从火山引擎控制台获取的沙箱工具 ID
    UserSessionID   string  // 用于隔离的唯一会话 ID
    
    // 可选：提供默认值
    Region        Region        // 默认：RegionOfBeijing
    SessionTTL    int          // 默认：1800 秒（30 分钟）
    ExecutionTimeout int       
    Timeout       time.Duration // HTTP 客户端超时

    // 可选：MultiModalRead 的图片/PDF/DPI 阈值。
    // 零值或负值字段回退为默认值；超过硬上限的值会被静默截断。
    MultiModalRead MultiModalReadConfig
}

type MultiModalReadConfig struct {
    MaxImageSizeMB        int           // 图片读取大小上限（MB）。       默认 10，硬上限 2048
    MaxPDFSizeMB          int           // PDF 全量读取大小上限（MB）。   默认 20，硬上限 2048
    MaxPagedPDFSizeMB     int           // PDF 分页读取大小上限（MB）。   默认 100，硬上限 2048
    MaxPDFPagesPerRequest int           // 单次分页读取最多页数。         默认 20，硬上限 1000
    PDFRenderDPI          int           // PDF 页面渲染 DPI。            默认 150，硬上限 600

    // PDFiumPool 用于调整分页 PDF 渲染所使用的进程级 PDFium worker pool。
    // 仅在首次延迟初始化时生效；后续调用方传入不同 sizing 会触发 WARN 日志，沿用已有 pool。
    PDFiumPool PDFiumPoolConfig

    // PDFiumAcquireTimeout 限制调用方 ctx 无 deadline 时获取 pdfium worker 的等待上限。
    // 是 per-read 配置（不同调用方可使用不同值）。默认 30s。
    PDFiumAcquireTimeout time.Duration
}

type PDFiumPoolConfig struct {
    MinIdle  int // 保持存活的最小空闲 worker 数。      默认 1
    MaxIdle  int // 保持存活的最大空闲 worker 数。      默认 2
    MaxTotal int // 最大 worker 数（>= 2）。            默认 max(2, NumCPU)
}
```

### 环境设置

```bash
# 将凭证设置为环境变量
export VOLC_ACCESS_KEY_ID="your_access_key"
export VOLC_SECRET_ACCESS_KEY="your_secret_key"
export VOLC_TOOL_ID="your_tool_id"
```

**获取凭证：**
1. 登录 [火山引擎控制台](https://console.volcengine.com/)
2. 导航到 IAM → 访问密钥
3. 在 Ark 平台创建 Ark Sandbox 工具
4. 复制凭证和工具 ID

## 示例

查看以下示例了解更多用法：

- [后端使用](./examples/backend/)
- [中间件集成](./examples/middlewares/)

## API 参考

### 核心方法

- **`LsInfo(ctx, req)`** - 列出目录内容
- **`Read(ctx, req)`** - 读取文件，支持可选的行偏移/限制
- **`MultiModalRead(ctx, req)`** - 将图片/PDF 读取为结构化的多模态 parts；非图片/非 PDF 文件回退到 `Read`。默认值：图片 10 MB / PDF 20 MB / 分页 PDF 100 MB 最多 20 页 @ 150 DPI。可通过 `Config.MultiModalRead` 调整。`Pages` 字段支持单页（`"3"`）或闭区间（`"1-5"`）。
- **`Write(ctx, req)`** - 写入文件内容；文件不存在时创建，存在时**直接覆盖**（父级目录会自动创建）。
- **`Edit(ctx, req)`** - 在文件中搜索和替换
- **`GrepRaw(ctx, req)`** - 在文件中搜索模式
- **`GlobInfo(ctx, req)`** - 按 glob 模式查找文件

**注意：** 使用 `/home/gem` 目录进行文件操作。默认的 `gem` 用户对系统路径的权限有限。

## 安全

### 最佳实践

- ✅ 将凭证存储在环境变量中，而不是代码中
- ✅ 为每个执行上下文使用唯一的会话 ID
- ✅ 设置适当的超时以防止资源耗尽
- ✅ 在生产环境中监控沙箱资源使用情况
- ✅ 实现适当的错误处理和重试逻辑



## 故障排除

**身份验证错误**
- 验证凭证是否正确
- 检查环境变量是否已设置
- 确保火山引擎账号具有 Ark Sandbox 访问权限

**超时错误**
- 增加配置中的 `Timeout` 或 `ExecutionTimeout`
- 检查网络连接
- 验证 Ark Sandbox 服务可用性

## 常见问题

**问：与本地后端有什么区别？**  
答：Ark Sandbox 在隔离的远程环境中执行（安全、沙箱化）。本地后端直接访问本地文件系统（快速、简单）。

**问：可以在生产环境中使用吗？**  
答：可以。确保进行适当的错误处理、超时设置、唯一会话 ID 和资源监控。

**问：有速率限制吗？**  
答：限制取决于您的火山引擎账号等级。有关详细信息，请查看火山引擎文档。

**问：会话持续多长时间？**  
答：默认为 30 分钟（1800 秒）。使用 `SessionTTL` 配置（范围：60-86400 秒）。

## 许可证

根据 Apache License 2.0 许可。有关详细信息，请参阅 [LICENSE](../../LICENSE) 文件。
