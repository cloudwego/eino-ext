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

package openai

import (
	"net/http"
	"time"
)

func newHTTPClientWithResponseHeaderTimeout(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		return &http.Client{}
	}

	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = transport.Clone()
		transport.ResponseHeaderTimeout = timeout
		return &http.Client{Transport: transport}
	}

	return &http.Client{Transport: http.DefaultTransport}
}

func getRequestTimeout(config *ChatModelConfig) time.Duration {
	if config.RequestTimeout > 0 {
		return config.RequestTimeout
	}
	if config.HTTPClient != nil {
		return 0
	}
	return config.Timeout
}

func getResponseHeaderTimeout(config *ChatModelConfig) time.Duration {
	if config.ResponseHeaderTimeout > 0 {
		return config.ResponseHeaderTimeout
	}
	return config.Timeout
}
