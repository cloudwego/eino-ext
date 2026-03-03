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
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockLangsmith 实现 Langsmith 接口，用于注入可控行为
type mockLangsmith struct {
	mock.Mock
}

func (m *mockLangsmith) CreateRun(ctx context.Context, run *Run) error {
	args := m.Called(ctx, run)
	return args.Error(0)
}

func (m *mockLangsmith) UpdateRun(ctx context.Context, runID string, patch *RunPatch) error {
	args := m.Called(ctx, runID, patch)
	return args.Error(0)
}

// TestNewLangsmithHandler 测试构造函数
func TestNewLangsmithHandler(t *testing.T) {
	cfg := &Config{APIKey: "test-key", APIURL: "http://test"}
	h, err := NewLangsmithHandler(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, h)
}

// TestOnStart 测试 OnStart 正常流程
func TestOnStart(t *testing.T) {
	mCli := new(mockLangsmith)
	cfg := &Config{APIKey: "test-key", APIURL: "http://test"}
	h, _ := NewLangsmithHandler(cfg)

	ctx := context.Background()
	info := &callbacks.RunInfo{Component: "test"}
	input := callbacks.CallbackInput("hello")

	// 期望 CreateRun 被调用一次
	mCli.On("CreateRun", mock.Anything, mock.Anything).Return(nil)

	newCtx := h.OnStart(ctx, info, input)
	assert.NotNil(t, newCtx)
}

// TestOnEnd 测试 OnEnd 正常流程
func TestOnEnd(t *testing.T) {
	mCli := new(mockLangsmith)
	cfg := &Config{APIKey: "test-key", APIURL: "http://test"}
	h, _ := NewLangsmithHandler(cfg)

	ctx := context.WithValue(context.Background(), langsmithStateKey{}, &LangsmithState{
		ParentRunID: "run-123",
	})
	info := &callbacks.RunInfo{Component: "test"}
	output := callbacks.CallbackOutput("world")

	mCli.On("UpdateRun", mock.Anything, "run-123", mock.Anything).Return(nil)

	newCtx := h.OnEnd(ctx, info, output)
	assert.NotNil(t, newCtx)
}

// TestOnError 测试 OnError 正常流程
func TestOnError(t *testing.T) {
	mCli := new(mockLangsmith)
	cfg := &Config{APIKey: "test-key", APIURL: "http://test"}
	h, _ := NewLangsmithHandler(cfg)

	ctx := context.WithValue(context.Background(), langsmithStateKey{}, &LangsmithState{
		ParentRunID: "run-123",
	})
	info := &callbacks.RunInfo{Component: "test"}
	err := errors.New("mock error")

	mCli.On("UpdateRun", mock.Anything, "run-123", mock.Anything).Return(nil)

	newCtx := h.OnError(ctx, info, err)
	assert.NotNil(t, newCtx)

}

// TestOnStartWithStreamInput 测试流式输入
func TestOnStartWithStreamInput(t *testing.T) {
	mCli := new(mockLangsmith)
	cfg := &Config{APIKey: "test-key", APIURL: "http://test"}
	h, _ := NewLangsmithHandler(cfg)

	ctx := context.Background()
	info := &callbacks.RunInfo{Component: "test"}

	// 用 schema.Pipe 构造一个空的 StreamReader[callbacks.CallbackInput]
	sr, sw := schema.Pipe[callbacks.CallbackInput](1)
	sw.Close() // 立即关闭，模拟无数据流

	// 期望 CreateRun 被调用一次
	mCli.On("CreateRun", mock.Anything, mock.Anything).Return(nil)

	newCtx := h.OnStartWithStreamInput(ctx, info, sr)
	assert.NotNil(t, newCtx)

	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)

}

// TestOnEndWithStreamOutput 测试流式输出
func TestOnEndWithStreamOutput(t *testing.T) {
	mCli := new(mockLangsmith)
	cfg := &Config{APIKey: "test-key", APIURL: "http://test"}
	h, _ := NewLangsmithHandler(cfg)

	ctx := context.WithValue(context.Background(), langsmithStateKey{}, &LangsmithState{
		ParentRunID: "run-123",
	})
	info := &callbacks.RunInfo{Component: "test"}

	// 用 schema.Pipe 构造一个空的 StreamReader[callbacks.CallbackOutput]
	sr, sw := schema.Pipe[callbacks.CallbackOutput](1)
	sw.Close() // 立即关闭，模拟无数据流

	mCli.On("UpdateRun", mock.Anything, "run-123", mock.Anything).Return(nil)

	newCtx := h.OnEndWithStreamOutput(ctx, info, sr)
	assert.NotNil(t, newCtx)

	// 等待 goroutine 完成
	time.Sleep(100 * time.Millisecond)
}
