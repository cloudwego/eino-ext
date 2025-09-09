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

package get

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

type mockTransport struct {
	RoundTripFunc func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

type errorReader struct{}

func (errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func (errorReader) Close() error {
	return nil
}

func TestGet_Success(t *testing.T) {
	mockResponse := `{"message": "Hello, World!"}`
	mockTransport := &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "https://example.com" {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResponse)),
				}, nil
			}
			return nil, fmt.Errorf("unexpected URL")
		},
	}
	client := &http.Client{Transport: mockTransport}
	tool := &GetRequestTool{
		config: &Config{
			Headers: make(map[string]string),
		},
		client: client,
	}

	req := &GetRequest{URL: "https://example.com"}
	result, err := tool.Get(context.Background(), req)
	assert.NoError(t, err)

	assert.Equal(t, mockResponse, result)
}

func TestGet_InvalidURL(t *testing.T) {
	tool := &GetRequestTool{
		config: &Config{
			Headers: make(map[string]string),
		},
		client: &http.Client{},
	}
	req := &GetRequest{URL: "http://:invalid"}
	_, err := tool.Get(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestGet_RequestError(t *testing.T) {
	mockTransport := &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network error")
		},
	}
	client := &http.Client{Transport: mockTransport}
	tool := &GetRequestTool{
		config: &Config{
			Headers: make(map[string]string),
		},
		client: client,
	}
	req := &GetRequest{URL: "https://example.com"}
	_, err := tool.Get(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute request")
}

func TestGet_ReadBodyError(t *testing.T) {
	mockTransport := &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       errorReader{},
			}, nil
		},
	}
	client := &http.Client{Transport: mockTransport}
	tool := &GetRequestTool{
		config: &Config{
			Headers: make(map[string]string),
		},
		client: client,
	}
	req := &GetRequest{URL: "https://example.com"}
	_, err := tool.Get(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read response body")
}

func TestConfig_Validate_Defaults(t *testing.T) {
	config := &Config{}
	err := config.validate()
	assert.NoError(t, err)
	assert.Equal(t, "request_get", config.ToolName)
	assert.NotEmpty(t, config.ToolDesc)
	assert.NotNil(t, config.Headers)
	assert.NotNil(t, config.HttpClient)
}

func TestConfig_Validate_WithValues(t *testing.T) {
	customClient := &http.Client{}
	config := &Config{
		ToolName:   "custom_get",
		ToolDesc:   "Custom description",
		Headers:    map[string]string{"Authorization": "Bearer token"},
		HttpClient: customClient,
	}
	err := config.validate()
	assert.NoError(t, err)
	assert.Equal(t, "custom_get", config.ToolName)
	assert.Equal(t, "Custom description", config.ToolDesc)
	assert.Equal(t, map[string]string{"Authorization": "Bearer token"}, config.Headers)
	assert.Equal(t, customClient, config.HttpClient)
}

func TestNewTool_NilConfig(t *testing.T) {
	_, err := NewTool(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request tool configuration is required")
}

func TestNewTool_Config(t *testing.T) {
	mockey.PatchConvey("NilConfig", t, func() {
		_, err := NewTool(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "request tool configuration is required")
	})

	mockey.PatchConvey("WithConfig", t, func() {
		tool, err := NewTool(context.Background(), &Config{})
		assert.NoError(t, err)

		info, err := tool.Info(context.Background())
		assert.Nil(t, err)

		doc, err := info.ParamsOneOf.ToJSONSchema()
		assert.Nil(t, err)
		assert.Equal(t, 1, doc.Properties.Len())
		for pair := doc.Properties.Oldest(); pair != nil; pair = pair.Next() {
			assert.NotEqual(t, "", pair.Value.Description)
		}
	})
}

func TestGet_WithHeaders(t *testing.T) {
	var receivedHeaders http.Header
	mockTransport := &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			receivedHeaders = req.Header
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}
	client := &http.Client{Transport: mockTransport}
	tool := &GetRequestTool{
		config: &Config{
			Headers: map[string]string{
				"Authorization": "Bearer token",
				"User-Agent":    "test-agent",
			},
		},
		client: client,
	}

	req := &GetRequest{URL: "https://example.com"}
	_, err := tool.Get(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "Bearer token", receivedHeaders.Get("Authorization"))
	assert.Equal(t, "test-agent", receivedHeaders.Get("User-Agent"))
}
