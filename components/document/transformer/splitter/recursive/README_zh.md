# recursive splitter

Recursive splitter 是一个递归地将文本拆分为块的分割器。对于将长文本拆分为块非常有用。

配置中的 `OverlapSize` 可以设置与上一个块重叠的内容长度，这可能有助于保留上一个块的上下文。

`Separators` 默认为 `["\n", ".", "?", "!", ""]`。末尾的空分隔符是关键：当其他分隔符都无法把片段切到
`ChunkSize` 以内时，它会让分割器退化为按字符切分。若没有它，不含任何分隔符的文本（中文尤其常见，因为默认
分隔符里没有中文标点）会被整段原样输出，可能远超 `ChunkSize`。自定义 `Separators` 时若希望保留这一降级
能力，请同样在末尾加上 `""`。

## 使用方法

示例位于：[examples/main.go](examples/main.go)
运行示例：`cd examples && go run main.go`

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
## 示例

查看以下示例了解更多用法：

- [测试数据示例](./examples/testdata/)

