# 本地后端

一个用于 EINO ADK 的文件系统后端，使用标准 Go 包直接在本地机器的文件系统上操作。

## 快速开始

### 安装

```bash
go get github.com/cloudwego/eino-ext/adk/backend/local
```

### 基本用法

```go
import (
    "context"
    "github.com/cloudwego/eino-ext/adk/backend/local"
    "github.com/cloudwego/eino/adk/middlewares/filesystem"
)

// 初始化后端
backend, err := local.NewLocalBackend(context.Background(), &local.Config{})
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
- **安全写入** - 默认情况下防止意外覆盖文件

## 配置

```go
type Config struct {
    // 可选：Execute() 方法安全性的命令验证器
    // 建议在生产环境中使用以防止命令注入
    ValidateCommand func(string) error
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

backend, _ := local.NewLocalBackend(ctx, &local.Config{
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
- **`Write(ctx, req)`** - 创建新文件（如果存在则失败）
- **`Edit(ctx, req)`** - 在文件中搜索和替换
- **`GrepRaw(ctx, req)`** - 在文件中搜索模式
- **`GlobInfo(ctx, req)`** - 按 glob 模式查找文件

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
backend, _ := local.NewLocalBackend(ctx, &local.Config{})
// 有命令注入风险！

// 好：有验证
backend, _ := local.NewLocalBackend(ctx, &local.Config{
    ValidateCommand: myValidator,
})
```

## 常见问题

**问：为什么所有路径都需要是绝对路径？**  
答：这可以防止目录遍历攻击。使用 `filepath.Abs()` 转换相对路径。

**问：为什么 Write 在文件存在时会失败？**  
答：这是一个安全功能，可以防止意外的数据丢失。使用 `Edit()` 修改现有文件。

**问：可以在生产环境中使用吗？**  
答：可以，但要确保进行适当的输入验证、命令验证和适当的权限设置。


## 许可证

根据 Apache License 2.0 许可。有关详细信息，请参阅 [LICENSE](../../LICENSE) 文件。
