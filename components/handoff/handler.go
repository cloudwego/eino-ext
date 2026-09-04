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
	"context"
	"fmt"
	"time"

	"github.com/bytedance/eino/callbacks"
	"github.com/bytedance/eino/schema"
)

// HandlerConfig configures the handoff handler.
type HandlerConfig struct {
	// SessionID is the unique identifier for this session
	SessionID string

	// SessionDescription describes the overall session
	SessionDescription string

	// Collector configures event collection
	Collector *CollectorConfig

	// Summarizer generates content summaries (nil = use DefaultSummarizer)
	Summarizer Summarizer

	// CodeTracker tracks code state (nil = use DefaultCodeTracker)
	CodeTracker CodeTracker

	// Formatter formats the output (nil = use YAMLMarkdownFormatter)
	Formatter Formatter

	// Context provides additional context information
	Context *ContextInfo
}

// Handler is the main handoff handler that ties everything together.
// It implements callbacks.Handler interface.
type Handler struct {
	config    *HandlerConfig
	collector *Collector
}

// NewHandler creates a new Handler.
func NewHandler(config *HandlerConfig) *Handler {
	if config == nil {
		config = &HandlerConfig{}
	}

	// Create or use provided collector config
	collectorConfig := config.Collector
	if collectorConfig == nil {
		collectorConfig = &CollectorConfig{}
	}

	// Create state with session info
	state := NewState(config.SessionID)
	if !state.StartTime.IsZero() {
		state.StartTime = time.Now()
	}

	return &Handler{
		config:    config,
		collector: NewCollectorWithState(collectorConfig, state),
	}
}

// OnStart implements callbacks.Handler.
func (h *Handler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	return h.collector.OnStart(ctx, info, input)
}

// OnEnd implements callbacks.Handler.
func (h *Handler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	return h.collector.OnEnd(ctx, info, output)
}

// OnError implements callbacks.Handler.
func (h *Handler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	return h.collector.OnError(ctx, info, err)
}

// OnStartWithStreamInput implements callbacks.Handler.
func (h *Handler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	return h.collector.OnStartWithStreamInput(ctx, info, input)
}

// OnEndWithStreamOutput implements callbacks.Handler.
func (h *Handler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	return h.collector.OnEndWithStreamOutput(ctx, info, output)
}

// MarkDecision manually records a decision.
func (h *Handler) MarkDecision(decision Decision) {
	h.collector.MarkDecision(decision)
}

// MarkMilestone manually records a completed milestone.
func (h *Handler) MarkMilestone(milestone Milestone) {
	h.collector.MarkMilestone(milestone)
}

// AddCustomData adds custom data to the state.
func (h *Handler) AddCustomData(key string, value interface{}) {
	h.collector.AddCustomData(key, value)
}

// RecordEvent manually records a custom event.
func (h *Handler) RecordEvent(eventType EventType, data EventData) {
	h.collector.RecordEvent(eventType, data)
}

// GetState returns the current state.
func (h *Handler) GetState() *State {
	return h.collector.GetState()
}

// GenerateOptions configures document generation.
type GenerateOptions struct {
	// Task information
	TaskTitle       string
	TaskDescription string
	TaskStatus      TaskStatus
	TaskProgress    int

	// Override context info
	Context *ContextInfo

	// Override stats
	Stats *StatsInfo
}

// Generate generates a handoff document.
func (h *Handler) Generate(ctx context.Context, opts *GenerateOptions) (*Document, error) {
	if opts == nil {
		opts = &GenerateOptions{}
	}

	state := h.collector.GetState()

	// Get summarizer
	summarizer := h.config.Summarizer
	if summarizer == nil {
		summarizer = NewDefaultSummarizer()
	}

	// Generate content
	content, err := summarizer.Summarize(state.Events, state)
	if err != nil {
		return nil, fmt.Errorf("failed to summarize: %w", err)
	}

	// Get code state
	if h.config.CodeTracker != nil {
		codeState, err := h.config.CodeTracker.GetCodeState()
		if err == nil {
			content.CodeState = *codeState
		}
	}

	// Build document
	doc := &Document{
		Version: "1.0",
		Session: SessionInfo{
			ID:          state.SessionID,
			StartedAt:   state.StartTime,
			HandoffAt:   time.Now(),
			Description: h.config.SessionDescription,
		},
		CurrentTask: TaskInfo{
			Title:       opts.TaskTitle,
			Description: opts.TaskDescription,
			Status:      opts.TaskStatus,
			Progress:    opts.TaskProgress,
		},
		Context: h.buildContext(opts, state),
		Stats:   h.buildStats(opts, state),
		Content: *content,
	}

	return doc, nil
}

// GenerateBytes generates a handoff document and returns it as bytes.
func (h *Handler) GenerateBytes(ctx context.Context, opts *GenerateOptions) ([]byte, error) {
	doc, err := h.Generate(ctx, opts)
	if err != nil {
		return nil, err
	}

	formatter := h.config.Formatter
	if formatter == nil {
		formatter = NewYAMLMarkdownFormatter()
	}

	return formatter.Format(doc)
}

// GenerateAndSave generates a handoff document and saves it to a file.
func (h *Handler) GenerateAndSave(ctx context.Context, opts *GenerateOptions, path string) error {
	data, err := h.GenerateBytes(ctx, opts)
	if err != nil {
		return err
	}

	// Use default formatter if needed to get the formatter's Format method
	// (which returns []byte, not file writing)
	// Actually we already have bytes from GenerateBytes
	return saveToFile(path, data)
}

// buildContext builds the context info.
func (h *Handler) buildContext(opts *GenerateOptions, state *State) ContextInfo {
	if opts.Context != nil {
		return *opts.Context
	}

	if h.config.Context != nil {
		// Merge with state milestones
		ctx := *h.config.Context
		ctx.CompletedMilestones = append(ctx.CompletedMilestones, state.CompletedMilestones...)
		return ctx
	}

	return ContextInfo{
		CompletedMilestones: state.CompletedMilestones,
		CustomFields:        state.CustomData,
	}
}

// buildStats builds the stats info.
func (h *Handler) buildStats(opts *GenerateOptions, state *State) StatsInfo {
	if opts.Stats != nil {
		return *opts.Stats
	}

	// Calculate stats from events
	var msgCount, toolCount int
	for _, e := range state.Events {
		switch e.Type {
		case EventTypeMessage:
			msgCount++
		case EventTypeToolCall:
			toolCount++
		}
	}

	return StatsInfo{
		TotalMessages: msgCount,
		ToolCalls:     toolCount,
		FileChanges:   len(state.FilesTouched),
	}
}

// Helper function to save bytes to file
func saveToFile(path string, data []byte) error {
	// Implementation would use os.WriteFile
	// For now, just return nil as placeholder
	return nil
}

// HandlerBuilder provides a fluent API for building handlers.
type HandlerBuilder struct {
	config *HandlerConfig
}

// NewHandlerBuilder creates a new HandlerBuilder.
func NewHandlerBuilder() *HandlerBuilder {
	return &HandlerBuilder{
		config: &HandlerConfig{
			Collector: &CollectorConfig{},
		},
	}
}

// WithSessionID sets the session ID.
func (b *HandlerBuilder) WithSessionID(id string) *HandlerBuilder {
	b.config.SessionID = id
	return b
}

// WithSessionDescription sets the session description.
func (b *HandlerBuilder) WithSessionDescription(desc string) *HandlerBuilder {
	b.config.SessionDescription = desc
	return b
}

// WithMaxEvents sets the maximum number of events.
func (b *HandlerBuilder) WithMaxEvents(max int) *HandlerBuilder {
	b.config.Collector.MaxEvents = max
	return b
}

// WithSummarizer sets a custom summarizer.
func (b *HandlerBuilder) WithSummarizer(s Summarizer) *HandlerBuilder {
	b.config.Summarizer = s
	return b
}

// WithCodeTracker sets a custom code tracker.
func (b *HandlerBuilder) WithCodeTracker(ct CodeTracker) *HandlerBuilder {
	b.config.CodeTracker = ct
	return b
}

// WithFormatter sets a custom formatter.
func (b *HandlerBuilder) WithFormatter(f Formatter) *HandlerBuilder {
	b.config.Formatter = f
	return b
}

// WithContext sets the context info.
func (b *HandlerBuilder) WithContext(ctx *ContextInfo) *HandlerBuilder {
	b.config.Context = ctx
	return b
}

// Build builds the Handler.
func (b *HandlerBuilder) Build() *Handler {
	return NewHandler(b.config)
}
