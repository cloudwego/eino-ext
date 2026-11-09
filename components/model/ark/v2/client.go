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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		opts = append(opts, arkruntime.WithHTTPClient(config.httpClient))
	}
	// Copy the SDK-configured HTTP client so capturing response bodies does not
	// mutate a caller-owned client or change its transport/timeout policies.
	connection := arkruntime.NewClientConfig(config.apiKey, config.accessKey, config.secretKey, opts...)
	httpClient := *connection.HTTPClient
	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpClient.Transport = &responseCaptureTransport{next: transport}
	opts = append(opts, arkruntime.WithHTTPClient(&httpClient))
	if config.apiKey != "" {
		return arkruntime.NewClientWithApiKey(config.apiKey, opts...)
	}
	return arkruntime.NewClientWithAkSk(config.accessKey, config.secretKey, opts...)
}

type responseCaptureKey struct{}
type responseCapture struct {
	rawBody []byte
}

type responseCaptureTransport struct {
	next http.RoundTripper
}

func (t *responseCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	capture, _ := req.Context().Value(responseCaptureKey{}).(*responseCapture)
	if capture == nil || (!strings.HasSuffix(req.URL.Path, "/chat/completions") && !strings.HasSuffix(req.URL.Path, "/responses")) {
		return t.next.RoundTrip(req)
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, err
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response payload: %w", err)
	}
	capture.rawBody = raw
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	return resp, nil
}
