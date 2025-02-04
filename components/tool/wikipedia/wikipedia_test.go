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

package wikipedia

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/wikipedia/wikipediaclient"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewTool(t *testing.T) {
	ctx := context.Background()
	tool, err := NewTool(ctx, &Config{})
	assert.NoError(t, err)
	assert.NotNil(t, tool)
}

func TestWikipedia_Search(t *testing.T) {
	ctx := context.Background()
	tool, err := NewTool(ctx, &Config{})
	assert.NoError(t, err)
	assert.NotNil(t, tool)
	test := []struct {
		name  string
		query *SearchRequest
		err   error
	}{
		{"normal1", &SearchRequest{"bytedance"}, nil},
		{"normal2", &SearchRequest{"Go programming language"}, nil},
		{"InvalidParameters", &SearchRequest{""}, fmt.Errorf("[LocalFunc] failed to invoke tool: %w", wikipediaclient.ErrInvalidParameters)},
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			m, err := sonic.MarshalString(tt.query)
			assert.NoError(t, err)
			toolRes, err := tool.InvokableRun(ctx, m)
			assert.Equal(t, tt.err, err)
			assert.NotNil(t, toolRes)
		})
	}
}
