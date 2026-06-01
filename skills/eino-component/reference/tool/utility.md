# Utility Tools

## HTTP Request

```go
import "github.com/cloudwego/eino-ext/components/tool/httprequest"

tool, err := httprequest.NewTool(ctx, &httprequest.Config{})
// Makes HTTP requests based on model-generated parameters
```

## Command Line

```go
import "github.com/cloudwego/eino-ext/components/tool/commandline"

tool, err := commandline.NewTool(ctx, &commandline.Config{})
// Executes shell commands
```

## Browser Use

```go
import "github.com/cloudwego/eino-ext/components/tool/browseruse"

tool, err := browseruse.NewTool(ctx, &browseruse.Config{})
// Browser automation tool
```
