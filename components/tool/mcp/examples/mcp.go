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
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	omcp "github.com/modelcontextprotocol/go-sdk/mcp"

	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
)

func main() {
	isOfficial := flag.Bool("o", true, "true for official mcp sdk, false for non-official mcp sdk, default is true")
	flag.Parse()
	if isOfficial == nil || *isOfficial {
		runByOfficialMCPSDK()
	} else {
		runByNonOfficialMCPSDK()
	}
}

func runByNonOfficialMCPSDK() {
	startMCPServer()
	time.Sleep(1 * time.Second)
	ctx := context.Background()

	mcpTools := getMCPTool(ctx)

	for i, mcpTool := range mcpTools {
		fmt.Println(i, ":")
		info, err := mcpTool.Info(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Name:", info.Name)
		fmt.Println("Desc:", info.Desc)
		result, err := mcpTool.(tool.InvokableTool).InvokableRun(ctx, `{"operation":"add", "x":1, "y":1}`)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Result:", result)
		fmt.Println()
	}
}

func runByOfficialMCPSDK() {
	httpServer := startOfficialMCPServer()
	time.Sleep(1 * time.Second)
	ctx := context.Background()

	sess := getMCPClientSession(ctx, httpServer.URL)
	defer sess.Close()

	mcpTools, err := mcpp.GetTools(ctx, &mcpp.Config{OfficialCli: sess})
	if err != nil {
		log.Fatal(err)
	}

	for i, mcpTool := range mcpTools {
		fmt.Println(i, ":")
		info, err := mcpTool.Info(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Name:", info.Name)
		fmt.Println("Desc:", info.Desc)
		result, err := mcpTool.(tool.InvokableTool).InvokableRun(ctx, `{"x":1, "y":1}`)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Result:", result)
		fmt.Println()
	}
}

func getMCPTool(ctx context.Context) []tool.BaseTool {
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

	tools, err := mcpp.GetTools(ctx, &mcpp.Config{Cli: cli})
	if err != nil {
		log.Fatal(err)
	}

	return tools
}

func startMCPServer() {
	svr := server.NewMCPServer("demo", mcp.LATEST_PROTOCOL_VERSION)
	svr.AddTool(mcp.NewTool("calculate",
		mcp.WithDescription("Perform basic arithmetic operations"),
		mcp.WithString("operation",
			mcp.Required(),
			mcp.Description("The operation to perform (add, subtract, multiply, divide)"),
			mcp.Enum("add", "subtract", "multiply", "divide"),
		),
		mcp.WithNumber("x",
			mcp.Required(),
			mcp.Description("First number"),
		),
		mcp.WithNumber("y",
			mcp.Required(),
			mcp.Description("Second number"),
		),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arg := request.Params.Arguments.(map[string]any)
		op := arg["operation"].(string)
		x := arg["x"].(float64)
		y := arg["y"].(float64)

		var result float64
		switch op {
		case "add":
			result = x + y
		case "subtract":
			result = x - y
		case "multiply":
			result = x * y
		case "divide":
			if y == 0 {
				return mcp.NewToolResultText("Cannot divide by zero"), nil
			}
			result = x / y
		}
		log.Printf("Calculated result: %.2f", result)
		return mcp.NewToolResultText(fmt.Sprintf("%.2f", result)), nil
	})
	go func() {
		defer func() {
			e := recover()
			if e != nil {
				fmt.Println(e)
			}
		}()

		err := server.NewSSEServer(svr, server.WithBaseURL("http://localhost:12345")).Start("localhost:12345")

		if err != nil {
			log.Fatal(err)
		}
	}()
}

type AddParams struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func Add(ctx context.Context, req *omcp.CallToolRequest, args AddParams) (*omcp.CallToolResult, any, error) {
	return &omcp.CallToolResult{
		Content: []omcp.Content{
			&omcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)},
		},
	}, nil, nil
}

func getMCPClientSession(ctx context.Context, addr string) *omcp.ClientSession {
	transport := &omcp.SSEClientTransport{Endpoint: addr}
	client := omcp.NewClient(&omcp.Implementation{Name: "test", Version: "v1.0.0"}, nil)
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatal(err)
	}
	return sess
}

func startOfficialMCPServer() *httptest.Server {
	server := omcp.NewServer(&omcp.Implementation{Name: "adder", Version: "v0.0.1"}, nil)
	omcp.AddTool(server, &omcp.Tool{Name: "add", Description: "add two numbers"}, Add)

	handler := omcp.NewSSEHandler(func(*http.Request) *omcp.Server { return server }, nil)

	httpServer := httptest.NewServer(handler)
	return httpServer
}
