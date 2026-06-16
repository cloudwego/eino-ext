# Official MCP Tool

A MCP Tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Tool` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation. Implementation based on Official MCP SDK.

## Features

- Implements `github.com/cloudwego/eino/components/tool.BaseTool`
- Easy integration with Eino's tool system
- Support for get&call mcp tools

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp@latest
```

## Quick Start

Here's a quick example of how to use the official mcp tool:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	omcp "github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp"
)

type AddParams struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func Add(ctx context.Context, req *mcp.CallToolRequest, args AddParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)},
		},
	}, nil, nil
}

func main() {
	httpServer := startMCPServer()
	time.Sleep(1 * time.Second)
	ctx := context.Background()

	cli := getMCPClient(ctx, httpServer.URL)
	defer cli.Close()

	mcpTools, err := omcp.GetTools(ctx, &omcp.Config{Cli: cli})
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

func getMCPClient(ctx context.Context, addr string) *mcp.ClientSession {
	transport := &mcp.SSEClientTransport{Endpoint: addr}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1.0.0"}, nil)
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatal(err)
	}
	return sess
}

func startMCPServer() *httptest.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "adder", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, Add)

	handler := mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return server }, nil)

	httpServer := httptest.NewServer(handler)
	return httpServer
}
```

## Configuration

The tool can be configured using the `mcp.Config` struct:

```go
type Config struct {
	// Cli is the MCP (Model Control Protocol) client, ref: https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools
	// Notice: should Initialize with server before use
	Cli *mcp.ClientSession
	// ToolNameList specifies which tools to fetch from MCP server
	// If empty, all available tools will be fetched
	ToolNameList []string
}
```

## Examples

See the [examples](./examples/) directory for complete usage examples.

## Dynamic credential injection (streamable-http / SSE)

If authorization is handled out of band — e.g. a credential vault that pre-authorizes
each server and hands back a ready auth header at request time — you don't need
interactive OAuth, just a way to inject the header on every connection request.
Unlike a static header map fixed at config time, a per-request provider lets the
credential be resolved late (less secret dwell time) and refreshed across requests
(e.g. an expiring token).

officialmcp consumes a `*mcp.ClientSession` you build, so this is wired at the
`http.Client` you hand to the transport. The seam is a small `http.RoundTripper`
that asks a provider for headers before each request:

```go
type CredentialProvider interface {
    Credentials(ctx context.Context) (headers map[string]string, err error)
}

type credentialRoundTripper struct {
    base     http.RoundTripper
    headers  map[string]string  // static base
    provider CredentialProvider // dynamic, overrides static; may be nil
}

func (rt credentialRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    clone := req.Clone(req.Context()) // never mutate the incoming request
    for k, v := range rt.headers {
        clone.Header.Set(k, v)
    }
    if rt.provider != nil {
        dyn, err := rt.provider.Credentials(req.Context()) // inherit request deadline
        if err != nil {
            return nil, err
        }
        for k, v := range dyn {
            clone.Header.Set(k, v)
        }
    }
    base := rt.base
    if base == nil {
        base = http.DefaultTransport
    }
    return base.RoundTrip(clone)
}

client := &http.Client{Transport: credentialRoundTripper{base: http.DefaultTransport, provider: p}}
transport := &mcp.StreamableClientTransport{Endpoint: serverURL, HTTPClient: client}
session, _ := mcp.NewClient(impl, nil).Connect(ctx, transport, nil)
tools, _ := omcp.GetTools(ctx, &omcp.Config{Cli: session})
```

A nil provider yields plain static-header behavior, so this composes cleanly with
existing setups. It needs no 401 handling, no `oauthex`, no build tag, and works on
any go-sdk version exposing `Transport.HTTPClient`. Use `SSEClientTransport{Endpoint,
HTTPClient}` for an SSE server. See [examples/credentialprovider](./examples/credentialprovider/)
for a complete program.

## OAuth for remote (streamable-http) servers

> This is the heavier, *interactive* path (authorization-code flow, 401 challenge).
> If your credentials are already resolved out of band, prefer the
> CredentialProvider seam above — the two are orthogonal.

Many hosted MCP servers (Notion, Linear, GitHub, ...) require OAuth rather than a
static token. officialmcp consumes a `*mcp.ClientSession` that you build, so OAuth
is wired at the transport you construct — not inside officialmcp.

The go-sdk drives the flow itself: it sends requests normally and only runs OAuth
when the server answers `401`/`403` with `WWW-Authenticate` (challenge-driven, per
the MCP Authorization spec). A server that needs no auth never triggers it, so there
is nothing to detect up front and nothing changes for existing servers.

Wire it by setting `OAuthHandler` on `mcp.StreamableClientTransport`:

```go
oauthHandler, err := auth.NewAuthorizationCodeHandler(&auth.AuthorizationCodeHandlerConfig{
    RedirectURL: redirectURL,
    DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
        Metadata: &oauthex.ClientRegistrationMetadata{
            ClientName:   "my-app",
            RedirectURIs: []string{redirectURL},
        },
    },
    // The only interaction you must supply: present args.URL to the user and
    // return the code/state from the redirect. Use a ctx with a timeout — the
    // transport holds a lock during the flow.
    AuthorizationCodeFetcher: fetchAuthorizationCode,
})
// ...
transport := &mcp.StreamableClientTransport{Endpoint: serverURL, OAuthHandler: oauthHandler}
session, _ := mcp.NewClient(impl, nil).Connect(ctx, transport, nil)
tools, _ := omcp.GetTools(ctx, &omcp.Config{Cli: session})
```

`auth.AuthorizationCodeHandler` is the SDK's ready-made handler implementing PRM
discovery → Auth Server Metadata → optional Dynamic Client Registration → auth code
+ PKCE. You only provide the interactive step. For lower-level control, the
`oauthex` package exposes the discovery/registration primitives directly. See
[examples/oauth](./examples/oauth/) for a complete program.

> Requires go-sdk v1.6.x+, which makes `OAuthHandler` a first-class transport field
> and no longer needs a build tag. `SSEClientTransport` has no OAuth field — use
> streamable-http for OAuth.

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [MCP Documentation](https://modelcontextprotocol.io/introduction)
- [Official MCP SDK Documentation](https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools)
