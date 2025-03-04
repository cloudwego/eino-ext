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
	"time"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	mcpp "github.com/cloudwego/eino-ext/components/prompt/mcp"
)

func main() {
	startMCPServer()
	time.Sleep(1 * time.Second)
	ctx := context.Background()

	mcpPrompt := getMCPPrompt(ctx)

	result, err := mcpPrompt.Format(ctx, map[string]interface{}{"persona": "Describe the content of the image"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
}

func getMCPPrompt(ctx context.Context) prompt.ChatTemplate {
	cli, err := client.NewSSEMCPClient("http://localhost:12345/sse")
	if err != nil {
		log.Fatal(err)
	}
	err = cli.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}

	_, err = cli.Initialize(ctx, initRequest)
	if err != nil {
		log.Fatal(err)
	}

	p, err := mcpp.NewPromptTemplate(ctx, &mcpp.Config{Cli: cli, Name: "test"})
	if err != nil {
		log.Fatal(err)
	}

	return p
}

func startMCPServer() {
	svr := server.NewMCPServer("demo", mcp.LATEST_PROTOCOL_VERSION, server.WithPromptCapabilities(false))
	svr.AddPrompt(mcp.Prompt{
		Name: "test",
	}, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(request.Params.Arguments["persona"])),
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewImageContent("https://upload.wikimedia.org/wikipedia/commons/3/3a/Cat03.jpg", "image/jpeg")),
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewEmbeddedResource(mcp.TextResourceContents{
					URI:      "https://upload.wikimedia.org/wikipedia/commons/3/3a/Cat03.jpg",
					MIMEType: "image/jpeg",
					Text:     "resource",
				})),
			},
		}, nil
	})
	go func() {
		defer func() {
			e := recover()
			if e != nil {
				fmt.Println(e)
			}
		}()

		err := server.NewSSEServer(svr, "http://localhost:12345").Start("localhost:12345")

		if err != nil {
			log.Fatal(err)
		}
	}()
}
