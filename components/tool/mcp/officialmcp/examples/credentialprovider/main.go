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

// This example shows how to inject *dynamic* auth headers into a remote
// (streamable-http / SSE) MCP connection, resolved per request rather than fixed
// at config time.
//
// The use case is an out-of-band credential vault: authorization happens
// elsewhere, and at request time the caller resolves the ready auth header for a
// given server URL. Unlike a static header map, a provider lets the credential be
// resolved late (less secret dwell time) and refreshed across requests (e.g. an
// expiring token).
//
// officialmcp consumes a *mcp.ClientSession the caller builds, so this is wired at
// the http.Client the caller hands to the transport — not inside officialmcp. The
// seam is just an http.RoundTripper: it asks the provider for headers before each
// request and sets them on a clone. This needs no 401 handling, no oauthex, no
// build tag, and works on any go-sdk version that exposes Transport.HTTPClient.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	omcp "github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp"
)

// CredentialProvider is called before each outbound HTTP request and returns the
// auth headers to inject. An empty map means "inject nothing" (bare request); an
// error fails the request. officialmcp is unaware of the credential shape —
// token, API key, and any refresh logic live inside the implementation.
type CredentialProvider interface {
	Credentials(ctx context.Context) (headers map[string]string, err error)
}

// credentialRoundTripper injects provider-supplied headers per request. Static
// headers form the base; provider headers override same-named entries. With a nil
// provider it just applies the static headers — matching plain HTTPClient behavior.
type credentialRoundTripper struct {
	base     http.RoundTripper
	headers  map[string]string   // static, applied first
	provider CredentialProvider  // dynamic, overrides static; may be nil
}

func (rt credentialRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Never mutate the incoming request — clone before setting headers.
	clone := req.Clone(req.Context())

	for k, v := range rt.headers {
		clone.Header.Set(k, v)
	}
	if rt.provider != nil {
		// Use the request's context so vault lookups inherit its deadline —
		// avoid unbounded blocking on the connection.
		dyn, err := rt.provider.Credentials(req.Context())
		if err != nil {
			return nil, fmt.Errorf("resolve credentials: %w", err)
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

func httpClientWithCredentials(staticHeaders map[string]string, p CredentialProvider) *http.Client {
	return &http.Client{Transport: credentialRoundTripper{
		base:     http.DefaultTransport,
		headers:  staticHeaders,
		provider: p,
	}}
}

// vaultProvider is a stand-in for an out-of-band credential vault. A real one
// would resolve (and refresh) the auth header for the target server URL here.
type vaultProvider struct {
	serverURL string
}

func (v vaultProvider) Credentials(ctx context.Context) (map[string]string, error) {
	// e.g. token, err := vault.Resolve(ctx, v.serverURL); ...
	return map[string]string{"Authorization": "Bearer " + "<token-from-vault>"}, nil
}

func main() {
	ctx := context.Background()

	const serverURL = "https://example.com/mcp" // replace with your server

	httpClient := httpClientWithCredentials(
		map[string]string{"X-App": "eino-officialmcp"}, // static base headers
		vaultProvider{serverURL: serverURL},            // dynamic, per-request
	)

	// streamable-http (shown) and SSE both expose HTTPClient; the same client works
	// for either. Use SSEClientTransport{Endpoint, HTTPClient} for an SSE server.
	transport := &mcp.StreamableClientTransport{
		Endpoint:   serverURL,
		HTTPClient: httpClient,
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
