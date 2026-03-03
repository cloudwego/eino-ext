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
	"strings"

	"github.com/cloudwego/eino-ext/callbacks/apmplus"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
)

func main() {
	ctx := context.Background()

	// init apmplus callback, for trace metrics and log
	fmt.Println("INFO: use apmplus as callback, watch at: https://console.volcengine.com/apmplus-server")

	cbh, shutdown, err := apmplus.NewApmplusHandler(&apmplus.Config{
		Host:        "apmplus-cn-beijing.volces.com:4317",
		AppKey:      "appkey-xxx",
		ServiceName: "eino-app",
		Release:     "release/v0.0.1",
	})
	if shutdown != nil {
		defer shutdown(ctx)
	}
	if err != nil {
		log.Fatal(err)
	}

	// Set apmplus as a global callback
	callbacks.AppendGlobalHandlers(cbh)

	// Create your graph instance
	g := compose.NewGraph[string, string]()

	// add node and edage to your eino graph, here is an simple example
	g.AddLambdaNode("node1", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	}), compose.WithNodeName("node1"))
	g.AddLambdaNode("node2", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		sb := strings.Builder{}
		for i := 0; i < 10; i++ {
			sb.WriteString(input)
		}
		return sb.String(), nil
	}), compose.WithNodeName("node2"))
	g.AddEdge(compose.START, "node1")
	g.AddEdge("node1", "node2")
	g.AddEdge("node2", compose.END)

	// Compile graph
	runner, err := g.Compile(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// option: set your session info
	ctx = apmplus.SetSession(ctx, apmplus.WithSessionID("session_abc"), apmplus.WithUserID("user_123"))

	// Invoke runner
	result, err := runner.Invoke(ctx, "input")

	// handler resp
	log.Printf("result: %+v\n err: %v\n\n", result, err)

}
