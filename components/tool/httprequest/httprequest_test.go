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

package httprequest

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewToolKit_Success(t *testing.T) {
	ctx := context.Background()
	conf := &Config{
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		HttpClient: &http.Client{},
	}

	tools, err := NewToolKit(ctx, conf)
	assert.NoError(t, err)
	assert.Len(t, tools, 4)

	var toolNames []string
	for _, tool := range tools {
		currentTool, _ := tool.Info(ctx)
		toolNames = append(toolNames, currentTool.Name)
	}
	assert.Contains(t, toolNames, "request_get")
	assert.Contains(t, toolNames, "requests_post")
	assert.Contains(t, toolNames, "requests_put")
	assert.Contains(t, toolNames, "requests_delete")
}

func TestNewToolKit_NilConfig(t *testing.T) {
	ctx := context.Background()
	tools, err := NewToolKit(ctx, nil)
	assert.NoError(t, err)
	assert.Len(t, tools, 4)

	var toolNames []string
	for _, tool := range tools {
		currentTool, _ := tool.Info(ctx)
		toolNames = append(toolNames, currentTool.Name)
	}
	assert.Contains(t, toolNames, "request_get")
	assert.Contains(t, toolNames, "requests_post")
	assert.Contains(t, toolNames, "requests_put")
	assert.Contains(t, toolNames, "requests_delete")
}
