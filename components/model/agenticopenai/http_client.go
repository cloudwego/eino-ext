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

package agenticopenai

import (
	"net/http"
	"time"
)

func newHTTPClient(timeout, responseHeaderTimeout time.Duration) *http.Client {
	client := &http.Client{Timeout: timeout}
	if responseHeaderTimeout <= 0 {
		return client
	}
	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = transport.Clone()
		transport.ResponseHeaderTimeout = responseHeaderTimeout
		client.Transport = transport
		return client
	}

	client.Transport = http.DefaultTransport
	return client
}

func getResponsesResponseHeaderTimeout(config *ResponsesConfig) *time.Duration {
	return config.ResponseHeaderTimeout
}
