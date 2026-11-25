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

package ark

import (
	"net/http"
	"time"

	"github.com/volcengine/ark-runtime-go/arkruntime"
)

type clientConfig struct {
	timeout    *time.Duration
	httpClient *http.Client
	retryTimes *int
	baseURL    string
	region     string
	apiKey     string
	accessKey  string
	secretKey  string
}

func newArkClient(config clientConfig) *arkruntime.Client {
	opts := []arkruntime.ConfigOption{}
	if config.baseURL != "" {
		opts = append(opts, arkruntime.WithBaseUrl(config.baseURL))
	}
	if config.region != "" {
		opts = append(opts, arkruntime.WithRegion(config.region))
	}
	if config.timeout != nil {
		opts = append(opts, arkruntime.WithTimeout(*config.timeout))
	}
	if config.retryTimes != nil {
		opts = append(opts, arkruntime.WithRetryTimes(*config.retryTimes))
	}
	if config.httpClient != nil {
		httpClient := *config.httpClient
		opts = append(opts, arkruntime.WithHTTPClient(&httpClient))
	}
	if config.apiKey != "" {
		return arkruntime.NewClientWithApiKey(config.apiKey, opts...)
	}
	return arkruntime.NewClientWithAkSk(config.accessKey, config.secretKey, opts...)
}
