# recursive splitter

Recursive splitter is a splitter that splits the text into chunks recursively. Useful for splitting long text into chunks.

`OverlapSize` in config can set the overlap content length from last chunk, this may help to keep the context of last chunk.

`Separators` defaults to `["\n", ".", "?", "!", ""]`. The trailing empty separator is what lets the
splitter fall back to per-character splitting once none of the other separators can bring a piece
under `ChunkSize` — without it, text containing no separator at all (often the case for Chinese,
whose punctuation is absent from the default set) is emitted as a single chunk however far over
`ChunkSize` it is. Keep a trailing `""` if you override `Separators` and want that fallback.

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
## Examples

See the following examples for more usage:

- [Test Data Example](./examples/testdata/)

