# 本地后端

一个用于 EINO ADK 的文件系统后端，使用标准 Go 包直接在本地机器的文件系统上操作。

## 快速开始

### 安装

```bash
go get github.com/cloudwego/eino-ext/adk/backend/local
```

#### `MultiModalRead` 的 PDF 渲染

`MultiModalRead` 通过 [`klippa-app/go-pdfium`](https://github.com/klippa-app/go-pdfium)
的 **WebAssembly** 后端（由 [`wazero`](https://github.com/tetratelabs/wazero) 在进程内执行）
将 PDF 页面光栅化。**无需 CGO 工具链，也无需 MuPDF/PDFium 等系统级原生库**，在 Linux、
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
    "github.com/cloudwego/eino-ext/adk/backend/local"
    "github.com/cloudwego/eino/adk/middlewares/filesystem"
)

// 初始化后端
backend, err := local.NewBackend(context.Background(), &local.Config{})
if err != nil {
    panic(err)
}

// 写入文件
err = backend.Write(ctx, &filesystem.WriteRequest{
    FilePath: "/path/to/file.txt",
    Content:  "Hello, World!",
})

// 读取文件
content, err := backend.Read(ctx, &filesystem.ReadRequest{
    FilePath: "/path/to/file.txt",
})
```

## 功能特性

- **零配置** - 开箱即用，无需设置
- **直接文件系统访问** - 使用本地性能操作本地文件
- **完整后端实现** - 支持所有 `filesystem.Backend` 操作
- **路径安全** - 强制使用绝对路径以防止目录遍历
- **多模态读取** - 将图片和 PDF 读取为结构化的多模态片段（PDF 支持整文或分页渲染）

## 配置

```go
type Config struct {
    // 可选：Execute() 方法安全性的命令验证器
    // 建议在生产环境中使用以防止命令注入
    ValidateCommand func(string) error

    // 可选：MultiModalRead 的图片/PDF/DPI 限制。
    // 字段为 0 或负数时使用默认值；超过硬上限时会被静默截断到上限。
    MultiModalRead MultiModalReadConfig
}

type MultiModalReadConfig struct {
    MaxImageSizeMB        int           // 单张图片读取大小上限（MB）。   默认 10，  硬上限 2048
    MaxPDFSizeMB          int           // 整文 PDF 读取大小上限（MB）。 默认 20,  硬上限 2048
    MaxPagedPDFSizeMB     int           // 分页 PDF 读取大小上限（MB）。 默认 100, 硬上限 2048
    MaxPDFPagesPerRequest int           // 单次分页读取的最大页数。      默认 20,  硬上限 1000
    PDFRenderDPI          int           // PDF 页面光栅化时使用的 DPI。  默认 150, 硬上限 600

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

### 命令验证示例

```go
func validateCommand(cmd string) error {
    allowed := map[string]bool{"ls": true, "cat": true, "grep": true}
    parts := strings.Fields(cmd)
    if len(parts) == 0 || !allowed[parts[0]] {
        return fmt.Errorf("command not allowed: %s", cmd)
    }
    return nil
}

backend, _ := local.NewBackend(ctx, &local.Config{
    ValidateCommand: validateCommand,
})
```

## 示例

查看以下示例了解更多用法：

- [后端使用](./examples/backend/)
- [中间件集成](./examples/middlewares/)

## API 参考

### 核心方法

- **`LsInfo(ctx, req)`** - 列出目录内容
- **`Read(ctx, req)`** - 读取文件，支持可选的行偏移/限制
- **`MultiModalRead(ctx, req)`** - 将图片/PDF 读取为结构化的多模态片段；非图片/非 PDF 文件回退到 `Read`。默认值：图片 10 MB / 整文 PDF 20 MB / 分页 PDF 100 MB，单次最多 20 页 @ 150 DPI。可通过 `Config.MultiModalRead` 调优。`Pages` 支持单页（`"3"`）或包含范围（`"1-5"`）。
- **`Write(ctx, req)`** - 写入文件内容；文件不存在时创建，否则**覆盖**现有内容（父目录会自动创建）。
- **`Edit(ctx, req)`** - 在文件中搜索和替换
- **`GrepRaw(ctx, req)`** - 在文件中搜索模式
- **`GlobInfo(ctx, req)`** - 按 glob 模式查找文件，跟随目录软链接并防止循环遍历

### 其他方法

- **`Execute(ctx, req)`** - 执行 shell 命令（需要验证）
- **`ExecuteStreaming(ctx, req)`** - 流式输出执行

**注意：** 所有路径必须是绝对路径。使用 `filepath.Abs()` 转换相对路径。

## 安全

### 最佳实践

- ✅ 在文件操作之前始终验证用户输入
- ✅ 使用绝对路径防止目录遍历
- ✅ 为命令执行实现 `ValidateCommand`
- ✅ 使用最小必要权限运行
- ✅ 在生产环境中监控文件系统操作

### 命令注入防护

`Execute()` 方法需要命令验证：

```go
// 不好：没有验证
backend, _ := local.NewBackend(ctx, &local.Config{})
// 有命令注入风险！

// 好：有验证
backend, _ := local.NewBackend(ctx, &local.Config{
    ValidateCommand: myValidator,
})
```

## 常见问题

**问：为什么所有路径都需要是绝对路径？**  
答：这可以防止目录遍历攻击。使用 `filepath.Abs()` 转换相对路径。

**问：可以在生产环境中使用吗？**  
答：可以，但要确保进行适当的输入验证、命令验证和适当的权限设置。


## 许可证

根据 Apache License 2.0 许可。有关详细信息，请参阅 [LICENSE](../../LICENSE) 文件。
