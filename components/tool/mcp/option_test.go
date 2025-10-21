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

package mcp

import (
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/stretchr/testify/assert"
)

func TestWithCustomHeaders(t *testing.T) {
	headers := map[string]string{
		"Authorization": "Bearer 123456",
	}
	opt := WithCustomHeaders(headers)
	opts := tool.GetImplSpecificOptions(&mcpOptions{}, opt)
	assert.Equal(t, headers, opts.customHeaders)
}
