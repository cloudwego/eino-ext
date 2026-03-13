/*
 * Copyright 2026 CloudWeGo Authors
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
	"io"
	"log"
	"os"

	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/agentic/gemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	// Example 1: Google Search with multi-turn conversation
	fmt.Println("========== Google Search Example ==========")
	runGoogleSearchExample(ctx, client, modelName)

	fmt.Println()

	// Example 2: Code Execution with multi-turn conversation
	fmt.Println("========== Code Execution Example ==========")
	runCodeExecutionExample(ctx, client, modelName)
}

// streamAndConcat is a helper function that streams responses and concatenates them.
func streamAndConcat(stream *schema.StreamReader[*schema.AgenticMessage]) (*schema.AgenticMessage, error) {
	var messages []*schema.AgenticMessage
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("stream receive error: %w", err)
		}
		messages = append(messages, resp)
	}

	m, err := schema.ConcatAgenticMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("concat messages failed: %w", err)
	}
	return m, nil
}

// runGoogleSearchExample demonstrates Gemini's web search (Google Search) capability
// with multi-turn conversation. The model can search the web and provide grounded
// responses with source citations.
func runGoogleSearchExample(ctx context.Context, client *genai.Client, modelName string) {
	cm, err := agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
		Client:             client,
		Model:              modelName,
		EnableGoogleSearch: &genai.GoogleSearch{},
	})
	if err != nil {
		log.Fatalf("NewAgenticModel with GoogleSearch failed, err=%v", err)
	}

	// First turn: Ask about AI news
	fmt.Println("\n--- First Turn ---")
	fmt.Println("User: What are the latest news about AI developments today?")

	stream, err := cm.Stream(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Use GoogleSearch and tell me what are the latest news about AI developments today?"),
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	firstResp, err := streamAndConcat(stream)
	if err != nil {
		log.Fatalf("First turn failed: %v", err)
	}
	fmt.Printf("Assistant:\n%s\n", firstResp.String())

	// Print grounding metadata if available
	if firstResp.ResponseMeta != nil && firstResp.ResponseMeta.GeminiExtension != nil && firstResp.ResponseMeta.GeminiExtension.GroundingMeta != nil {
		fmt.Println("\nGrounding Metadata (Search Sources):")
		for i, chunk := range firstResp.ResponseMeta.GeminiExtension.GroundingMeta.GroundingChunks {
			if chunk.Web != nil {
				fmt.Printf("  [%d] %s: %s\n", i+1, chunk.Web.Title, chunk.Web.URI)
			}
		}
	}

	// Second turn: Follow-up question based on the search results
	fmt.Println("\n--- Second Turn ---")
	fmt.Println("User: Can you give me more details about the most significant one?")

	stream, err = cm.Stream(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Use GoogleSearch and tell me what are the latest news about AI developments today?"),
		firstResp,
		schema.UserAgenticMessage("Can you give me more details about the most significant one? Search for more information if needed."),
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	secondResp, err := streamAndConcat(stream)
	if err != nil {
		log.Fatalf("Second turn failed: %v", err)
	}
	fmt.Printf("Assistant:\n%s\n", secondResp.String())

	// Print grounding metadata for second response
	if secondResp.ResponseMeta != nil && secondResp.ResponseMeta.GeminiExtension != nil && secondResp.ResponseMeta.GeminiExtension.GroundingMeta != nil {
		fmt.Println("\nGrounding Metadata (Search Sources):")
		for i, chunk := range secondResp.ResponseMeta.GeminiExtension.GroundingMeta.GroundingChunks {
			if chunk.Web != nil {
				fmt.Printf("  [%d] %s: %s\n", i+1, chunk.Web.Title, chunk.Web.URI)
			}
		}
	}

	// Third turn: Ask for a summary
	fmt.Println("\n--- Third Turn ---")
	fmt.Println("User: Please summarize all the key points we discussed.")

	stream, err = cm.Stream(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Use GoogleSearch and tell me what are the latest news about AI developments today?"),
		firstResp,
		schema.UserAgenticMessage("Can you give me more details about the most significant one? Search for more information if needed."),
		secondResp,
		schema.UserAgenticMessage("Please summarize all the key points we discussed."),
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	thirdResp, err := streamAndConcat(stream)
	if err != nil {
		log.Fatalf("Third turn failed: %v", err)
	}
	fmt.Printf("Assistant:\n%s\n", thirdResp.String())
}

// runCodeExecutionExample demonstrates Gemini's code execution capability
// with multi-turn conversation. The model can write and execute Python code
// to solve problems and iterate on solutions.
func runCodeExecutionExample(ctx context.Context, client *genai.Client, modelName string) {
	cm, err := agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
		Client:              client,
		Model:               modelName,
		EnableCodeExecution: &genai.ToolCodeExecution{},
	})
	if err != nil {
		log.Fatalf("NewAgenticModel with CodeExecution failed, err=%v", err)
	}

	// First turn: Calculate factorial and Fibonacci
	fmt.Println("\n--- First Turn ---")
	fmt.Println("User: Calculate the factorial of 10 and generate the first 20 Fibonacci numbers.")

	stream, err := cm.Stream(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Calculate the factorial of 10 using Python code, and also generate the first 20 Fibonacci numbers."),
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	firstResp, err := streamAndConcat(stream)
	if err != nil {
		log.Fatalf("First turn failed: %v", err)
	}
	fmt.Printf("Assistant:\n%s\n", firstResp.String())

	// Second turn: Ask to optimize the Fibonacci implementation
	fmt.Println("\n--- Second Turn ---")
	fmt.Println("User: Can you optimize the Fibonacci function using memoization and compare performance?")

	stream, err = cm.Stream(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Calculate the factorial of 10 using Python code, and also generate the first 20 Fibonacci numbers."),
		firstResp,
		schema.UserAgenticMessage("Can you optimize the Fibonacci function using memoization and compare the performance with the previous implementation? Generate Fibonacci for n=30 and time both approaches."),
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	secondResp, err := streamAndConcat(stream)
	if err != nil {
		log.Fatalf("Second turn failed: %v", err)
	}
	fmt.Printf("Assistant:\n%s\n", secondResp.String())

	// Third turn: Ask for a visualization
	fmt.Println("\n--- Third Turn ---")
	fmt.Println("User: Plot the Fibonacci sequence growth using matplotlib.")

	stream, err = cm.Stream(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Calculate the factorial of 10 using Python code, and also generate the first 20 Fibonacci numbers."),
		firstResp,
		schema.UserAgenticMessage("Can you optimize the Fibonacci function using memoization and compare the performance with the previous implementation? Generate Fibonacci for n=30 and time both approaches."),
		secondResp,
		schema.UserAgenticMessage("Now create a simple visualization showing the exponential growth of Fibonacci numbers. Use matplotlib to plot the first 20 Fibonacci numbers."),
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	thirdResp, err := streamAndConcat(stream)
	if err != nil {
		log.Fatalf("Third turn failed: %v", err)
	}
	fmt.Printf("Assistant:\n%s\n", thirdResp.String())
}
