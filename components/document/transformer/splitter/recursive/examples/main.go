package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   1500,
		OverlapSize: 300,
		KeepType:    recursive.KeepTypeNone,
	})
	if err != nil {
		panic(err)
	}

	file := "./eino_readme.md"
	data, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	docs, err := splitter.Transform(ctx, []*schema.Document{
		{
			Content: string(data),
		},
	})

	if err != nil {
		panic(err)
	}

	for idx, doc := range docs {
		fmt.Printf("====== %02d ======\n", idx)
		fmt.Println(doc.Content)
	}

}
