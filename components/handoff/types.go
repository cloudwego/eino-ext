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
	"time"
)

// Document represents a handoff document with both structured metadata
// and human-readable content.
type Document struct {
	// Version is the schema version of the handoff document
	Version string `yaml:"handoff_version"`

	// Session contains session-level metadata
	Session SessionInfo `yaml:"session"`

	// CurrentTask describes the task being handed off
	CurrentTask TaskInfo `yaml:"current_task"`

	// Context provides additional context about the work
	Context ContextInfo `yaml:"context"`

	// Stats contains statistics about the session
	Stats StatsInfo `yaml:"stats"`

	// Content is the human-readable markdown content
	Content Content `yaml:"-"`
}

// SessionInfo contains metadata about the session
type SessionInfo struct {
	ID          string    `yaml:"id"`
	StartedAt   time.Time `yaml:"started_at"`
	HandoffAt   time.Time `yaml:"handoff_at"`
	Description string    `yaml:"description,omitempty"`
}

// TaskStatus represents the current status of a task
type TaskStatus string

const (
	TaskStatusInProgress   TaskStatus = "in_progress"
	TaskStatusBlocked      TaskStatus = "blocked"
	TaskStatusReviewNeeded TaskStatus = "review_needed"
	TaskStatusCompleted    TaskStatus = "completed"
)

// TaskInfo describes the current task being worked on
type TaskInfo struct {
	Title              string        `yaml:"title"`
	Description        string        `yaml:"description,omitempty"`
	Status             TaskStatus    `yaml:"status"`
	Progress           int           `yaml:"progress"` // 0-100
	StartedAt          time.Time     `yaml:"started_at,omitempty"`
	EstimatedRemaining string        `yaml:"estimated_remaining,omitempty"`
}

// Milestone represents a completed sub-task or checkpoint
type Milestone struct {
	Title       string    `yaml:"title"`
	CompletedAt time.Time `yaml:"completed_at"`
	Description string    `yaml:"description,omitempty"`
}

// ContextInfo provides additional context about the work
type ContextInfo struct {
	ParentGoal           string            `yaml:"parent_goal,omitempty"`
	CompletedMilestones  []Milestone       `yaml:"completed_milestones,omitempty"`
	Dependencies         []string          `yaml:"dependencies,omitempty"`
	CustomFields         map[string]string `yaml:"custom_fields,omitempty"`
}

// StatsInfo contains statistics about the session
type StatsInfo struct {
	TotalMessages int `yaml:"total_messages"`
	ToolCalls     int `yaml:"tool_calls"`
	FileChanges   int `yaml:"file_changes"`
	LLMTokensIn   int `yaml:"llm_tokens_in,omitempty"`
	LLMTokensOut  int `yaml:"llm_tokens_out,omitempty"`
}

// Content contains the human-readable sections of the document
type Content struct {
	Summary    string     // Executive summary of current state
	Decisions  []Decision // Key decisions made
	CodeState  CodeState  // Code/file state
	NextSteps  []NextStep // Recommended next steps
	OpenIssues []Issue    // Open questions or blockers
	Resources  []Resource // References and resources
}

// Decision records a key decision made during the session
type Decision struct {
	Time        time.Time `yaml:"time"`
	Title       string    `yaml:"title"`
	Reasoning   string    `yaml:"reasoning"`
	Status      string    `yaml:"status"` // decided, pending, rejected
	Alternatives []string `yaml:"alternatives,omitempty"`
}

// CodeState describes the current state of the codebase
type CodeState struct {
	WorkingFiles   []WorkingFile   `yaml:"working_files,omitempty"`
	RecentChanges  []FileChange    `yaml:"recent_changes,omitempty"`
	RepositoryInfo *RepositoryInfo `yaml:"repository_info,omitempty"`
}

// WorkingFile describes a file currently being worked on
type WorkingFile struct {
	Path       string `yaml:"path"`
	Status     string `yaml:"status"` // editing, created, modified
	LineNumber int    `yaml:"line_number,omitempty"`
	Snippet    string `yaml:"snippet,omitempty"`
}

// FileChange describes a file change
type FileChange struct {
	Path      string    `yaml:"path"`
	ChangeType string   `yaml:"change_type"` // added, modified, deleted
	Time      time.Time `yaml:"time,omitempty"`
}

// RepositoryInfo contains information about the repository state
type RepositoryInfo struct {
	Branch          string            `yaml:"branch,omitempty"`
	Commit          string            `yaml:"commit,omitempty"`
	UncommittedChanges []string       `yaml:"uncommitted_changes,omitempty"`
}

// NextStep represents a recommended next step
type NextStep struct {
	Priority    Priority `yaml:"priority"` // high, medium, low
	Title       string   `yaml:"title"`
	Description string   `yaml:"description,omitempty"`
	EstimatedTime string `yaml:"estimated_time,omitempty"`
}

// Priority represents task priority
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// Issue represents an open question or blocker
type Issue struct {
	ID          string   `yaml:"id,omitempty"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description,omitempty"`
	Severity    Severity `yaml:"severity"` // blocking, warning, info
}

// Severity represents issue severity
type Severity string

const (
	SeverityBlocking Severity = "blocking"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// Resource represents a reference or resource
type Resource struct {
	Type        string `yaml:"type"` // doc, code, link, note
	Title       string `yaml:"title"`
	URL         string `yaml:"url,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// Event represents a captured event from the callback system
// This is used internally by the Collector
type Event struct {
	Type      EventType     `json:"type"`
	Timestamp time.Time     `json:"timestamp"`
	AgentName string        `json:"agent_name,omitempty"`
	Data      EventData     `json:"data"`
}

// EventType represents the type of event
type EventType string

const (
	EventTypeMessage    EventType = "message"
	EventTypeToolCall   EventType = "tool_call"
	EventTypeToolResult EventType = "tool_result"
	EventTypeDecision   EventType = "decision"
	EventTypeFileChange EventType = "file_change"
	EventTypeError      EventType = "error"
	EventTypeCustom     EventType = "custom"
)

// EventData contains event-specific data
type EventData struct {
	// For message events
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`

	// For tool events
	ToolName string                 `json:"tool_name,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Output   interface{}            `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`

	// For decision events
	Decision Decision `json:"decision,omitempty"`

	// For file change events
	FileChange FileChange `json:"file_change,omitempty"`

	// For custom events
	CustomType string                 `json:"custom_type,omitempty"`
	CustomData map[string]interface{} `json:"custom_data,omitempty"`
}

// State represents the accumulated state during a session
// This is used internally by the Collector
type State struct {
	SessionID       string
	StartTime       time.Time
	Events          []Event
	Decisions       []Decision
	FilesTouched    map[string]FileChange
	CustomData      map[string]interface{}
	MilestoneStack  []Milestone
}

// NewState creates a new State instance
func NewState(sessionID string) *State {
	return &State{
		SessionID:    sessionID,
		StartTime:    time.Now(),
		Events:       make([]Event, 0),
		Decisions:    make([]Decision, 0),
		FilesTouched: make(map[string]FileChange),
		CustomData:   make(map[string]interface{}),
	}
}