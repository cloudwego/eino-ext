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
	"os"
	"time"
)

// Bytes returns the document as formatted bytes.
func (d *Document) Bytes() ([]byte, error) {
	formatter := NewYAMLMarkdownFormatter()
	return formatter.Format(d)
}

// String returns the document as a string.
func (d *Document) String() (string, error) {
	data, err := d.Bytes()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Save saves the document to a file.
func (d *Document) Save(path string) error {
	data, err := d.Bytes()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Duration returns the session duration.
func (d *Document) Duration() time.Duration {
	return d.Session.HandoffAt.Sub(d.Session.StartedAt)
}

// IsComplete returns true if the task is completed.
func (d *Document) IsComplete() bool {
	return d.CurrentTask.Status == TaskStatusCompleted
}

// IsBlocked returns true if the task is blocked.
func (d *Document) IsBlocked() bool {
	return d.CurrentTask.Status == TaskStatusBlocked
}

// HasBlockingIssues returns true if there are blocking issues.
func (d *Document) HasBlockingIssues() bool {
	for _, issue := range d.Content.OpenIssues {
		if issue.Severity == SeverityBlocking {
			return true
		}
	}
	return false
}

// GetHighPrioritySteps returns high priority next steps.
func (d *Document) GetHighPrioritySteps() []NextStep {
	var steps []NextStep
	for _, step := range d.Content.NextSteps {
		if step.Priority == PriorityHigh {
			steps = append(steps, step)
		}
	}
	return steps
}

// GetFilesByStatus returns files filtered by status.
func (d *Document) GetFilesByStatus(status string) []WorkingFile {
	var files []WorkingFile
	for _, f := range d.Content.CodeState.WorkingFiles {
		if f.Status == status {
			files = append(files, f)
		}
	}
	return files
}

// AddDecision adds a decision to the document.
func (d *Document) AddDecision(decision Decision) {
	d.Content.Decisions = append(d.Content.Decisions, decision)
}

// AddNextStep adds a next step to the document.
func (d *Document) AddNextStep(step NextStep) {
	d.Content.NextSteps = append(d.Content.NextSteps, step)
}

// AddIssue adds an issue to the document.
func (d *Document) AddIssue(issue Issue) {
	d.Content.OpenIssues = append(d.Content.OpenIssues, issue)
}

// UpdateTask updates the current task information.
func (d *Document) UpdateTask(title string, status TaskStatus, progress int) {
	if title != "" {
		d.CurrentTask.Title = title
	}
	if status != "" {
		d.CurrentTask.Status = status
	}
	if progress >= 0 && progress <= 100 {
		d.CurrentTask.Progress = progress
	}
	d.Session.HandoffAt = time.Now()
}
