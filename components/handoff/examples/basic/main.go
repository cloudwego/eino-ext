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

// Package main demonstrates basic usage of the handoff package.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bytedance/eino-ext/components/handoff"
)

func main() {
	ctx := context.Background()

	// Create a handoff handler
	handler := handoff.NewHandler(&handoff.HandlerConfig{
		SessionID:          "example-session-001",
		SessionDescription: "演示如何使用 handoff 包",
		CodeTracker:        handoff.NewDefaultCodeTracker(),
	})

	// Simulate some work by recording events
	simulateWork(handler)

	// Generate handoff document
	fmt.Println("正在生成 handoff 文档...")

	doc, err := handler.Generate(ctx, &handoff.GenerateOptions{
		TaskTitle:       "实现用户认证系统",
		TaskDescription: "使用 JWT 实现无状态认证",
		TaskStatus:      handoff.TaskStatusInProgress,
		TaskProgress:    65,
	})
	if err != nil {
		fmt.Printf("生成文档失败: %v\n", err)
		os.Exit(1)
	}

	// Print document info
	fmt.Printf("\n文档信息:\n")
	fmt.Printf("  会话 ID: %s\n", doc.Session.ID)
	fmt.Printf("  任务: %s\n", doc.CurrentTask.Title)
	fmt.Printf("  进度: %d%%\n", doc.CurrentTask.Progress)
	fmt.Printf("  持续时间: %v\n", doc.Duration())
	fmt.Printf("  决策数: %d\n", len(doc.Content.Decisions))
	fmt.Printf("  下一步数: %d\n", len(doc.Content.NextSteps))

	// Save to file
	outputPath := "handoff_example.md"
	if err := doc.Save(outputPath); err != nil {
		fmt.Printf("保存文档失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n文档已保存到: %s\n", outputPath)

	// Print document content preview
	content, _ := doc.String()
	fmt.Printf("\n文档预览:\n%s\n", content[:min(len(content), 800)])
}

func simulateWork(handler *handoff.Handler) {
	// Record some decisions
	handler.MarkDecision(handoff.Decision{
		Title:     "使用 JWT 而非 Session",
		Reasoning: "支持无状态架构，易于水平扩展",
		Status:    "decided",
		Time:      time.Now().Add(-30 * time.Minute),
	})

	handler.MarkDecision(handoff.Decision{
		Title:     "使用 RS256 签名算法",
		Reasoning: "比 HS256 更安全，支持密钥轮换",
		Status:    "decided",
		Time:      time.Now().Add(-20 * time.Minute),
	})

	handler.MarkDecision(handoff.Decision{
		Title:     "Access Token 有效期 15 分钟",
		Reasoning: "平衡安全性和用户体验",
		Status:    "decided",
		Time:      time.Now().Add(-10 * time.Minute),
	})

	// Record a milestone
	handler.MarkMilestone(handoff.Milestone{
		Title:       "完成接口设计",
		Description: "定义了 AuthService 和 TokenManager 接口",
		CompletedAt: time.Now().Add(-25 * time.Minute),
	})

	// Record custom events
	handler.RecordEvent(handoff.EventTypeMessage, handoff.EventData{
		Role:    "user",
		Content: "请帮我实现 JWT 认证系统",
	})

	handler.RecordEvent(handoff.EventTypeToolCall, handoff.EventData{
		ToolName: "create_file",
		Input: map[string]interface{}{
			"path": "auth/service.go",
		},
	})

	handler.RecordEvent(handoff.EventTypeToolCall, handoff.EventData{
		ToolName: "create_file",
		Input: map[string]interface{}{
			"path": "auth/jwt.go",
		},
	})

	handler.RecordEvent(handoff.EventTypeMessage, handoff.EventData{
		Role:    "assistant",
		Content: "我已经创建了基本结构，接下来实现登录逻辑...",
	})

	// Add custom data
	handler.AddCustomData("framework", "gin")
	handler.AddCustomData("database", "postgresql")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
