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

// This example shows how to connect officialmcp to a remote (streamable-http)
// MCP server that requires OAuth, e.g. Notion, Linear, or GitHub.
//
// officialmcp consumes a *mcp.ClientSession that the caller builds, so OAuth is
// wired at the transport the caller constructs — not inside officialmcp. The
// go-sdk handles the challenge-driven flow itself: it sends requests normally
// and only runs OAuth when the server answers 401/403 + WWW-Authenticate. A
// server that needs no auth never triggers the handler.
//
// auth.AuthorizationCodeHandler is the SDK's ready-made handler implementing
// PRM discovery -> Auth Server Metadata -> (optional DCR) -> auth code + PKCE.
// The only thing the caller must supply is the interactive step: how to present
// the authorization URL to the user and how to read back the redirect.
package main

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	omcp "github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp"
)

func main() {
	ctx := context.Background()

	const serverURL = "https://mcp.notion.com/mcp" // replace with your server
	const redirectURL = "http://127.0.0.1:8085/callback"

	// The handler runs the OAuth flow on a 401/403. Its only injected behavior is
	// the interactive fetcher below — everything else (discovery, DCR, token
	// exchange, PKCE) is the SDK's. officialmcp provides the mechanism (the
	// transport seam); the interaction policy lives here, in the caller.
	oauthHandler, err := auth.NewAuthorizationCodeHandler(&auth.AuthorizationCodeHandlerConfig{
		RedirectURL: redirectURL,
		// Register dynamically; if the server preregisters clients, set
		// PreregisteredClient instead and drop this block.
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				ClientName:   "eino-officialmcp-example",
				RedirectURIs: []string{redirectURL},
			},
		},
		AuthorizationCodeFetcher: fetchAuthorizationCode,
	})
	if err != nil {
		log.Fatalf("build oauth handler: %v", err)
	}

	// OAuth is a first-class field on the streamable-http transport. Wiring it
	// here is the entire integration — officialmcp itself is unchanged.
	// (SSEClientTransport has no OAuth field; use streamable-http for OAuth.)
	transport := &mcp.StreamableClientTransport{
		Endpoint:     serverURL,
		OAuthHandler: oauthHandler,
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "eino-officialmcp", Version: "v1.0.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer session.Close()

	tools, err := omcp.GetTools(ctx, &omcp.Config{Cli: session})
	if err != nil {
		log.Fatalf("get tools: %v", err)
	}
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			log.Fatalf("tool info: %v", err)
		}
		fmt.Printf("- %s: %s\n", info.Name, info.Desc)
	}
}

// fetchAuthorizationCode is the interactive seam. A real Agent runtime would push
// args.URL to its event stream to guide the user, then block (on a ctx with a
// timeout) until the redirect lands and hand back the code + state. Here we just
// print the URL and read the redirect from stdin.
//
// Use a context with a timeout when blocking on user input: the transport's
// RoundTrip holds a lock during the OAuth flow, so an unbounded wait stalls the
// connection.
func fetchAuthorizationCode(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
	fmt.Println("Open this URL in your browser to authorize:")
	fmt.Println(" ", args.URL)
	fmt.Print("After approving, paste the full redirect URL (or just the code): ")

	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		return nil, fmt.Errorf("read authorization code: %w", err)
	}

	code, state := parseRedirect(input)
	return &auth.AuthorizationResult{Code: code, State: state}, nil
}

// parseRedirect accepts either the raw code or the full redirect URL and pulls
// out code/state. Kept trivial for the example.
func parseRedirect(input string) (code, state string) {
	u, err := url.Parse(input)
	if err != nil || u.Query().Get("code") == "" {
		return input, ""
	}
	return u.Query().Get("code"), u.Query().Get("state")
}
