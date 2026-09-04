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
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Formatter formats documents to/from bytes.
type Formatter interface {
	Format(doc *Document) ([]byte, error)
	Parse(data []byte) (*Document, error)
}

// YAMLMarkdownFormatter formats documents as YAML frontmatter + Markdown body.
type YAMLMarkdownFormatter struct {
	// Indentation spaces for lists (default: 2)
	ListIndent int
}

// NewYAMLMarkdownFormatter creates a new YAMLMarkdownFormatter.
func NewYAMLMarkdownFormatter() *YAMLMarkdownFormatter {
	return &YAMLMarkdownFormatter{
		ListIndent: 2,
	}
}

// Format formats a document to YAML frontmatter + Markdown.
func (f *YAMLMarkdownFormatter) Format(doc *Document) ([]byte, error) {
	var buf bytes.Buffer

	// 1. YAML Frontmatter
	if err := f.writeFrontmatter(&buf, doc); err != nil {
		return nil, fmt.Errorf("failed to write frontmatter: %w", err)
	}

	// 2. Markdown Body
	if err := f.writeMarkdownBody(&buf, doc); err != nil {
		return nil, fmt.Errorf("failed to write markdown body: %w", err)
	}

	return buf.Bytes(), nil
}

// writeFrontmatter writes the YAML frontmatter section.
func (f *YAMLMarkdownFormatter) writeFrontmatter(buf *bytes.Buffer, doc *Document) error {
	buf.WriteString("---\n")

	// Only include non-empty fields in YAML
	type frontmatterDoc struct {
		Version     string      `yaml:"handoff_version"`
		Session     SessionInfo `yaml:"session,omitempty"`
		CurrentTask TaskInfo    `yaml:"current_task,omitempty"`
		Context     ContextInfo `yaml:"context,omitempty"`
		Stats       StatsInfo   `yaml:"stats,omitempty"`
	}

	fm := frontmatterDoc{
		Version:     doc.Version,
		Session:     doc.Session,
		CurrentTask: doc.CurrentTask,
		Context:     doc.Context,
		Stats:       doc.Stats,
	}

	yamlData, err := yaml.Marshal(fm)
	if err != nil {
		return err
	}

	buf.Write(yamlData)
	buf.WriteString("---\n\n")
	return nil
}

// writeMarkdownBody writes the Markdown body section.
func (f *YAMLMarkdownFormatter) writeMarkdownBody(buf *bytes.Buffer, doc *Document) error {
	content := doc.Content

	// 1. Task Summary
	f.writeSection(buf, "任务摘要", content.Summary)

	// 2. Key Decisions
	f.writeDecisionsSection(buf, content.Decisions)

	// 3. Code State
	f.writeCodeStateSection(buf, content.CodeState)

	// 4. Next Steps
	f.writeNextStepsSection(buf, content.NextSteps)

	// 5. Open Issues
	f.writeIssuesSection(buf, content.OpenIssues)

	// 6. Resources
	f.writeResourcesSection(buf, content.Resources)

	return nil
}

// writeSection writes a section with a header.
func (f *YAMLMarkdownFormatter) writeSection(buf *bytes.Buffer, title, content string) {
	if content == "" {
		return
	}
	buf.WriteString(fmt.Sprintf("# %s\n\n", title))
	buf.WriteString(content)
	buf.WriteString("\n\n")
}

// writeDecisionsSection writes the decisions table.
func (f *YAMLMarkdownFormatter) writeDecisionsSection(buf *bytes.Buffer, decisions []Decision) {
	if len(decisions) == 0 {
		return
	}

	buf.WriteString("## 关键决策\n\n")
	buf.WriteString("| 时间 | 决策 | 理由 | 状态 |\n")
	buf.WriteString("|------|------|------|------|\n")

	for _, d := range decisions {
		timeStr := d.Time.Format("15:04")
		if d.Time.IsZero() {
			timeStr = "-"
		}

		reasoning := truncate(d.Reasoning, 30)
		status := d.Status
		if status == "" {
			status = "decided"
		}

		buf.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			timeStr,
			escapeTableCell(d.Title),
			escapeTableCell(reasoning),
			status,
		))
	}

	buf.WriteString("\n")
}

// writeCodeStateSection writes the code state section.
func (f *YAMLMarkdownFormatter) writeCodeStateSection(buf *bytes.Buffer, codeState CodeState) {
	hasContent := len(codeState.WorkingFiles) > 0 || len(codeState.RecentChanges) > 0
	if !hasContent && codeState.RepositoryInfo == nil {
		return
	}

	buf.WriteString("## 代码状态\n\n")

	// Working files
	if len(codeState.WorkingFiles) > 0 {
		buf.WriteString("### 工作文件\n\n")
		for _, wf := range codeState.WorkingFiles {
			buf.WriteString(fmt.Sprintf("- `%s`", wf.Path))
			if wf.LineNumber > 0 {
				buf.WriteString(fmt.Sprintf(" (第 %d 行)", wf.LineNumber))
			}
			if wf.Status != "" {
				statusEmoji := f.getStatusEmoji(wf.Status)
				buf.WriteString(fmt.Sprintf(" %s", statusEmoji))
			}
			if wf.Snippet != "" {
				buf.WriteString(fmt.Sprintf("\n\n  ```\n  %s\n  ```", indent(wf.Snippet, f.ListIndent)))
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	// Recent changes
	if len(codeState.RecentChanges) > 0 {
		buf.WriteString("### 最近变更\n\n")
		for _, change := range codeState.RecentChanges {
			emoji := f.getChangeTypeEmoji(change.ChangeType)
			buf.WriteString(fmt.Sprintf("- %s `%s`", emoji, change.Path))
			if !change.Time.IsZero() {
				buf.WriteString(fmt.Sprintf(" (%s)", formatTime(change.Time)))
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	// Repository info
	if codeState.RepositoryInfo != nil {
		buf.WriteString("### 仓库信息\n\n")
		if codeState.RepositoryInfo.Branch != "" {
			buf.WriteString(fmt.Sprintf("- **分支**: `%s`\n", codeState.RepositoryInfo.Branch))
		}
		if codeState.RepositoryInfo.Commit != "" {
			commit := codeState.RepositoryInfo.Commit
			if len(commit) > 7 {
				commit = commit[:7]
			}
			buf.WriteString(fmt.Sprintf("- **提交**: `%s`\n", commit))
		}
		if len(codeState.RepositoryInfo.UncommittedChanges) > 0 {
			buf.WriteString(fmt.Sprintf("- **未提交变更**: %d 个文件\n",
				len(codeState.RepositoryInfo.UncommittedChanges)))
		}
		buf.WriteString("\n")
	}
}

// writeNextStepsSection writes the next steps section.
func (f *YAMLMarkdownFormatter) writeNextStepsSection(buf *bytes.Buffer, steps []NextStep) {
	if len(steps) == 0 {
		return
	}

	buf.WriteString("## 下一步\n\n")

	// Group by priority
	priorityOrder := []Priority{PriorityHigh, PriorityMedium, PriorityLow}
	priorityNames := map[Priority]string{
		PriorityHigh:   "高优先级",
		PriorityMedium: "中优先级",
		PriorityLow:    "低优先级",
	}

	priorityGroups := make(map[Priority][]NextStep)
	for _, step := range steps {
		if step.Priority == "" {
			step.Priority = PriorityMedium
		}
		priorityGroups[step.Priority] = append(priorityGroups[step.Priority], step)
	}

	for _, priority := range priorityOrder {
		group := priorityGroups[priority]
		if len(group) == 0 {
			continue
		}

		buf.WriteString(fmt.Sprintf("### %s\n\n", priorityNames[priority]))
		for i, step := range group {
			buf.WriteString(fmt.Sprintf("%d. **%s**", i+1, step.Title))
			if step.EstimatedTime != "" {
				buf.WriteString(fmt.Sprintf(" (预估: %s)", step.EstimatedTime))
			}
			buf.WriteString("\n")
			if step.Description != "" {
				buf.WriteString(fmt.Sprintf("   %s\n", step.Description))
			}
		}
		buf.WriteString("\n")
	}
}

// writeIssuesSection writes the open issues section.
func (f *YAMLMarkdownFormatter) writeIssuesSection(buf *bytes.Buffer, issues []Issue) {
	if len(issues) == 0 {
		return
	}

	buf.WriteString("## 待解决问题\n\n")
	for _, issue := range issues {
		emoji := f.getSeverityEmoji(issue.Severity)
		if issue.ID != "" {
			buf.WriteString(fmt.Sprintf("- %s **%s** (%s): %s\n",
				emoji, issue.Title, issue.ID, issue.Description))
		} else {
			buf.WriteString(fmt.Sprintf("- %s **%s**: %s\n",
				emoji, issue.Title, issue.Description))
		}
	}
	buf.WriteString("\n")
}

// writeResourcesSection writes the resources section.
func (f *YAMLMarkdownFormatter) writeResourcesSection(buf *bytes.Buffer, resources []Resource) {
	if len(resources) == 0 {
		return
	}

	buf.WriteString("## 参考资源\n\n")
	for _, res := range resources {
		if res.URL != "" {
			buf.WriteString(fmt.Sprintf("- **%s**: [%s](%s)\n",
				res.Type, res.Title, res.URL))
		} else {
			buf.WriteString(fmt.Sprintf("- **%s**: %s\n",
				res.Type, res.Title))
		}
		if res.Description != "" {
			buf.WriteString(fmt.Sprintf("  - %s\n", res.Description))
		}
	}
	buf.WriteString("\n")
}

// getStatusEmoji returns an emoji for file status.
func (f *YAMLMarkdownFormatter) getStatusEmoji(status string) string {
	switch status {
	case "editing":
		return "✏️"
	case "modified":
		return "📝"
	case "created", "added":
		return "✨"
	case "deleted":
		return "🗑️"
	case "completed", "done":
		return "✅"
	default:
		return "📄"
	}
}

// getChangeTypeEmoji returns an emoji for change type.
func (f *YAMLMarkdownFormatter) getChangeTypeEmoji(changeType string) string {
	switch changeType {
	case "added", "created":
		return "➕"
	case "modified":
		return "📝"
	case "deleted":
		return "➖"
	case "renamed":
		return "📛"
	default:
		return "📝"
	}
}

// getSeverityEmoji returns an emoji for issue severity.
func (f *YAMLMarkdownFormatter) getSeverityEmoji(severity Severity) string {
	switch severity {
	case SeverityBlocking:
		return "🚫"
	case SeverityWarning:
		return "⚠️"
	case SeverityInfo:
		return "ℹ️"
	default:
		return "•"
	}
}

// Parse parses a handoff document from bytes.
func (f *YAMLMarkdownFormatter) Parse(data []byte) (*Document, error) {
	// Find the frontmatter boundaries
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("invalid handoff format: missing frontmatter start")
	}

	// Find end of frontmatter
	endIdx := strings.Index(content[4:], "\n---")
	if endIdx == -1 {
		return nil, fmt.Errorf("invalid handoff format: missing frontmatter end")
	}
	endIdx += 4 // Account for the initial 4 chars we skipped

	// Extract frontmatter
	frontmatter := content[4:endIdx]

	// Extract markdown body
	body := ""
	if endIdx+4 < len(content) {
		body = strings.TrimSpace(content[endIdx+4:])
	}

	// Parse YAML frontmatter
	type frontmatterDoc struct {
		Version     string      `yaml:"handoff_version"`
		Session     SessionInfo `yaml:"session"`
		CurrentTask TaskInfo    `yaml:"current_task"`
		Context     ContextInfo `yaml:"context"`
		Stats       StatsInfo   `yaml:"stats"`
	}

	var fm frontmatterDoc
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	doc := &Document{
		Version:     fm.Version,
		Session:     fm.Session,
		CurrentTask: fm.CurrentTask,
		Context:     fm.Context,
		Stats:       fm.Stats,
		Content: Content{
			Summary: body, // Store raw body as summary for now
		},
	}

	return doc, nil
}

// Helper functions

func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func indent(s string, spaces int) string {
	indentStr := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indentStr + line
		}
	}
	return strings.Join(lines, "\n")
}

func formatTime(t time.Time) string {
	return t.Format("01-02 15:04")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
