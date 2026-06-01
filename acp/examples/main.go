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
	"os"

	hertzserver "github.com/cloudwego/hertz/pkg/app/server"

	acp "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
	acpserver "github.com/eino-contrib/acp/server"
	"github.com/eino-contrib/acp/transport/stdio"
)

func main() {
	// Required environment variables:
	//   ARK_API_KEY  - your ARK API key
	//   ARK_MODEL    - model to use
	//   ARK_BASE_URL - optional, custom base URL for ARK APIs
	if os.Getenv("ARK_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "ARK_API_KEY environment variable is required")
		os.Exit(1)
	}

	transportMode := flag.String("transport", "stdio", "transport mode: stdio or http")
	listenAddr := flag.String("listen", ":8080", "listen address when -transport=http")
	flag.Parse()
	a := newAgent()
	ctx := context.Background()

	switch *transportMode {
	case "stdio":
		if err := runStdioTransport(ctx, a); err != nil {
			fmt.Fprintf(os.Stderr, "agent error: %v\n", err)
			os.Exit(1)
		}
	case "http":
		runHTTPTransport(*listenAddr)
	default:
		fmt.Fprintf(os.Stderr, "unsupported transport: %s\n", *transportMode)
		os.Exit(1)
	}
}

func runStdioTransport(ctx context.Context, agent acp.Agent) error {
	transport := stdio.NewTransport(os.Stdin, os.Stdout)
	conn := acpconn.NewAgentConnectionFromTransport(agent, transport)
	if aware, ok := agent.(acpserver.ConnectionAwareAgent); ok {
		aware.SetClientConnection(conn)
	}
	if err := conn.Start(ctx); err != nil {
		return err
	}
	<-conn.Done()
	return nil
}

func runHTTPTransport(listenAddr string) {
	srv := hertzserver.New(hertzserver.WithHostPorts(listenAddr))
	srv.NoHijackConnPool = true
	remote, err := acpserver.NewACPServer(func(_ context.Context) acp.Agent { return newAgent() })
	if err != nil {
		fmt.Fprintf(os.Stderr, "create ACP server: %v\n", err)
		os.Exit(1)
	}
	remote.Mount(srv)
	fmt.Fprintf(os.Stderr, "Listening on %s\n", listenAddr)

	srv.Spin()
}
