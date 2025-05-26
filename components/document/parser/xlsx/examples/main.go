/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	callbacksHelper "github.com/cloudwego/eino/utils/callbacks"

	"github.com/cloudwego/eino-ext/components/document/loader/file"
	"github.com/cloudwego/eino-ext/components/document/parser/xlsx"
)

func main() {
	ctx := context.Background()

	log.Printf("===== call XLSX Parser directly =====")
	// Initialize the parser
	parser, err := xlsx.NewXlsxParser(ctx, nil)
	if err != nil {
		log.Fatalf("xlsx.NewXLSXParser failed, err=%v", err)
	}

	// Initialize the loader
	loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{
		Parser: parser,
	})
	if err != nil {
		log.Fatalf("file.NewFileLoader failed, err=%v", err)
	}

	// Load the document
	filePath := "./testdata/test.xlsx"
	docs, err := loader.Load(ctx, document.Source{
		URI: filePath,
	})
	if err != nil {
		log.Fatalf("loader.Load failed, err=%v", err)
	}

	log.Printf("===== Documents Content =====")
	for _, doc := range docs {
		log.Printf("Id %v content: %v metadata: %v", doc.ID, doc.Content, doc.MetaData)
	}

	log.Printf("===== call XLSX Parser in Chain =====")
	// Create callback handler
	handlerHelper := &callbacksHelper.LoaderCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *document.LoaderCallbackInput) context.Context {
			log.Printf("start loading docs...: %s\n", input.Source.URI)
			return ctx
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *document.LoaderCallbackOutput) context.Context {
			log.Printf("complete loading docs，total loaded docs: %d\n", len(output.Docs))
			return ctx
		},
		// OnError
	}

	// Use callback handler
	handler := callbacksHelper.NewHandlerHelper().
		Loader(handlerHelper).
		Handler()

	chain := compose.NewChain[document.Source, []*schema.Document]()
	chain.AppendLoader(loader)
	// Use at runtime
	run, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("chain.Compile failed, err=%v", err)
	}

	outDocs, err := run.Invoke(ctx, document.Source{
		URI: filePath,
	}, compose.WithCallbacks(handler))
	if err != nil {
		log.Fatalf("run.Invoke failed, err=%v", err)
	}

	log.Printf("===== Documents Content =====")
	for _, doc := range outDocs {
		log.Printf("Id %v content: %v metadata: %v", doc.ID, doc.Content, doc.MetaData)
	}
}
