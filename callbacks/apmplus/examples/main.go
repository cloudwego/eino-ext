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

	"github.com/cloudwego/eino-ext/callbacks/apmplus"
	"github.com/cloudwego/eino/callbacks"
)

func main() {
	ctx := context.Background()

	// init apmplus callback, for trace metrics and log
	fmt.Println("INFO: use apmplus as callback, watch at: https://console.volcengine.com/apmplus-server/region:apmplus-server+cn-beijing/console/overview/server?")

	cbh, shutdown, err := apmplus.NewApmplusHandler(&apmplus.Config{
		Host:        "apmplus-cn-beijing.volces.com:4317",
		AppKey:      "xxx",
		ServiceName: "eino-chat",
		Release:     "release/v0.0.1",
	})
	if shutdown != nil {
		defer shutdown(ctx)
	}
	if err != nil {
		log.Fatal(err)
	}

	// Set apmplus as a global callback
	callbacks.InitCallbackHandlers([]callbacks.Handler{cbh})
}
