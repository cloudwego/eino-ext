package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/callbacks/langsmith"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
)

func main() {
	// Create a langsmith handler
	// In a real application, you would get the API key from environment variables or a config file.
	cfg := &langsmith.Config{
		APIKey: "your api key",
		APIURL: "your api url",
		RunIDGen: func(ctx context.Context) string { // optional. run id generator. default is uuid.NewString
			return uuid.NewString()
		},
	}
	// ft := langsmith.NewFlowTrace(cfg)
	cbh, err := langsmith.NewLangsmithHandler(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Set langsmith as a global callback handler
	callbacks.AppendGlobalHandlers(cbh)
	// Set trace-level information using the context
	ctx := context.Background()
	ctx = langsmith.SetTrace(ctx,
		langsmith.WithSessionName("your session name"),
	)

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
	result, err := runner.Invoke(ctx, "some input\n")
	if err != nil {
		fmt.Println(err)
	}
	// Process the result
	log.Printf("Got result: %s", result)
}
