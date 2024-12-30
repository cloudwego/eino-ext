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
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

func main() {
	accessKey := os.Getenv("OPENAI_API_KEY")

	ctx := context.Background()

	var (
		defaultDim = 1024
	)

	embedding, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
		APIKey:     accessKey,
		Model:      "text-embedding-3-large",
		Dimensions: &defaultDim,
		Timeout:    0,
	})
	if err != nil {
		panic(fmt.Errorf("new embedder error: %v\n", err))
	}

	resp, err := embedding.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		panic(fmt.Errorf("generate failed, err=%v", err))
	}

	fmt.Printf("output=%v", resp)
}
