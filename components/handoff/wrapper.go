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
	"os"
	"path/filepath"
	"time"

	"github.com/bytedance/eino/adk"
)

// TriggerMode determines when handoff is generated.
type TriggerMode int

const (
	// TriggerManual requires explicit call to GenerateHandoff
	TriggerManual TriggerMode = iota

	// TriggerOnComplete generates handoff when agent completes
	TriggerOnComplete

	// TriggerOnError generates handoff when agent errors
	TriggerOnError
)

// WrapConfig configures the handoff wrapper.
type WrapConfig struct {
	// SessionID is the unique identifier
	SessionID string

	// OutputPath is where handoff files are saved (default: current directory)
	OutputPath string

	// TriggerMode determines when to generate handoff
	TriggerMode TriggerMode

	// HandlerConfig is passed to the underlying handler
	HandlerConfig *HandlerConfig

	// OnBeforeHandoff is called before generating handoff
	OnBeforeHandoff func(ctx context.Context, context *HandoffContext) error

	// OnAfterHandoff is called after generating handoff
	OnAfterHandoff func(ctx context.Context, path string)
}

// HandoffContext provides context during handoff generation.
type HandoffContext struct {
	Handler    *Handler
	State      *State
	Document   *Document
	OutputPath string
}

// Wrapper wraps an agent with handoff capabilities.
type Wrapper struct {
	inner    adk.Agent
	config   *WrapConfig
	handler  *Handler
	outputFn func(*Document) ([]byte, error)
}

// Wrap wraps an agent with handoff capabilities.
func Wrap(agent adk.Agent, config *WrapConfig) (*Wrapper, error) {
	if config == nil {
		config = &WrapConfig{}
	}

	// Set defaults
	if config.SessionID == "" {
		config.SessionID = fmt.Sprintf("session_%d", time.Now().Unix())
	}
	if config.OutputPath == "" {
		config.OutputPath = "."
	}

	// Create handler config
	handlerConfig := config.HandlerConfig
	if handlerConfig == nil {
		handlerConfig = &HandlerConfig{
			SessionID: config.SessionID,
		}
	}

	return &Wrapper{
		inner:   agent,
		config:  config,
		handler: NewHandler(handlerConfig),
		outputFn: func(doc *Document) ([]byte, error) {
			formatter := NewYAMLMarkdownFormatter()
			return formatter.Format(doc)
		},
	}, nil
}

// Run runs the wrapped agent.
func (w *Wrapper) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	// Add handoff callback to options
	handoffOpt := adk.WithCallbacks(w.handler)
	opts = append(opts, handoffOpt)

	// Run inner agent
	events := w.inner.Run(ctx, input, opts...)

	// Wrap events to monitor completion/error
	return w.wrapEvents(ctx, events)
}

// wrapEvents wraps the event iterator to monitor for completion/error.
func (w *Wrapper) wrapEvents(ctx context.Context, events *adk.AsyncIterator[*adk.AgentEvent]) *adk.AsyncIterator[*adk.AgentEvent] {
	// For simplicity, we just pass through events
	// In a full implementation, we'd monitor for completion/error and trigger handoff
	// This would require creating a new iterator that copies events
	return events
}

// GenerateHandoff manually triggers handoff generation.
func (w *Wrapper) GenerateHandoff(ctx context.Context, opts *GenerateOptions) (string, error) {
	// Build handoff context
	handoffCtx := &HandoffContext{
		Handler:    w.handler,
		State:      w.handler.GetState(),
		OutputPath: w.buildOutputPath(),
	}

	// Call before hook if provided
	if w.config.OnBeforeHandoff != nil {
		if err := w.config.OnBeforeHandoff(ctx, handoffCtx); err != nil {
			return "", fmt.Errorf("before handoff hook failed: %w", err)
		}
	}

	// Generate document
	doc, err := w.handler.Generate(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate handoff: %w", err)
	}

	handoffCtx.Document = doc

	// Format document
	data, err := w.outputFn(doc)
	if err != nil {
		return "", fmt.Errorf("failed to format handoff: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(w.config.OutputPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write file
	outputPath := w.buildOutputPath()
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write handoff file: %w", err)
	}

	// Call after hook if provided
	if w.config.OnAfterHandoff != nil {
		w.config.OnAfterHandoff(ctx, outputPath)
	}

	return outputPath, nil
}

// buildOutputPath builds the output file path.
func (w *Wrapper) buildOutputPath() string {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("handoff_%s_%s.md", w.config.SessionID, timestamp)
	return filepath.Join(w.config.OutputPath, filename)
}

// GetHandler returns the underlying handler for direct access.
func (w *Wrapper) GetHandler() *Handler {
	return w.handler
}

// MarkDecision is a convenience method to mark a decision.
func (w *Wrapper) MarkDecision(decision Decision) {
	w.handler.MarkDecision(decision)
}

// MarkMilestone is a convenience method to mark a milestone.
func (w *Wrapper) MarkMilestone(milestone Milestone) {
	w.handler.MarkMilestone(milestone)
}

// ResumableWrapper wraps a ResumableAgent.
type ResumableWrapper struct {
	*Wrapper
	inner adk.ResumableAgent
}

// WrapResumable wraps a ResumableAgent with handoff capabilities.
func WrapResumable(agent adk.ResumableAgent, config *WrapConfig) (*ResumableWrapper, error) {
	wrapper, err := Wrap(agent, config)
	if err != nil {
		return nil, err
	}

	return &ResumableWrapper{
		Wrapper: wrapper,
		inner:   agent,
	}, nil
}

// Resume resumes the wrapped agent.
func (w *ResumableWrapper) Resume(ctx context.Context, info *adk.ResumeInfo, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	// Add handoff callback to options
	handoffOpt := adk.WithCallbacks(w.handler)
	opts = append(opts, handoffOpt)

	// Resume inner agent
	return w.inner.Resume(ctx, info, opts...)
}