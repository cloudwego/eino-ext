/*
 * Copyright 2024 CloudWeGo Authors
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

package internal

import (
	"net/http"
)

// ClientOption is a functional option for the Wikipedia client.
type ClientOption func(*WikipediaClient)

// WithHTTPClient sets the HTTP client for the Wikipedia client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *WikipediaClient) {
		c.httpClient = client
	}
}

// WithLanguage sets the language for the Wikipedia client.
func WithLanguage(lang string) ClientOption {
	return func(c *WikipediaClient) {
		c.language = lang
	}
}

// WithBaseURL sets the base URL for the Wikipedia client.
func WithBaseURL(url string) ClientOption {
	return func(c *WikipediaClient) {
		c.baseURL = url
	}
}

// WithUserAgent sets the user agent for the Wikipedia client.
func WithUserAgent(ua string) ClientOption {
	return func(c *WikipediaClient) {
		c.userAgent = ua
	}
}

// WithTopK sets the number of search results to return.
func WithTopK(topK int) ClientOption {
	return func(c *WikipediaClient) {
		c.topK = topK
	}
}
