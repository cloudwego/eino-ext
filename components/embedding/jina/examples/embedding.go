package main

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/embedding/jina"
)

func main() {
	ctx := context.Background()
	embedder, err := jina.NewEmbedder(ctx, &jina.JinaConfig{
		APIKey: "jina_5bbcd9***",
	})
	if err != nil {
		fmt.Println("Jina init error:", err)
		return
	}
	embedding, err := embedder.EmbedStrings(ctx, []string{"hello world"})
	if err != nil {
		fmt.Println("Jina embed error:", err)
		return
	}
	fmt.Println("Jina embedding:", embedding)
	fmt.Println("Jina embedding length:", len(embedding[0]))
}
