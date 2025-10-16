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
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetTrace(t *testing.T) {
	ctx := context.Background()

	// Test empty options
	ctx1 := SetTrace(ctx)
	opts1 := ctx1.Value(langsmithTraceOptionKey{}).(*traceOptions)
	assert.NotNil(t, opts1)
	assert.Empty(t, opts1.SessionName)
	assert.Empty(t, opts1.ReferenceExampleID)
	assert.Empty(t, opts1.Tags)

	// Test with options
	metadata := &sync.Map{}
	metadata.Store("key1", "value1")

	ctx2 := SetTrace(ctx,
		WithSessionName("test-project"),
		WithReferenceExampleID("example-123"),
		WithTraceID("trace-456"),
		AddTag("tag1"),
		AddTag("tag2"),
		AddTag("tag1"), // 重复tag不应重复添加
		SetMetadata(metadata),
	)

	opts2 := ctx2.Value(langsmithTraceOptionKey{}).(*traceOptions)
	assert.Equal(t, "test-project", opts2.SessionName)
	assert.Equal(t, "example-123", opts2.ReferenceExampleID)
	assert.Equal(t, "trace-456", opts2.TraceID)
	assert.ElementsMatch(t, []string{"tag1", "tag2"}, opts2.Tags)
	assert.NotNil(t, opts2.Metadata)

	val, ok := opts2.Metadata.Load("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)
}

func TestWithSessionName(t *testing.T) {
	opt := WithSessionName("my-project")
	options := &traceOptions{}
	opt(options)
	assert.Equal(t, "my-project", options.SessionName)
}

func TestWithReferenceExampleID(t *testing.T) {
	opt := WithReferenceExampleID("example-abc")
	options := &traceOptions{}
	opt(options)
	assert.Equal(t, "example-abc", options.ReferenceExampleID)
}

func TestWithTraceID(t *testing.T) {
	opt := WithTraceID("trace-xyz")
	options := &traceOptions{}
	opt(options)
	assert.Equal(t, "trace-xyz", options.TraceID)
}

func TestAddTag(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		addTag   string
		expected []string
	}{
		{
			name:     "add to empty",
			initial:  []string{},
			addTag:   "tag1",
			expected: []string{"tag1"},
		},
		{
			name:     "add new tag",
			initial:  []string{"tag1"},
			addTag:   "tag2",
			expected: []string{"tag1", "tag2"},
		},
		{
			name:     "add duplicate tag",
			initial:  []string{"tag1", "tag2"},
			addTag:   "tag1",
			expected: []string{"tag1", "tag2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &traceOptions{Tags: tt.initial}
			opt := AddTag(tt.addTag)
			opt(options)
			assert.ElementsMatch(t, tt.expected, options.Tags)
		})
	}
}

func TestSetMetadata(t *testing.T) {
	tests := []struct {
		name     string
		initial  *sync.Map
		newData  *sync.Map
		expected map[string]interface{}
	}{
		{
			name:     "nil initial",
			initial:  nil,
			newData:  &sync.Map{},
			expected: map[string]interface{}{},
		},
		{
			name:    "merge with existing",
			initial: func() *sync.Map { m := &sync.Map{}; m.Store("old", "value"); return m }(),
			newData: func() *sync.Map { m := &sync.Map{}; m.Store("new", "data"); return m }(),
			expected: map[string]interface{}{
				"old": "value",
				"new": "data",
			},
		},
		{
			name:    "overwrite existing",
			initial: func() *sync.Map { m := &sync.Map{}; m.Store("key", "old"); return m }(),
			newData: func() *sync.Map { m := &sync.Map{}; m.Store("key", "new"); return m }(),
			expected: map[string]interface{}{
				"key": "new",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &traceOptions{Metadata: tt.initial}
			opt := SetMetadata(tt.newData)
			opt(options)

			result := map[string]interface{}{}
			if options.Metadata != nil {
				options.Metadata.Range(func(k, v interface{}) bool {
					result[k.(string)] = v
					return true
				})
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTraceOptionChaining(t *testing.T) {
	ctx := context.Background()

	ctx = SetTrace(ctx,
		WithSessionName("project1"),
		WithReferenceExampleID("ref1"),
		AddTag("tag1"),
		WithTraceID("trace1"),
	)

	// 验证可以链式调用
	ctx = SetTrace(ctx,
		WithSessionName("project2"),
		AddTag("tag2"),
	)

	opts := ctx.Value(langsmithTraceOptionKey{}).(*traceOptions)
	assert.Equal(t, "project2", opts.SessionName)
	assert.NotEqual(t, "ref1", opts.ReferenceExampleID) // 这个应该被重置
	assert.NotEqual(t, "trace1", opts.TraceID)          // 这个应该被重置
	assert.ElementsMatch(t, []string{"tag2"}, opts.Tags)
}
