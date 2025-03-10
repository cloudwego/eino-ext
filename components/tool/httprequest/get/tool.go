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
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type Config struct {
	// Inspired by the "Requests" tool from the LangChain project, specifically the RequestsGetTool.
	// For more details, visit: https://python.langchain.com/docs/integrations/tools/requests/
	// Optional. Default: "request_get".
	ToolName string `json:"tool_name"`
	// Optional. Default: "A portal to the internet. Use this tool when you need to fetch specific content from a website.
	// Input should be a URL (e.g., https://www.google.com). The output will be the text response from the GET request."
	ToolDesc string `json:"tool_desc"`

	// Optional.
	// Headers is a map of HTTP header names to their corresponding values.
	// These headers will be included in every request made by the tool.
	Headers map[string]string `json:"headers"`

	// Optional.
	// HttpClient is the HTTP client used to perform the requests.
	// If not provided, a default client with a 30-second timeout and a standard transport
	// will be initialized and used.
	HttpClient *http.Client
}

func (c *Config) validate() error {
	if c.ToolName == "" {
		c.ToolName = "request_get"
	}
	if c.ToolDesc == "" {
		c.ToolDesc = `A portal to the internet. Use this when you need to get specific
		content from a website. Input should be a URL (i.e. https://www.google.com).
		The output will be the text response of the GET request.`
	}
	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	if c.HttpClient == nil {
		c.HttpClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{},
		}
	}
	return nil
}

func NewTool(ctx context.Context, config *Config) (tool.InvokableTool, error) {
	reqTool, err := newRequestTool(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create request tool: %w", err)
	}

	invokableTool, err := utils.InferTool(config.ToolName, config.ToolDesc, reqTool.Get)
	if err != nil {
		return nil, fmt.Errorf("failed to infer the tool: %w", err)
	}

	return invokableTool, nil
}

type GetRequestTool struct {
	config *Config
	client *http.Client
}

func newRequestTool(config *Config) (*GetRequestTool, error) {
	if config == nil {
		return nil, errors.New("request tool configuration is required")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &GetRequestTool{
		config: config,
		client: config.HttpClient,
	}, nil
}
