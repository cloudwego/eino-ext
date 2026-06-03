# Local Backend

A filesystem backend for EINO ADK that operates directly on the local machine's filesystem using standard Go packages.

## Quick Start

### Installation

```bash
go get github.com/cloudwego/eino-ext/adk/backend/local
```

#### Native dependency for `MultiModalRead` (PDF rendering)

`MultiModalRead` rasterises PDF pages via [`go-fitz`](https://github.com/gen2brain/go-fitz),
which loads MuPDF at runtime through `purego`. Install MuPDF before running:

- macOS: `brew install mupdf`
- Ubuntu/Debian: `sudo apt-get install -y libmupdf-dev`
- CentOS/RHEL: `sudo yum install -y mupdf-devel`

If you don't use `MultiModalRead`, MuPDF is not required at runtime.

### Basic Usage

```go
import (
    "context"
    "github.com/cloudwego/eino-ext/adk/backend/local"
    "github.com/cloudwego/eino/adk/middlewares/filesystem"
)

// Initialize backend
backend, err := local.NewBackend(context.Background(), &local.Config{})
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
- **Path Security** - Enforces absolute paths to prevent directory traversal
- **Multimodal Read** - Reads images and PDFs as structured parts (PDF supports full or paged rendering)

## Configuration

```go
type Config struct {
    // Optional: Command validator for Execute() method security
    // Recommended for production use to prevent command injection
    ValidateCommand func(string) error

    // Optional: image/PDF/DPI limits for MultiModalRead.
    // Zero/negative fields fall back to defaults; values above hard-caps are silently clamped.
    MultiModalRead MultiModalReadConfig
}

type MultiModalReadConfig struct {
    MaxImageSizeMB        int     // image read size limit (MB).      Default 10,  hard-cap 2048
    MaxPDFSizeMB          int     // full PDF read size limit (MB).   Default 20,  hard-cap 2048
    MaxPagedPDFSizeMB     int     // paged PDF read size limit (MB).  Default 100, hard-cap 2048
    MaxPDFPagesPerRequest int     // max pages per paged read.        Default 20,  hard-cap 1000
    PDFRenderDPI          float64 // DPI when rasterising PDF pages.  Default 150, hard-cap 600
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

backend, _ := local.NewBackend(ctx, &local.Config{
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
- **`MultiModalRead(ctx, req)`** - Read images/PDFs as structured multimodal parts; non-image/non-PDF files fall back to `Read`. Defaults: image 10 MB / PDF 20 MB / paged-PDF 100 MB up to 20 pages @ 150 DPI. Tunable via `Config.MultiModalRead`. `Pages` accepts a single page (`"3"`) or an inclusive range (`"1-5"`).
- **`Write(ctx, req)`** - Write file content; creates the file if it doesn't exist, otherwise **overwrites** existing content (parent directories are created automatically).
- **`Edit(ctx, req)`** - Search and replace in file
- **`GrepRaw(ctx, req)`** - Search pattern in files
- **`GlobInfo(ctx, req)`** - Find files by glob pattern

### Additional Methods

- **`Execute(ctx, req)`** - Execute shell command (requires validation)
- **`ExecuteStreaming(ctx, req)`** - Execute with streaming output

**Note:** All paths must be absolute. Use `filepath.Abs()` to convert relative paths.

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
backend, _ := local.NewBackend(ctx, &local.Config{})
// Command injection risk!

// Good: With validation
backend, _ := local.NewBackend(ctx, &local.Config{
    ValidateCommand: myValidator,
})
```

## FAQ

**Q: Why do all paths need to be absolute?**  
A: This prevents directory traversal attacks. Use `filepath.Abs()` to convert relative paths.

**Q: Can I use this in production?**  
A: Yes, but ensure proper input validation, command validation, and appropriate permissions.


## License

Licensed under the Apache License, Version 2.0. See the [LICENSE](../../LICENSE) file for details.
