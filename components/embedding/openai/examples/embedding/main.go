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
	"os"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/compose"
	callbacksHelper "github.com/cloudwego/eino/utils/callbacks"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

func main() {
	accessKey := os.Getenv("OPENAI_API_KEY")

	ctx := context.Background()

	var (
		defaultDim = 1024
	)

	embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
		APIKey:     accessKey,
		Model:      "text-embedding-3-large",
		Dimensions: &defaultDim,
		Timeout:    0,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of openai failed, err=%v", err)
	}

	log.Printf("===== call Embedder directly =====")

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of openai failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)

	log.Printf("===== call Embedder in Chain =====")

	handlerHelper := &callbacksHelper.EmbeddingCallbackHandler{
		OnStart: func(ctx context.Context, runInfo *callbacks.RunInfo, input *embedding.CallbackInput) context.Context {
			log.Printf("input access, len: %v, content: %s\n", len(input.Texts), input.Texts)
			return ctx
		},
		OnEnd: func(ctx context.Context, runInfo *callbacks.RunInfo, output *embedding.CallbackOutput) context.Context {
			log.Printf("output finished, len: %v\n", len(output.Embeddings))
			return ctx
		},
	}

	handler := callbacksHelper.NewHandlerHelper().
		Embedding(handlerHelper).
		Handler()

	chain := compose.NewChain[[]string, [][]float64]()
	chain.AppendEmbedding(embedder)

	// 编译并运行
	runnable, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("chain Compile failed, err=%v", err)
	}

	vectors, err = runnable.Invoke(ctx, []string{"hello", "how are you"},
		compose.WithCallbacks(handler))
	if err != nil {
		log.Fatalf("Invoke of runnable failed, err=%v", err)
	}

	log.Printf("vectors in chain: %v", vectors)
}
