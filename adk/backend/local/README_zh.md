# 本地后端

一个用于 EINO ADK 的文件系统后端，使用标准 Go 包直接在本地机器的文件系统上操作。

## 快速开始

### 安装

```bash
go get github.com/cloudwego/eino-ext/adk/backend/local
```

#### `MultiModalRead` 的原生依赖（PDF 渲染）

`MultiModalRead` 通过 [`go-fitz`](https://github.com/gen2brain/go-fitz) 将 PDF 页面光栅化,
该库通过 `purego` 在运行时加载 MuPDF。运行前请安装 MuPDF：

- macOS: `brew install mupdf`
- Ubuntu/Debian: `sudo apt-get install -y libmupdf-dev`
- CentOS/RHEL: `sudo yum install -y mupdf-devel`

如果不使用 `MultiModalRead`，则运行时不需要 MuPDF。

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
- **感知软链接的发现能力** - 基于遍历的操作默认跟随软链接目录
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
    MaxImageSizeMB        int     // 单张图片读取大小上限（MB）。   默认 10，  硬上限 2048
    MaxPDFSizeMB          int     // 整文 PDF 读取大小上限（MB）。 默认 20，  硬上限 2048
    MaxPagedPDFSizeMB     int     // 分页 PDF 读取大小上限（MB）。 默认 100， 硬上限 2048
    MaxPDFPagesPerRequest int     // 单次分页读取的最大页数。      默认 20，  硬上限 1000
    PDFRenderDPI          float64 // PDF 页面光栅化时使用的 DPI。  默认 150， 硬上限 600
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
- **`GrepRaw(ctx, req)`** - 在文件中搜索模式；默认跟随软链接目录
- **`GlobInfo(ctx, req)`** - 按 glob 模式查找文件；默认跟随软链接目录，并防止软链接循环

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
