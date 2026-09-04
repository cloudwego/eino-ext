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
	"strings"
	"time"
)

// Summarizer generates intelligent summaries from collected events.
type Summarizer interface {
	Summarize(events []Event, state *State) (*Content, error)
}

// DefaultSummarizer provides a rule-based summarizer with optional LLM refinement.
type DefaultSummarizer struct {
	// LLM is an optional language model client for refinement
	LLM LLMClient

	// UseLLM enables LLM-based refinement
	UseLLM bool

	// MaxSummaryLength limits the summary length
	MaxSummaryLength int
}

// LLMClient is an interface for LLM-based refinement.
type LLMClient interface {
	// Generate generates text based on a prompt
	Generate(prompt string) (string, error)
}

// NewDefaultSummarizer creates a new DefaultSummarizer.
func NewDefaultSummarizer() *DefaultSummarizer {
	return &DefaultSummarizer{
		MaxSummaryLength: 500,
	}
}

// Summarize generates a summary from events and state.
func (s *DefaultSummarizer) Summarize(events []Event, state *State) (*Content, error) {
	content := &Content{
		Decisions:  state.Decisions,
		CodeState:  s.extractCodeState(state),
		OpenIssues: s.extractOpenIssues(events),
		Resources:  s.extractResources(events),
	}

	// Generate rule-based summary
	content.Summary = s.generateSummary(events, state)

	// Extract next steps
	content.NextSteps = s.extractNextSteps(events)

	// Optional: refine with LLM
	if s.UseLLM && s.LLM != nil {
		refined, err := s.refineWithLLM(content, events)
		if err == nil && refined != nil {
			content = refined
		}
	}

	return content, nil
}

// generateSummary creates a human-readable summary.
func (s *DefaultSummarizer) generateSummary(events []Event, state *State) string {
	if len(events) == 0 {
		return "No activity recorded."
	}

	var parts []string

	// Calculate duration
	startTime := state.StartTime
	if startTime.IsZero() && len(events) > 0 {
		startTime = events[0].Timestamp
	}
	endTime := events[len(events)-1].Timestamp
	duration := endTime.Sub(startTime)

	// Count different event types
	var msgCount, toolCount, decisionCount int
	for _, e := range events {
		switch e.Type {
		case EventTypeMessage:
			msgCount++
		case EventTypeToolCall:
			toolCount++
		case EventTypeDecision:
			decisionCount++
		}
	}

	// Build summary
	parts = append(parts, fmt.Sprintf("Session duration: %s", formatDuration(duration)))
	parts = append(parts, fmt.Sprintf("Messages: %d, Tool calls: %d", msgCount, toolCount))

	if len(state.Decisions) > 0 {
		parts = append(parts, fmt.Sprintf("Key decisions made: %d", len(state.Decisions)))
	}

	if len(state.FilesTouched) > 0 {
		parts = append(parts, fmt.Sprintf("Files modified: %d", len(state.FilesTouched)))
	}

	// Extract current focus from recent events
	recentFocus := s.extractRecentFocus(events)
	if recentFocus != "" {
		parts = append(parts, fmt.Sprintf("\nCurrent focus: %s", recentFocus))
	}

	summary := strings.Join(parts, "\n")
	if len(summary) > s.MaxSummaryLength {
		summary = summary[:s.MaxSummaryLength] + "..."
	}

	return summary
}

// extractCodeState extracts code state from events and state.
func (s *DefaultSummarizer) extractCodeState(state *State) CodeState {
	codeState := CodeState{
		WorkingFiles:  make([]WorkingFile, 0),
		RecentChanges: make([]FileChange, 0),
	}

	// Convert files touched to working files
	for path, change := range state.FilesTouched {
		wf := WorkingFile{
			Path:   path,
			Status: change.ChangeType,
		}
		codeState.WorkingFiles = append(codeState.WorkingFiles, wf)
	}

	// Convert map values to slice for recent changes
	for _, change := range state.FilesTouched {
		codeState.RecentChanges = append(codeState.RecentChanges, change)
	}

	return codeState
}

// extractNextSteps extracts potential next steps from recent events.
func (s *DefaultSummarizer) extractNextSteps(events []Event) []NextStep {
	if len(events) == 0 {
		return nil
	}

	var nextSteps []NextStep

	// Look at the last few messages for clues
	startIdx := len(events) - 5
	if startIdx < 0 {
		startIdx = 0
	}

	recentEvents := events[startIdx:]

	// Check for incomplete tasks or TODOs
	for _, event := range recentEvents {
		if event.Type == EventTypeMessage && event.Data.Content != "" {
			content := strings.ToLower(event.Data.Content)

			// Look for TODO patterns
			if strings.Contains(content, "todo") || strings.Contains(content, "fixme") {
				nextSteps = append(nextSteps, NextStep{
					Priority:    PriorityHigh,
					Title:       extractTodoItem(event.Data.Content),
					Description: "Identified from recent discussion",
				})
			}

			// Look for "next" or "then" patterns
			if strings.Contains(content, "next we need") || strings.Contains(content, "then we should") {
				nextSteps = append(nextSteps, NextStep{
					Priority:    PriorityMedium,
					Title:       "Continue with planned steps",
					Description: "Follow-up from conversation context",
				})
			}
		}
	}

	// If we have files being worked on, suggest completing them
	for _, event := range recentEvents {
		if event.Type == EventTypeToolCall {
			if path, ok := event.Data.Input["path"].(string); ok {
				// Check if this is a recent file operation
				exists := false
				for _, step := range nextSteps {
					if strings.Contains(step.Title, path) {
						exists = true
						break
					}
				}
				if !exists {
					nextSteps = append(nextSteps, NextStep{
						Priority:    PriorityMedium,
						Title:       fmt.Sprintf("Complete work on %s", path),
						Description: "File was recently modified",
					})
				}
			}
		}
	}

	return nextSteps
}

// extractOpenIssues extracts open issues or questions from events.
func (s *DefaultSummarizer) extractOpenIssues(events []Event) []Issue {
	var issues []Issue

	for _, event := range events {
		if event.Type == EventTypeMessage && event.Data.Content != "" {
			content := event.Data.Content

			// Look for question patterns
			if strings.Contains(content, "?") {
				// Simple heuristic: if message ends with ?, it's a question
				trimmed := strings.TrimSpace(content)
				if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '?' {
					issues = append(issues, Issue{
						ID:          fmt.Sprintf("Q%d", len(issues)+1),
						Title:       truncate(trimmed, 80),
						Description: "Open question from discussion",
						Severity:    SeverityInfo,
					})
				}
			}

			// Look for "not sure", "uncertain", "need to decide" patterns
			lowerContent := strings.ToLower(content)
			uncertaintyPatterns := []string{"not sure", "uncertain", "need to decide", "tbd", "to be determined"}
			for _, pattern := range uncertaintyPatterns {
				if strings.Contains(lowerContent, pattern) {
					issues = append(issues, Issue{
						ID:          fmt.Sprintf("U%d", len(issues)+1),
						Title:       "Decision needed",
						Description: truncate(content, 120),
						Severity:    SeverityWarning,
					})
					break
				}
			}
		}
	}

	return issues
}

// extractResources extracts references and resources from events.
func (s *DefaultSummarizer) extractResources(events []Event) []Resource {
	var resources []Resource

	for _, event := range events {
		if event.Type == EventTypeMessage && event.Data.Content != "" {
			content := event.Data.Content

			// Extract URLs
			urls := extractURLs(content)
			for _, url := range urls {
				resources = append(resources, Resource{
					Type: "link",
					Title: truncate(url, 60),
					URL:   url,
				})
			}

			// Extract file references
			files := extractFileReferences(content)
			for _, file := range files {
				exists := false
				for _, r := range resources {
					if r.Type == "code" && r.Title == file {
						exists = true
						break
					}
				}
				if !exists {
					resources = append(resources, Resource{
						Type: "code",
						Title: file,
						Description: "Referenced in discussion",
					})
				}
			}
		}
	}

	return resources
}

// extractRecentFocus extracts the current focus from recent events.
func (s *DefaultSummarizer) extractRecentFocus(events []Event) string {
	if len(events) == 0 {
		return ""
	}

	// Look at last 3 events
	startIdx := len(events) - 3
	if startIdx < 0 {
		startIdx = 0
	}

	var focuses []string
	for _, event := range events[startIdx:] {
		if event.Type == EventTypeToolCall && event.Data.ToolName != "" {
			focuses = append(focuses, fmt.Sprintf("working with %s", event.Data.ToolName))
		}
		if event.Type == EventTypeMessage && event.Data.Content != "" {
			// Try to extract key phrases
			content := event.Data.Content
			if len(content) > 20 {
				focuses = append(focuses, truncate(content, 60))
			}
		}
	}

	if len(focuses) > 0 {
		return focuses[len(focuses)-1]
	}
	return ""
}

// refineWithLLM uses an LLM to refine the content.
func (s *DefaultSummarizer) refineWithLLM(content *Content, events []Event) (*Content, error) {
	// Build prompt
	prompt := s.buildRefinementPrompt(content, events)

	// Generate refined content
	refined, err := s.LLM.Generate(prompt)
	if err != nil {
		return nil, err
	}

	// Parse refined content and update
	content.Summary = refined
	return content, nil
}

// buildRefinementPrompt creates a prompt for LLM refinement.
func (s *DefaultSummarizer) buildRefinementPrompt(content *Content, events []Event) string {
	var sb strings.Builder

	sb.WriteString("You are helping to summarize a coding session. Based on the following information, write a clear, concise summary (2-3 sentences) of what was accomplished and what the current state is.\n\n")

	sb.WriteString("Summary so far:\n")
	sb.WriteString(content.Summary)
	sb.WriteString("\n\n")

	if len(content.Decisions) > 0 {
		sb.WriteString("Key decisions:\n")
		for _, d := range content.Decisions {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", d.Title, d.Reasoning))
		}
		sb.WriteString("\n")
	}

	if len(content.CodeState.WorkingFiles) > 0 {
		sb.WriteString("Files being worked on:\n")
		for _, f := range content.CodeState.WorkingFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", f.Path))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Provide a refined summary:\n")

	return sb.String()
}

// Helper functions

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func extractTodoItem(content string) string {
	// Simple extraction - look for text after TODO or FIXME
	lower := strings.ToLower(content)
	idx := strings.Index(lower, "todo")
	if idx == -1 {
		idx = strings.Index(lower, "fixme")
	}
	if idx != -1 {
		start := idx + 4
		if start < len(content) {
			return truncate(strings.TrimSpace(content[start:]), 60)
		}
	}
	return "Review TODO item"
}

func extractURLs(content string) []string {
	// Simple URL extraction - look for http:// or https://
	var urls []string
	words := strings.Fields(content)
	for _, word := range words {
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			// Clean up trailing punctuation
			word = strings.TrimRight(word, ".,;:!?")
			urls = append(urls, word)
		}
	}
	return urls
}

func extractFileReferences(content string) []string {
	// Simple file reference extraction - look for patterns like `filename.go` or "filename.js"
	var files []string
	// This is a simple heuristic - in production might want more sophisticated parsing
	return files
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
