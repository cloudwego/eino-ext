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

package langsmith

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Langsmith 定义了与 Langsmith API 交互的客户端接口
type Langsmith interface {
	CreateRun(ctx context.Context, run *Run) error
	UpdateRun(ctx context.Context, runID string, patch *RunPatch) error
}

const (
	// DefaultLangsmithAPIURL 是 Langsmith API 的默认地址
	DefaultLangsmithAPIURL = "https://api.smith.langchain.com"
)

// RunType 定义了 run 的类型
type RunType string

const (
	RunTypeChain RunType = "chain"
	RunTypeLLM   RunType = "llm"
	RunTypeTool  RunType = "tool"
)

// Run 代表一个 Langsmith run
type Run struct {
	ID                 string                 `json:"id"`
	Name               string                 `json:"name"`
	RunType            RunType                `json:"run_type"`
	StartTime          time.Time              `json:"start_time"`
	EndTime            *time.Time             `json:"end_time,omitempty"`
	Inputs             map[string]interface{} `json:"inputs"`
	Outputs            map[string]interface{} `json:"outputs,omitempty"`
	Error              *string                `json:"error,omitempty"`
	ParentRunID        *string                `json:"parent_run_id,omitempty"`
	TraceID            string                 `json:"trace_id,omitempty"`
	Extra              map[string]interface{} `json:"extra,omitempty"`
	SessionName        string                 `json:"session_name,omitempty"`
	ReferenceExampleID *string                `json:"reference_example_id,omitempty"`
	DottedOrder        string                 `json:"dotted_order,omitempty"`
}

// RunPatch 用于更新一个 run
type RunPatch struct {
	EndTime *time.Time             `json:"end_time,omitempty"`
	Inputs  map[string]interface{} `json:"inputs,omitempty"`
	Outputs map[string]interface{} `json:"outputs,omitempty"`
	Error   *string                `json:"error,omitempty"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

// langsmithClient 实现了 Langsmith 接口
type langsmithClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewLangsmith 创建一个新的 Langsmith 客户端
func NewLangsmith(apiKey, apiUrl string) Langsmith {
	if apiUrl == "" {
		apiUrl = DefaultLangsmithAPIURL
	}
	return &langsmithClient{
		apiKey:     apiKey,
		baseURL:    apiUrl,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// CreateRun 在 Langsmith 中创建一个新的 run
func (c *langsmithClient) CreateRun(ctx context.Context, run *Run) error {
	jsonData, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("failed to marshal run data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/runs", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create run, status: %s, body: %s", resp.Status, string(body))
	}

	// 将返回的 run 数据解码回传入的 run 对象，这样调用者可以获取到 ID 等服务端生成的数据
	if err := json.NewDecoder(resp.Body).Decode(run); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	return nil
}

// UpdateRun 更新 Langsmith 中的一个 run
func (c *langsmithClient) UpdateRun(ctx context.Context, runID string, patch *RunPatch) error {
	jsonData, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch data: %w", err)
	}

	url := fmt.Sprintf("%s/runs/%s", c.baseURL, runID)
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update run, status: %s, body: %s", resp.Status, string(body))
	}

	return nil
}
