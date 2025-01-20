package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	callbacksHelper "github.com/cloudwego/eino/utils/callbacks"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
)

func main() {
	ctx := context.Background()

	// 初始化 transformer (以 markdown 为例)
	transformer, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		// 配置参数
		Headers: map[string]string{
			"##": "",
		},
	})
	if err != nil {
		log.Fatalf("markdown.NewHeaderSplitter failed, err=%v", err)
	}

	markdownDoc := &schema.Document{
		Content: "## Title 1\nHello Word\n## Title 2\nWord Hello",
	}

	log.Printf("===== call Header Splitter directly =====")

	// 转换文档
	transformedDocs, err := transformer.Transform(ctx, []*schema.Document{markdownDoc})
	if err != nil {
		log.Fatalf("transformer.Transform failed, err=%v", err)
	}

	for idx, doc := range transformedDocs {
		log.Printf("doc segment %v: %v", idx, doc.Content)
	}

	log.Printf("===== call Header Splitter in chain =====")

	// 创建 callback handler
	handler := &callbacksHelper.TransformerCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *document.TransformerCallbackInput) context.Context {
			log.Printf("input access, len: %v, content: %s\n", len(input.Input), input.Input[0].Content)
			return ctx
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *document.TransformerCallbackOutput) context.Context {
			log.Printf("output finished, len: %v\n", len(output.Output))
			return ctx
		},
		// OnError
	}

	// 使用 callback handler
	helper := callbacksHelper.NewHandlerHelper().
		Transformer(handler).
		Handler()

	chain := compose.NewChain[[]*schema.Document, []*schema.Document]()
	chain.AppendDocumentTransformer(transformer)

	// 在运行时使用
	run, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("chain.Compile failed, err=%v", err)
	}

	outDocs, err := run.Invoke(ctx, []*schema.Document{markdownDoc}, compose.WithCallbacks(helper))
	if err != nil {
		log.Fatalf("run.Invoke failed, err=%v", err)
	}

	for idx, doc := range outDocs {
		log.Printf("doc segment %v: %v", idx, doc.Content)
	}
}
