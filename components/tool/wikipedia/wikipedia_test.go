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
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/components/tool/wikipedia/internal"
)

func TestNewTool(t *testing.T) {
	ctx := context.Background()
	tool, err := NewTool(ctx, &Config{})
	assert.NoError(t, err)
	assert.NotNil(t, tool)

	info, err := tool.Info(ctx)
	assert.Nil(t, err)

	doc, err := info.ParamsOneOf.ToJSONSchema()
	assert.Nil(t, err)
	assert.Equal(t, 1, doc.Properties.Len())
	for pair := doc.Properties.Oldest(); pair != nil; pair = pair.Next() {
		assert.NotEqual(t, "", pair.Value.Description)
	}
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
		{"InvalidParameters", &SearchRequest{""}, fmt.Errorf("[LocalFunc] failed to invoke tool, toolName=wikipedia_search, err=%w", internal.ErrInvalidParameters)},
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
