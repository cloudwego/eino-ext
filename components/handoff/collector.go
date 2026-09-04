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
	"sync"
	"time"

	"github.com/bytedance/eino/callbacks"
	"github.com/bytedance/eino/components/model"
	"github.com/bytedance/eino/compose"
	"github.com/bytedance/eino/schema"
)

// CollectorConfig configures the event collector.
type CollectorConfig struct {
	// MaxEvents limits the number of events stored (0 = unlimited)
	MaxEvents int

	// Filters filter out unwanted events
	Filters []EventFilter

	// CustomExtractors extract custom data from events
	CustomExtractors []CustomExtractor
}

// EventFilter filters events.
type EventFilter interface {
	Filter(event Event) bool
}

// CustomExtractor extracts custom data from events.
type CustomExtractor interface {
	Extract(event Event) map[string]interface{}
}

// Collector collects events through the callback system.
// It implements callbacks.Handler interface.
type Collector struct {
	config *CollectorConfig
	state  *State
	mu     sync.RWMutex
}

// NewCollector creates a new Collector.
func NewCollector(config *CollectorConfig) *Collector {
	if config == nil {
		config = &CollectorConfig{}
	}
	return &Collector{
		config: config,
		state:  NewState(""),
	}
}

// NewCollectorWithState creates a new Collector with existing state.
func NewCollectorWithState(config *CollectorConfig, state *State) *Collector {
	if config == nil {
		config = &CollectorConfig{}
	}
	return &Collector{
		config: config,
		state:  state,
	}
}

// GetState returns the current state (thread-safe).
func (c *Collector) GetState() *State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SetSessionID sets the session ID.
func (c *Collector) SetSessionID(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.SessionID = sessionID
}

// MarkDecision manually records a decision.
func (c *Collector) MarkDecision(decision Decision) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Decisions = append(c.state.Decisions, decision)
}

// MarkMilestone manually records a completed milestone.
func (c *Collector) MarkMilestone(milestone Milestone) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.CompletedMilestones = append(c.state.CompletedMilestones, milestone)
}

// AddCustomData adds custom data to the state.
func (c *Collector) AddCustomData(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.CustomData == nil {
		c.state.CustomData = make(map[string]interface{})
	}
	c.state.CustomData[key] = value
}

// RecordEvent manually records a custom event.
func (c *Collector) RecordEvent(eventType EventType, data EventData) {
	event := Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}
	c.addEvent(event)
}

// addEvent adds an event to the state (thread-safe).
func (c *Collector) addEvent(event Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Apply filters
	for _, filter := range c.config.Filters {
		if !filter.Filter(event) {
			return
		}
	}

	// Check max events limit
	if c.config.MaxEvents > 0 && len(c.state.Events) >= c.config.MaxEvents {
		// Remove oldest event
		c.state.Events = c.state.Events[1:]
	}

	c.state.Events = append(c.state.Events, event)

	// Extract file changes from tool calls
	if event.Type == EventTypeToolCall {
		c.extractFileChange(event)
	}
}

// extractFileChange extracts file change information from tool events.
func (c *Collector) extractFileChange(event Event) {
	if event.Data.ToolName == "" {
		return
	}

	// Common file operation tool names
	fileTools := []string{"read_file", "edit_file", "write_file", "apply_diff", "create_file"}
	for _, toolName := range fileTools {
		if event.Data.ToolName == toolName {
			if path, ok := event.Data.Input["path"].(string); ok {
				change := FileChange{
					Path:       path,
					ChangeType: "modified",
					Time:       event.Timestamp,
				}
				if event.Data.ToolName == "create_file" {
					change.ChangeType = "added"
				}
				c.state.FilesTouched[path] = change
			}
			break
		}
	}
}

// OnStart implements callbacks.Handler.
func (c *Collector) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	event := c.convertInputToEvent(info, input)
	c.addEvent(event)
	return ctx
}

// OnEnd implements callbacks.Handler.
func (c *Collector) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	event := c.convertOutputToEvent(info, output)
	c.addEvent(event)
	return ctx
}

// OnError implements callbacks.Handler.
func (c *Collector) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	event := Event{
		Type:      EventTypeError,
		Timestamp: time.Now(),
		AgentName: info.Name,
		Data: EventData{
			Error: err.Error(),
		},
	}
	c.addEvent(event)
	return ctx
}

// OnStartWithStreamInput implements callbacks.Handler.
func (c *Collector) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	// For streaming input, we just record that streaming started
	event := Event{
		Type:      EventTypeCustom,
		Timestamp: time.Now(),
		AgentName: info.Name,
		Data: EventData{
			CustomType: "stream_start",
		},
	}
	c.addEvent(event)
	return ctx
}

// OnEndWithStreamOutput implements callbacks.Handler.
func (c *Collector) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	// For streaming output, we just record that streaming ended
	event := Event{
		Type:      EventTypeCustom,
		Timestamp: time.Now(),
		AgentName: info.Name,
		Data: EventData{
			CustomType: "stream_end",
		},
	}
	c.addEvent(event)
	return ctx
}

// convertInputToEvent converts callback input to an event.
func (c *Collector) convertInputToEvent(info *callbacks.RunInfo, input callbacks.CallbackInput) Event {
	event := Event{
		Timestamp: time.Now(),
		AgentName: info.Name,
	}

	// Try to convert to model callback input (chat model)
	if modelInput, ok := input.(*model.CallbackInput); ok {
		event.Type = EventTypeMessage
		if len(modelInput.Messages) > 0 {
			lastMsg := modelInput.Messages[len(modelInput.Messages)-1]
			event.Data = EventData{
				Role:    string(lastMsg.Role),
				Content: lastMsg.Content,
			}
		}
		return event
	}

	// Try to convert to agent callback input
	if agentInput, ok := input.(*schema.AgentInput); ok {
		event.Type = EventTypeMessage
		event.Data = EventData{
			Role:    "user",
			Content: agentInput.Input,
		}
		return event
	}

	// Try to convert to tool input
	if toolInput, ok := input.(*compose.ToolInvokeInput); ok {
		event.Type = EventTypeToolCall
		event.Data = EventData{
			ToolName: toolInput.Name,
			Input:    toolInput.Arguments,
		}
		return event
	}

	// Default: custom event
	event.Type = EventTypeCustom
	event.Data.CustomType = "input"
	return event
}

// convertOutputToEvent converts callback output to an event.
func (c *Collector) convertOutputToEvent(info *callbacks.RunInfo, output callbacks.CallbackOutput) Event {
	event := Event{
		Timestamp: time.Now(),
		AgentName: info.Name,
	}

	// Try to convert to model callback output
	if modelOutput, ok := output.(*model.CallbackOutput); ok {
		event.Type = EventTypeMessage
		if modelOutput.Message != nil {
			event.Data = EventData{
				Role:    string(modelOutput.Message.Role),
				Content: modelOutput.Message.Content,
			}
		}
		return event
	}

	// Try to convert to tool output
	if toolOutput, ok := output.(*compose.ToolInvokeOutput); ok {
		event.Type = EventTypeToolResult
		event.Data = EventData{
			ToolName: toolOutput.Name,
			Output:   toolOutput.Result,
		}
		return event
	}

	// Default: custom event
	event.Type = EventTypeCustom
	event.Data.CustomType = "output"
	return event
}

// Common event filters

// FilterInternalEvents filters out internal/framework events.
type FilterInternalEvents struct{}

func (f FilterInternalEvents) Filter(event Event) bool {
	// Filter out stream_start/stream_end if too noisy
	if event.Type == EventTypeCustom {
		if event.Data.CustomType == "stream_start" || event.Data.CustomType == "stream_end" {
			return false
		}
	}
	return true
}

// FilterByType filters events by type.
type FilterByType struct {
	AllowedTypes []EventType
}

func (f FilterByType) Filter(event Event) bool {
	if len(f.AllowedTypes) == 0 {
		return true
	}
	for _, t := range f.AllowedTypes {
		if event.Type == t {
			return true
		}
	}
	return false
}
