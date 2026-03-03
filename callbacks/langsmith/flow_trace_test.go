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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestNewFlowTrace 测试构造函数
func TestNewFlowTrace(t *testing.T) {
	cfg := &Config{
		APIKey: "test-key",
		APIURL: "http://test.com",
	}

	ft := NewFlowTrace(cfg)
	assert.NotNil(t, ft)
	assert.NotNil(t, ft.cfg.RunIDGen)
}

// TestFlowTrace_StartSpan 测试 StartSpan 正常流程
func TestFlowTrace_StartSpan(t *testing.T) {
	mCli := new(mockLangsmith)
	ft := &FlowTrace{
		cli: mCli,
		cfg: &Config{
			RunIDGen: func(ctx context.Context) string {
				return "test-run-id"
			},
		},
	}

	t.Run("basic span creation", func(t *testing.T) {
		ctx := context.Background()

		// 期望 CreateRun 被调用一次
		mCli.On("CreateRun", mock.Anything, mock.Anything).Return(nil)

		newCtx, runID, err := ft.StartSpan(ctx, "test-span", nil)
		require.NoError(t, err)
		assert.Equal(t, "test-run-id", runID)
		assert.NotNil(t, newCtx)

		mCli.AssertExpectations(t)
	})

	t.Run("with trace options", func(t *testing.T) {
		ctx := SetTrace(context.Background(),
			WithSessionName("test-project"),
			WithReferenceExampleID("example-123"),
			AddTag("tag1"),
			AddTag("tag2"),
		)

		state := &LangsmithState{
			TraceID:           "parent-trace-id",
			ParentRunID:       "parent-run-id",
			ParentDottedOrder: "parent.dotted.order",
		}

		// 期望 CreateRun 被调用一次
		mCli.On("CreateRun", mock.Anything, mock.Anything).Return(nil)

		newCtx, runID, err := ft.StartSpan(ctx, "span-with-options", state)
		require.NoError(t, err)
		assert.Equal(t, "test-run-id", runID)
		assert.NotNil(t, newCtx)

		mCli.AssertExpectations(t)
	})

	t.Run("empty state", func(t *testing.T) {
		ctx := context.Background()

		// 期望 CreateRun 被调用一次
		mCli.On("CreateRun", mock.Anything, mock.Anything).Return(nil)

		_, runID, err := ft.StartSpan(ctx, "empty-state", &LangsmithState{})
		require.NoError(t, err)
		assert.Equal(t, "test-run-id", runID)

		mCli.AssertExpectations(t)
	})
}

// TestFlowTrace_FinishSpan 测试 FinishSpan 正常流程
func TestFlowTrace_FinishSpan(t *testing.T) {
	mCli := new(mockLangsmith)
	ft := &FlowTrace{
		cli: mCli,
	}

	ctx := context.WithValue(context.Background(), langsmithStateKey{}, &LangsmithState{
		ParentRunID: "run-123",
	})

	// 期望 UpdateRun 被调用一次
	mCli.On("UpdateRun", mock.Anything, "run-123", mock.Anything).Return(nil)

	ft.FinishSpan(ctx, "run-123")
	mCli.AssertExpectations(t)
}

// TestFlowTrace_SpanToString 测试状态序列化
func TestFlowTrace_SpanToString(t *testing.T) {
	ft := &FlowTrace{}

	t.Run("empty state", func(t *testing.T) {
		ctx := context.Background()
		str, err := ft.SpanToString(ctx)
		assert.NoError(t, err)
		assert.Empty(t, str)
	})

	t.Run("valid state", func(t *testing.T) {
		metadata := &sync.Map{}
		metadata.Store("key", "value")

		state := &LangsmithState{
			TraceID:           "test-trace-id",
			ParentRunID:       "test-parent-id",
			ParentDottedOrder: "test.dotted.order",
			Metadata:          metadata,
			Tags:              []string{"tag1", "tag2"},
		}

		ctx := context.WithValue(context.Background(), langsmithStateKey{}, state)
		str, err := ft.SpanToString(ctx)
		assert.NoError(t, err)
		assert.Contains(t, str, "test-trace-id")
		assert.Contains(t, str, "test-parent-id")
		assert.Contains(t, str, "test.dotted.order")
	})
}

// TestFlowTrace_StringToSpan 测试状态反序列化
func TestFlowTrace_StringToSpan(t *testing.T) {
	ft := &FlowTrace{}

	t.Run("empty string", func(t *testing.T) {
		state, err := ft.StringToSpan("")
		assert.NoError(t, err)
		assert.Nil(t, state)
	})

	t.Run("valid JSON", func(t *testing.T) {
		jsonStr := `{"trace_id":"test-trace","parent_run_id":"test-parent","parent_dotted_order":"test.order","marshal_metadata":{"key":"value"},"tags":["tag1","tag2"]}`
		state, err := ft.StringToSpan(jsonStr)
		assert.NoError(t, err)
		assert.NotNil(t, state)
		assert.Equal(t, "test-trace", state.TraceID)
		assert.Equal(t, "test-parent", state.ParentRunID)
		assert.Equal(t, "test.order", state.ParentDottedOrder)

		// 验证metadata正确转换为sync.Map
		assert.NotNil(t, state.Metadata)
		val, ok := state.Metadata.Load("key")
		assert.True(t, ok)
		assert.Equal(t, "value", val)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		state, err := ft.StringToSpan("invalid json")
		assert.Error(t, err)
		assert.Nil(t, state)
	})
}

// TestFlowTrace_Integration 测试完整流程
func TestFlowTrace_Integration(t *testing.T) {
	mCli := new(mockLangsmith)
	ft := &FlowTrace{
		cli: mCli,
		cfg: &Config{
			RunIDGen: func(ctx context.Context) string {
				return "integration-test-id"
			},
		},
	}

	// 设置mock期望
	mCli.On("CreateRun", mock.Anything, mock.Anything).Return(nil)
	mCli.On("UpdateRun", mock.Anything, "integration-test-id", mock.Anything).Return(nil)

	// 完整流程测试
	ctx := SetTrace(context.Background(),
		WithSessionName("integration-project"),
		AddTag("integration"),
	)

	// 开始span
	newCtx, runID, err := ft.StartSpan(ctx, "integration-span", nil)
	require.NoError(t, err)
	assert.Equal(t, "integration-test-id", runID)

	// 验证状态已设置
	state := newCtx.Value(langsmithStateKey{}).(*LangsmithState)
	assert.NotNil(t, state)
	assert.Equal(t, "integration-test-id", state.TraceID)

	// 序列化状态
	stateStr, err := ft.SpanToString(newCtx)
	assert.NoError(t, err)
	assert.NotEmpty(t, stateStr)

	// 反序列化状态
	newState, err := ft.StringToSpan(stateStr)
	assert.NoError(t, err)
	assert.Equal(t, state.TraceID, newState.TraceID)
	assert.Equal(t, state.ParentRunID, newState.ParentRunID)
	assert.Equal(t, state.ParentDottedOrder, newState.ParentDottedOrder)

	// 结束span
	ft.FinishSpan(newCtx, runID)

	mCli.AssertExpectations(t)
}
