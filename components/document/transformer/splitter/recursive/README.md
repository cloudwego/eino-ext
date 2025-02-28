# recursive splitter

Recursive splitter is a splitter that splits the text into chunks recursively. Useful for splitting long text into chunks.

`OverlapSize` in config can set the overlap content length from last chunk, this may help to keep the context of last chunk.

## Usage

example at: [examples/main.go](examples/main.go)
run example: `cd examples && go run main.go`

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
)

func main() {
	ctx := context.Background()

	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   1500,
		OverlapSize: 300,
	})

    docs, err := splitter.Transform(ctx, []*schema.Document{
        {Content: "test content"},
    })
}
```
