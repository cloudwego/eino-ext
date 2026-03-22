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

package handoff

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/eino/callbacks"
)

func TestCollector_MarkDecision(t *testing.T) {
	c := NewCollector(&CollectorConfig{})

	decision := Decision{
		Title:     "使用 RS256 签名",
		Reasoning: "比 HS256 更安全",
		Status:    "decided",
		Time:      time.Now(),
	}

	c.MarkDecision(decision)

	state := c.GetState()
	if len(state.Decisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(state.Decisions))
	}

	if state.Decisions[0].Title != decision.Title {
		t.Errorf("expected title %s, got %s", decision.Title, state.Decisions[0].Title)
	}
}

func TestCollector_MarkMilestone(t *testing.T) {
	c := NewCollector(&CollectorConfig{})

	milestone := Milestone{
		Title:       "完成接口设计",
		Description: "定义了所有公共接口",
		CompletedAt: time.Now(),
	}

	c.MarkMilestone(milestone)

	state := c.GetState()
	if len(state.CompletedMilestones) != 1 {
		t.Errorf("expected 1 milestone, got %d", len(state.CompletedMilestones))
	}
}

func TestFormatter_FormatAndParse(t *testing.T) {
	doc := &Document{
		Version: "1.0",
		Session: SessionInfo{
			ID:        "test-session",
			StartedAt: time.Now(),
			HandoffAt: time.Now(),
		},
		CurrentTask: TaskInfo{
			Title:    "测试任务",
			Status:   TaskStatusInProgress,
			Progress: 50,
		},
		Content: Content{
			Summary: "这是一个测试摘要",
			Decisions: []Decision{
				{
					Title:     "测试决策",
					Reasoning: "测试理由",
					Status:    "decided",
				},
			},
		},
	}

	formatter := NewYAMLMarkdownFormatter()

	// Format
	data, err := formatter.Format(doc)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}

	// Check output contains expected content
	output := string(data)
	if !strings.Contains(output, "handoff_version: \"1.0\"") {
		t.Error("output missing version")
	}
	if !strings.Contains(output, "测试任务") {
		t.Error("output missing task title")
	}
	if !strings.Contains(output, "任务摘要") {
		t.Error("output missing section header")
	}

	// Parse
	parsed, err := formatter.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if parsed.Version != doc.Version {
		t.Errorf("expected version %s, got %s", doc.Version, parsed.Version)
	}

	if parsed.CurrentTask.Title != doc.CurrentTask.Title {
		t.Errorf("expected task title %s, got %s", doc.CurrentTask.Title, parsed.CurrentTask.Title)
	}
}

func TestSummarizer_Summarize(t *testing.T) {
	summarizer := NewDefaultSummarizer()

	state := NewState("test-session")
	state.Events = []Event{
		{
			Type:      EventTypeMessage,
			Timestamp: time.Now(),
			Data: EventData{
				Role:    "user",
				Content: "请帮我实现用户认证",
			},
		},
		{
			Type:      EventTypeToolCall,
			Timestamp: time.Now(),
			Data: EventData{
				ToolName: "create_file",
				Input: map[string]interface{}{
					"path": "auth.go",
				},
			},
		},
	}
	state.Decisions = []Decision{
		{
			Title:     "使用 JWT",
			Reasoning: "无状态，性能好",
			Status:    "decided",
			Time:      time.Now(),
		},
	}

	content, err := summarizer.Summarize(state.Events, state)
	if err != nil {
		t.Fatalf("failed to summarize: %v", err)
	}

	if content.Summary == "" {
		t.Error("expected non-empty summary")
	}

	if len(content.Decisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(content.Decisions))
	}
}

func TestLoader_Load(t *testing.T) {
	// Create a test document
	doc := &Document{
		Version: "1.0",
		Session: SessionInfo{
			ID:        "test-session",
			StartedAt: time.Now(),
			HandoffAt: time.Now(),
		},
		CurrentTask: TaskInfo{
			Title:    "测试任务",
			Status:   TaskStatusInProgress,
			Progress: 75,
		},
		Content: Content{
			Summary:   "测试摘要",
			NextSteps: []NextStep{
				{Priority: PriorityHigh, Title: "步骤1"},
				{Priority: PriorityLow, Title: "步骤2"},
			},
		},
	}

	// Save to temp file
	formatter := NewYAMLMarkdownFormatter()
	data, _ := formatter.Format(doc)

	loader := NewLoader()
	parsed, err := loader.LoadFromBytes(data)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if parsed.CurrentTask.Progress != 75 {
		t.Errorf("expected progress 75, got %d", parsed.CurrentTask.Progress)
	}
}

func TestDocument_Methods(t *testing.T) {
	doc := &Document{
		Version: "1.0",
		Session: SessionInfo{
			StartedAt: time.Now().Add(-1 * time.Hour),
			HandoffAt: time.Now(),
		},
		CurrentTask: TaskInfo{
			Status: TaskStatusCompleted,
		},
		Content: Content{
			NextSteps: []NextStep{
				{Priority: PriorityHigh, Title: "高优先级步骤"},
				{Priority: PriorityMedium, Title: "中优先级步骤"},
			},
			OpenIssues: []Issue{
				{Severity: SeverityBlocking, Title: "阻塞问题"},
				{Severity: SeverityInfo, Title: "信息问题"},
			},
		},
	}

	// Test Duration
	duration := doc.Duration()
	if duration < 50*time.Minute || duration > 70*time.Minute {
		t.Errorf("expected duration ~1 hour, got %v", duration)
	}

	// Test IsComplete
	if !doc.IsComplete() {
		t.Error("expected IsComplete() to be true")
	}

	// Test HasBlockingIssues
	if !doc.HasBlockingIssues() {
		t.Error("expected HasBlockingIssues() to be true")
	}

	// Test GetHighPrioritySteps
	highSteps := doc.GetHighPrioritySteps()
	if len(highSteps) != 1 {
		t.Errorf("expected 1 high priority step, got %d", len(highSteps))
	}
}

func TestHandler_Generate(t *testing.T) {
	handler := NewHandler(&HandlerConfig{
		SessionID: "test-session",
	})

	// Record some events
	ctx := context.Background()
	info := &callbacks.RunInfo{
		Name: "test-agent",
		Type: "test",
	}

	handler.OnStart(ctx, info, nil)

	// Mark a decision
	handler.MarkDecision(Decision{
		Title:     "测试决策",
		Reasoning: "测试",
		Status:    "decided",
		Time:      time.Now(),
	})

	// Generate document
	doc, err := handler.Generate(ctx, &GenerateOptions{
		TaskTitle:    "测试任务",
		TaskStatus:   TaskStatusInProgress,
		TaskProgress: 50,
	})
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	if doc.Session.ID != "test-session" {
		t.Errorf("expected session ID test-session, got %s", doc.Session.ID)
	}

	if doc.CurrentTask.Title != "测试任务" {
		t.Errorf("expected task title '测试任务', got %s", doc.CurrentTask.Title)
	}

	if len(doc.Content.Decisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(doc.Content.Decisions))
	}
}

func TestHandlerBuilder(t *testing.T) {
	handler := NewHandlerBuilder().
		WithSessionID("builder-test").
		WithSessionDescription("测试会话").
		WithMaxEvents(100).
		Build()

	if handler.config.SessionID != "builder-test" {
		t.Errorf("expected session ID builder-test, got %s", handler.config.SessionID)
	}

	if handler.config.SessionDescription != "测试会话" {
		t.Errorf("expected description '测试会话', got %s", handler.config.SessionDescription)
	}

	if handler.config.Collector.MaxEvents != 100 {
		t.Errorf("expected max events 100, got %d", handler.config.Collector.MaxEvents)
	}
}

func TestLoader_GetNextSteps(t *testing.T) {
	doc := &Document{
		Content: Content{
			NextSteps: []NextStep{
				{Priority: PriorityHigh, Title: "高1"},
				{Priority: PriorityHigh, Title: "高2"},
				{Priority: PriorityMedium, Title: "中1"},
				{Priority: PriorityLow, Title: "低1"},
			},
		},
	}

	loader := NewLoader()

	highSteps := loader.GetNextSteps(doc, PriorityHigh)
	if len(highSteps) != 2 {
		t.Errorf("expected 2 high priority steps, got %d", len(highSteps))
	}

	mediumSteps := loader.GetNextSteps(doc, PriorityMedium)
	if len(mediumSteps) != 3 { // medium + high
		t.Errorf("expected 3 steps (medium+high), got %d", len(mediumSteps))
	}
}

func TestFormatter_ParseInvalidDocument(t *testing.T) {
	formatter := NewYAMLMarkdownFormatter()

	// Invalid: missing frontmatter start
	_, err := formatter.Parse([]byte("no frontmatter"))
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}

	// Invalid: missing frontmatter end
	_, err = formatter.Parse([]byte("---\ninvalid"))
	if err == nil {
		t.Error("expected error for missing frontmatter end")
	}
}

func TestDocument_BytesAndSave(t *testing.T) {
	doc := &Document{
		Version: "1.0",
		Session: SessionInfo{
			ID:        "byte-test",
			StartedAt: time.Now(),
			HandoffAt: time.Now(),
		},
		CurrentTask: TaskInfo{
			Title: "字节测试",
		},
		Content: Content{
			Summary: "测试字节方法",
		},
	}

	// Test Bytes
	data, err := doc.Bytes()
	if err != nil {
		t.Fatalf("failed to get bytes: %v", err)
	}

	if !bytes.Contains(data, []byte("字节测试")) {
		t.Error("bytes missing task title")
	}

	// Test String
	str, err := doc.String()
	if err != nil {
		t.Fatalf("failed to get string: %v", err)
	}

	if !strings.Contains(str, "测试字节方法") {
		t.Error("string missing summary")
	}
}
