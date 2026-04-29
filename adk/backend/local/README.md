# Local Backend

A filesystem backend for EINO ADK that operates directly on the local machine's filesystem using standard Go packages.

## Quick Start

### Installation

```bash
go get github.com/cloudwego/eino-ext/adk/backend/local
```

### Basic Usage

```go
import (
    "context"
    "github.com/cloudwego/eino-ext/adk/backend/local"
    "github.com/cloudwego/eino/adk/filesystem"
)

// Initialize backend
backend, err := local.NewLocalBackend(context.Background(), &local.Config{})
if err != nil {
    panic(err)
}

// Write a file
err = backend.Write(ctx, &filesystem.WriteRequest{
    FilePath: "/path/to/file.txt",
    Content:  "Hello, World!",
})

// Read a file
content, err := backend.Read(ctx, &filesystem.ReadRequest{
    FilePath: "/path/to/file.txt",
})
```

## Features

- **Zero Configuration** - Works out of the box with no setup required
- **Direct Filesystem Access** - Operates on local files with native performance
- **Full Backend Implementation** - Supports all `filesystem.Backend` operations
- **MultiModal Read** - Returns structured image/PDF content for multimodal models; non-image/PDF files fall back to plain text `Read`
- **Path Security** - Enforces absolute paths to prevent directory traversal
- **Safe Write** - Prevents accidental file overwrites by default

## Configuration

```go
type Config struct {
    // Optional: Command validator for Execute() method security
    // Recommended for production use to prevent command injection
    ValidateCommand func(string) error
}
```

### Command Validation Example

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

## Examples

See the following examples for more usage:

- [Backend Usage](./examples/backend/)
- [Middleware Integration](./examples/middlewares/)

## API Reference

### Core Methods

- **`LsInfo(ctx, req)`** - List directory contents
- **`Read(ctx, req)`** - Read file with optional line offset/limit
- **`Write(ctx, req)`** - Create new file (fails if exists)
- **`Edit(ctx, req)`** - Search and replace in file
- **`GrepRaw(ctx, req)`** - Search pattern in files
- **`GlobInfo(ctx, req)`** - Find files by glob pattern

### Additional Methods

- **`MultiModalRead(ctx, req)`** - Read file as multimodal content (images / PDFs). Non-image/PDF files delegate to `Read`.
- **`Execute(ctx, req)`** - Execute shell command (requires validation)
- **`ExecuteStreaming(ctx, req)`** - Execute with streaming output

**Note:** All paths must be absolute. Use `filepath.Abs()` to convert relative paths.

## MultiModalRead

`MultiModalRead` returns structured parts suitable for multimodal model input.

Supported file types:

- **Images**: `.jpg` / `.jpeg` / `.png` / `.gif` / `.bmp` / `.webp` / `.tiff` / `.tif` — returned as an `image` part with detected MIME type.
- **PDF**:
  - Without `Pages`: the full PDF is returned as a `pdf` part.
  - With `Pages` (e.g. `"1"`, `"1-5"`): the specified page range is rendered to PNG (150 DPI) and returned as `image` parts.
- **Other files**: fall back to `Read`, returned via `MultiFileContent.FileContent`.

Size and page limits:

| Scenario         | Limit              |
| ---------------- | ------------------ |
| Image            | 10 MB              |
| PDF (full read)  | 20 MB              |
| PDF (paged read) | 100 MB, 20 pages per request |

Files exceeding the limit are rejected up-front based on `os.Stat` size, and the returned error message includes the actual size and limit. For oversize PDFs, the error message suggests using `Pages` to switch to paged reading.

### PDF Rendering Dependency

PDF page rendering is provided by [`go-fitz`](https://github.com/gen2brain/go-fitz), which uses MuPDF via `purego`/FFI (no classic CGO). The native library must be installed on the build/run machine:

- macOS: `brew install mupdf`
- Ubuntu / Debian: `apt-get install -y libmupdf-dev`
- CentOS / RHEL: `yum install -y mupdf-devel`

### Example

```go
res, err := backend.MultiModalRead(ctx, &filesystem.MultiModalReadRequest{
    ReadRequest: filesystem.ReadRequest{FilePath: "/path/to/page.pdf"},
    Pages:       "1-3",
})
if err != nil {
    // handle error
}
for _, part := range res.Parts {
    // part.Type: "image" | "pdf"
    // part.MIMEType: "image/png", "application/pdf", ...
    // part.Data: raw bytes
}
```

## Security

### Best Practices

- ✅ Always validate user input before file operations
- ✅ Use absolute paths to prevent directory traversal
- ✅ Implement `ValidateCommand` for command execution
- ✅ Run with minimal necessary permissions
- ✅ Monitor filesystem operations in production

### Command Injection Prevention

The `Execute()` method requires command validation:

```go
// Bad: No validation
backend, _ := local.NewLocalBackend(ctx, &local.Config{})
// Command injection risk!

// Good: With validation
backend, _ := local.NewLocalBackend(ctx, &local.Config{
    ValidateCommand: myValidator,
})
```

## FAQ

**Q: Why do all paths need to be absolute?**  
A: This prevents directory traversal attacks. Use `filepath.Abs()` to convert relative paths.

**Q: Why does Write fail if the file exists?**  
A: This is a safety feature to prevent accidental data loss. Use `Edit()` to modify existing files.

**Q: Can I use this in production?**  
A: Yes, but ensure proper input validation, command validation, and appropriate permissions.


## License

Licensed under the Apache License, Version 2.0. See the [LICENSE](../../LICENSE) file for details.
