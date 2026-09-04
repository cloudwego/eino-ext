// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main demonstrates using the Wrapper for quick integration.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bytedance/eino/adk"
	"github.com/bytedance/eino-ext/components/handoff"
)

// MockAgent is a simple mock agent for demonstration.
type MockAgent struct {
	name string
}

func (a *MockAgent) Name(ctx context.Context) string {
	return a.name
}

func (a *MockAgent) Description(ctx context.Context) string {
	return "Mock agent for demonstration"
}

func (a *MockAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	// In a real scenario, this would process the input and generate events
	// For this example, we just return an empty iterator
	return adk.NewAsyncIterator(make(chan *adk.AgentEvent))
}

func main() {
	ctx := context.Background()

	// Create a mock agent
	agent := &MockAgent{name: "demo-agent"}

	// Wrap the agent with handoff capabilities
	wrapped, err := handoff.Wrap(agent, &handoff.WrapConfig{
		SessionID:  "wrapper-example",
		OutputPath: "./handoffs/",
		OnBeforeHandoff: func(ctx context.Context, h *handoff.HandoffContext) error {
			fmt.Println("即将生成 handoff 文档...")
			return nil
		},
		OnAfterHandoff: func(ctx context.Context, path string) {
			fmt.Printf("Handoff 文档已保存到: %s\n", path)
		},
	})
	if err != nil {
		fmt.Printf("包装 agent 失败: %v\n", err)
		os.Exit(1)
	}

	// Use the wrapped agent like a normal agent
	input := &adk.AgentInput{Input: "帮我实现一个功能"}
	events := wrapped.Run(ctx, input)

	// Process events (in a real scenario)
	_ = events

	// Mark some milestones during work
	wrapped.MarkMilestone(handoff.Milestone{
		Title:       "需求分析完成",
		Description: "确定了功能范围和实现方案",
	})

	wrapped.MarkDecision(handoff.Decision{
		Title:     "使用 Clean Architecture",
		Reasoning: "提高代码可测试性和可维护性",
		Status:    "decided",
	})

	// Request handoff generation
	outputPath, err := wrapped.GenerateHandoff(ctx, &handoff.GenerateOptions{
		TaskTitle:       "实现订单管理模块",
		TaskDescription: "包含订单创建、查询、更新功能",
		TaskStatus:      handoff.TaskStatusInProgress,
		TaskProgress:    40,
	})
	if err != nil {
		fmt.Printf("生成 handoff 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nHandoff 文档路径: %s\n", outputPath)

	// Read and display the generated document
	loader := handoff.NewLoader()
	doc, err := loader.Load(outputPath)
	if err != nil {
		fmt.Printf("加载文档失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n生成的文档摘要:\n")
	fmt.Printf("  任务: %s\n", doc.CurrentTask.Title)
	fmt.Printf("  进度: %d%%\n", doc.CurrentTask.Progress)
	fmt.Printf("  状态: %s\n", doc.CurrentTask.Status)
	fmt.Printf("  决策数: %d\n", len(doc.Content.Decisions))
}
