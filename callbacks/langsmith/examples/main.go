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

package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/cloudwego/eino-ext/callbacks/langsmith"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
)

func main() {
	// Create a langsmith handler
	// In a real application, you would get the API key from environment variables or a config file.
	cfg := &langsmith.Config{
		APIKey: "xxx",
		APIURL: "xxx",
		RunIDGen: func(ctx context.Context) string { // optional. run id generator. default is uuid.NewString
			return uuid.NewString()
		},
	}
	ft := langsmith.NewFlowTrace(cfg)
	cbh, err := langsmith.NewLangsmithHandler(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Set langsmith as a global callback handler
	callbacks.AppendGlobalHandlers(cbh)
	// Set trace-level information using the context
	ctx := context.Background()
	var tmpMetadata sync.Map
	tmpMetadata.Store("metadata", map[string]interface{}{
		"cid": "cid_test",
		"env": "env_test",
	})
	ctx = langsmith.SetTrace(ctx,
		langsmith.WithSessionName(""),
		langsmith.AddTag("cid_test"),
		langsmith.AddTag("env_test"),
		langsmith.SetMetadata(&tmpMetadata),
	)

	ctx, spanID, err := ft.StartSpan(ctx, "test", nil)
	defer func() {
		ft.FinishSpan(ctx, spanID)
	}()

	// Build and compile an eino graph
	g := compose.NewGraph[string, string]()
	// ... add nodes and edges to your graph
	// add node and edage to your eino graph, here is an simple example
	g.AddLambdaNode("node1", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	}), compose.WithNodeName("node1"))
	g.AddLambdaNode("node2", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		return "test output", nil
	}), compose.WithNodeName("node2"))
	g.AddEdge(compose.START, "node1")
	g.AddEdge("node1", "node2")
	g.AddEdge("node2", compose.END)

	runner, err := g.Compile(ctx)
	if err != nil {
		fmt.Println(err)
	}
	// Invoke the runner
	ctx = langsmith.SetTrace(ctx,
		langsmith.WithSessionName("test"),
		langsmith.AddTag("env_test"),
		langsmith.SetMetadata(&tmpMetadata),
	)
	result, err := runner.Invoke(ctx, "some input\n")
	if err != nil {
		fmt.Println(err)
	}
	// Process the result
	log.Printf("Got result: %s", result)
}
