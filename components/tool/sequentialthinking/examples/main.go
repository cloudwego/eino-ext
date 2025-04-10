package examples

import (
	"context"
	"fmt"
	
	"github.com/cloudwego/eino-ext/components/tool/sequentialthinking"
)

func main() {
	ctx := context.Background()
	
	// Instantiate the tool
	tool, err := sequentialthinking.NewTool()
	if err != nil {
		panic(err)
	}
	
	// Use the tool
	// (This is just a placeholder; actual usage will depend on the tool's functionality)
	result, err := tool.InvokableRun(ctx, "example input")
	if err != nil {
		panic(err)
	}
	
	// Process the result
	// (This is just a placeholder; actual processing will depend on the tool's output)
	fmt.Println(result)
}
