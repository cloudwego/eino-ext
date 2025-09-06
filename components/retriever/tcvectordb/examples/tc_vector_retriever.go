/*
 * Copyright 2025 CloudWeGo Authors
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

package examples

import (
	"context"
	"fmt"
	"os"
	"time"

	oaiembedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	callbacksHelper "github.com/cloudwego/eino/utils/callbacks"

	"github.com/cloudwego/eino-ext/components/retriever/tcvectordb"
)

const (
	EmbeddingModel3Large    = "text-embedding-3-large"
	EmbeddingModelDimension = 3072
)

func main() {
	ctx := context.Background()
	userID := os.Getenv("TC_VECTOR_USER_ID")
	secretKey := os.Getenv("TC_VECTOR_SECRET_KEY")

	// using Openai embedding model
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	openaiBaseURL := os.Getenv("OPENAI_BASE_URL")

	// fake collection name and index name
	collectionName := "eino_examples"
	indexName := "example_index"

	/*
	 * The following example assumes that a collection named eino_examples has been created with the appropriate index
	 * The collection fields are configured as:
	 * Field Name        Field Type      Vector Dimension
	 * id                string
	 * vector            vector          3072
	 * content           string
	 * metadata          json
	 *
	 * Note:
	 * 1. The vector dimension must match the output dimension of the Embedder
	 * 2. Ensure the collection is created and properly configured
	 */

	// Create embedder
	embedder, err := oaiembedding.NewEmbedder(ctx, &oaiembedding.EmbeddingConfig{
		APIKey:     openaiAPIKey,
		BaseURL:    openaiBaseURL,
		Model:      EmbeddingModel3Large,
		Dimensions: &[]int{EmbeddingModelDimension}[0],
	})
	if err != nil {
		fmt.Printf("Failed to create embedder: %v\n", err)
		return
	}

	// Create tcvectordb retriever config
	cfg := &tcvectordb.RetrieverConfig{
		URL:            "http://127.0.0.1:8080", // URL provided by Tencent Cloud
		Username:       userID,                  // Username provided by Tencent Cloud
		Key:            secretKey,               // Key provided by Tencent Cloud
		Database:       "eino_db",               // example db
		Collection:     collectionName,          // The collection where data is stored
		Timeout:        time.Second * 5,
		TopK:           10,
		ScoreThreshold: of(0.75),
		Index:          indexName,
		EmbeddingConfig: tcvectordb.EmbeddingConfig{
			UseBuiltin: false,
			Embedding:  embedder,
		},
	}

	// Create retriever instance
	tcRetriever, err := tcvectordb.NewRetriever(ctx, cfg)
	if err != nil {
		fmt.Printf("Failed to create TcVectorDB retriever: %v\n", err)
		return
	}

	fmt.Println("===== Direct Retriever Call =====")

	query := "Features of Tencent Cloud Vector Database"
	docs, err := tcRetriever.Retrieve(ctx, query)
	if err != nil {
		fmt.Printf("Retrieval failed: %v\n", err)
		return
	}

	fmt.Printf("Retrieval successful, query=%v, document count=%v\n", query, len(docs))
	for i, doc := range docs {
		fmt.Printf("Document %d: Content=%s, Score=%f\n", i+1, doc.Content, doc.Score())
	}

	fmt.Println("===== Using Retriever in a Chain =====")

	// Create callback handler
	handlerHelper := &callbacksHelper.RetrieverCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *retriever.CallbackInput) context.Context {
			fmt.Printf("Starting retrieval, query content: %s\n", input.Query)
			return ctx
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *retriever.CallbackOutput) context.Context {
			fmt.Printf("Retrieval completed, result count: %v\n", len(output.Docs))
			return ctx
		},
		// OnError is optional
	}

	// Use callback handler
	handler := callbacksHelper.NewHandlerHelper().
		Retriever(handlerHelper).
		Handler()

	// Create chain
	chain := compose.NewChain[string, []*schema.Document]()
	chain.AppendRetriever(tcRetriever)

	// Run chain
	run, err := chain.Compile(ctx)
	if err != nil {
		fmt.Printf("Chain compilation failed: %v\n", err)
		return
	}

	outDocs, err := run.Invoke(ctx, query, compose.WithCallbacks(handler))
	if err != nil {
		fmt.Printf("Chain execution failed: %v\n", err)
		return
	}

	fmt.Printf("Chain retrieval successful, query=%v, document count=%v\n", query, len(outDocs))
	for i, doc := range outDocs {
		fmt.Printf("Document %d: Content=%s, Score=%f\n", i+1, doc.Content, doc.Score())
	}

}

func of[T any](v T) *T {
	return &v
}
