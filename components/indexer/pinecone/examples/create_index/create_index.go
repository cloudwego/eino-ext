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

	"github.com/pinecone-io/go-pinecone/pinecone"
)

// component does not support create index, you can create index on platform or by codes below

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("PINECONE_API_KEY")
	indexName := "eino-index-test"

	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: apiKey,
	})
	if err != nil {
		panic(fmt.Errorf("failed to create client: %v", err))
	}

	// [dimensionality]: https://docs.pinecone.io/guides/indexes/choose-a-pod-type-and-size#dimensionality-of-vectors
	// [Serverless]: https://docs.pinecone.io/guides/indexes/understanding-indexes#serverless-indexes
	// [similarity]: https://docs.pinecone.io/guides/indexes/understanding-indexes#distance-metrics
	// [region]: https://docs.pinecone.io/troubleshooting/available-cloud-regions
	// [cloud provider]: https://docs.pinecone.io/troubleshooting/available-cloud-regions#regions-available-for-serverless-indexes
	// [deletion protection]: https://docs.pinecone.io/guides/indexes/prevent-index-deletion#enable-deletion-protection
	idx, err := pc.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
		Name:      indexName,
		Dimension: 1024,
		Metric:    pinecone.Cosine, // use pinecone.Dotproduct if you need hybrid search
		Cloud:     pinecone.Aws,
		Region:    "us-east-1",
	})
	if err != nil {
		panic(fmt.Errorf("failed to create serverless index \"%v\": %v", indexName, err))
	}

	fmt.Printf("Successfully created serverless index: %v", idx.Name)
	// Successfully created serverless index: eino-index-test
}
