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
	"fmt"
	"io"
	"os"
)

// Loader loads handoff documents from various sources.
type Loader struct {
	formatter Formatter
}

// NewLoader creates a new Loader with the default formatter.
func NewLoader() *Loader {
	return &Loader{
		formatter: NewYAMLMarkdownFormatter(),
	}
}

// NewLoaderWithFormatter creates a new Loader with a custom formatter.
func NewLoaderWithFormatter(formatter Formatter) *Loader {
	return &Loader{
		formatter: formatter,
	}
}

// Load loads a document from a file path.
func (l *Loader) Load(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	doc, err := l.formatter.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document from %s: %w", path, err)
	}

	return doc, nil
}

// LoadFromReader loads a document from an io.Reader.
func (l *Loader) LoadFromReader(r io.Reader) (*Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read from reader: %w", err)
	}

	doc, err := l.formatter.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	return doc, nil
}

// LoadFromBytes loads a document from a byte slice.
func (l *Loader) LoadFromBytes(data []byte) (*Document, error) {
	doc, err := l.formatter.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	return doc, nil
}

// Validate validates a handoff document without loading it fully.
func (l *Loader) Validate(path string) error {
	doc, err := l.Load(path)
	if err != nil {
		return err
	}

	return l.ValidateDocument(doc)
}

// ValidateDocument validates a loaded document.
func (l *Loader) ValidateDocument(doc *Document) error {
	if doc.Version == "" {
		return fmt.Errorf("missing required field: handoff_version")
	}

	// Check version compatibility
	supportedVersions := []string{"1.0"}
	isSupported := false
	for _, v := range supportedVersions {
		if doc.Version == v {
			isSupported = true
			break
		}
	}
	if !isSupported {
		return fmt.Errorf("unsupported handoff_version: %s (supported: %v)",
			doc.Version, supportedVersions)
	}

	if doc.Session.ID == "" {
		return fmt.Errorf("missing required field: session.id")
	}

	return nil
}

// ExtractSummary extracts a human-readable summary from a document.
func (l *Loader) ExtractSummary(doc *Document) string {
	var summary string

	// Build summary
	if doc.CurrentTask.Title != "" {
		summary += fmt.Sprintf("Task: %s\n", doc.CurrentTask.Title)
	}

	if doc.CurrentTask.Progress > 0 {
		summary += fmt.Sprintf("Progress: %d%%\n", doc.CurrentTask.Progress)
	}

	if doc.CurrentTask.Status != "" {
		summary += fmt.Sprintf("Status: %s\n", doc.CurrentTask.Status)
	}

	if len(doc.Context.CompletedMilestones) > 0 {
		summary += fmt.Sprintf("\nCompleted milestones: %d\n", len(doc.Context.CompletedMilestones))
	}

	if len(doc.Content.Decisions) > 0 {
		summary += fmt.Sprintf("Key decisions: %d\n", len(doc.Content.Decisions))
	}

	if len(doc.Content.NextSteps) > 0 {
		summary += fmt.Sprintf("\nNext steps:\n")
		for i, step := range doc.Content.NextSteps {
			if i >= 5 { // Limit to 5 steps
				summary += "...\n"
				break
			}
			summary += fmt.Sprintf("- [%s] %s\n", step.Priority, step.Title)
		}
	}

	return summary
}

// GetNextSteps returns the next steps from a document, filtered by priority.
func (l *Loader) GetNextSteps(doc *Document, minPriority Priority) []NextStep {
	var filtered []NextStep

	priorityOrder := map[Priority]int{
		PriorityHigh:   3,
		PriorityMedium: 2,
		PriorityLow:    1,
	}

	minLevel := priorityOrder[minPriority]

	for _, step := range doc.Content.NextSteps {
		if priorityOrder[step.Priority] >= minLevel {
			filtered = append(filtered, step)
		}
	}

	return filtered
}

// GetBlockingIssues returns all blocking issues from a document.
func (l *Loader) GetBlockingIssues(doc *Document) []Issue {
	var blocking []Issue

	for _, issue := range doc.Content.OpenIssues {
		if issue.Severity == SeverityBlocking {
			blocking = append(blocking, issue)
		}
	}

	return blocking
}

// MergeDocuments merges multiple handoff documents into one.
// This is useful for combining handoffs from different sessions.
func (l *Loader) MergeDocuments(docs []*Document, newSessionID string) (*Document, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents to merge")
	}

	// Use the first document as base
	merged := &Document{
		Version: "1.0",
		Session: SessionInfo{
			ID:        newSessionID,
			StartedAt: docs[0].Session.StartedAt,
		},
		CurrentTask: docs[len(docs)-1].CurrentTask, // Use latest task info
		Content:     Content{},
	}

	// Merge decisions
	decisionMap := make(map[string]Decision)
	for _, doc := range docs {
		for _, d := range doc.Content.Decisions {
			key := d.Title // Use title as key to deduplicate
			decisionMap[key] = d
		}
	}
	for _, d := range decisionMap {
		merged.Content.Decisions = append(merged.Content.Decisions, d)
	}

	// Merge milestones
	milestoneMap := make(map[string]Milestone)
	for _, doc := range docs {
		for _, m := range doc.Context.CompletedMilestones {
			milestoneMap[m.Title] = m
		}
	}
	for _, m := range milestoneMap {
		merged.Context.CompletedMilestones = append(merged.Context.CompletedMilestones, m)
	}

	// Merge next steps (take from latest)
	merged.Content.NextSteps = docs[len(docs)-1].Content.NextSteps

	// Merge open issues
	issueMap := make(map[string]Issue)
	for _, doc := range docs {
		for _, i := range doc.Content.OpenIssues {
			key := i.Title
			issueMap[key] = i
		}
	}
	for _, i := range issueMap {
		merged.Content.OpenIssues = append(merged.Content.OpenIssues, i)
	}

	// Create summary
	merged.Content.Summary = fmt.Sprintf(
		"Merged handoff from %d sessions. Original sessions: %s",
		len(docs),
		joinSessionIDs(docs),
	)

	return merged, nil
}

// Helper function
func joinSessionIDs(docs []*Document) string {
	ids := make([]string, len(docs))
	for i, doc := range docs {
		ids[i] = doc.Session.ID
	}

	result := ""
	for i, id := range ids {
		if i > 0 {
			result += ", "
		}
		result += id
	}
	return result
}
