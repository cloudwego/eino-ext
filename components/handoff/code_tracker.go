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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CodeTracker tracks code state from various sources.
type CodeTracker interface {
	GetCodeState() (*CodeState, error)
}

// DefaultCodeTracker provides a filesystem + git-based code tracker.
type DefaultCodeTracker struct {
	// WorkDir is the working directory to track (default: current directory)
	WorkDir string

	// UseGit enables git integration (default: true)
	UseGit bool

	// MaxFiles limits the number of files to track (default: 20)
	MaxFiles int

	// IncludePatterns are glob patterns for files to include
	IncludePatterns []string

	// ExcludePatterns are glob patterns for files to exclude
	ExcludePatterns []string
}

// NewDefaultCodeTracker creates a new DefaultCodeTracker.
func NewDefaultCodeTracker() *DefaultCodeTracker {
	return &DefaultCodeTracker{
		WorkDir:         ".",
		UseGit:          true,
		MaxFiles:        20,
		IncludePatterns: []string{"*.go", "*.js", "*.ts", "*.py", "*.java", "*.md", "*.yaml", "*.yml", "*.json"},
		ExcludePatterns: []string{"node_modules/*", ".git/*", "vendor/*", "*.tmp", "*.log"},
	}
}

// GetCodeState returns the current code state.
func (t *DefaultCodeTracker) GetCodeState() (*CodeState, error) {
	state := &CodeState{
		WorkingFiles:  make([]WorkingFile, 0),
		RecentChanges: make([]FileChange, 0),
	}

	// Get git info if available
	if t.UseGit {
		repoInfo, err := t.getGitInfo()
		if err == nil {
			state.RepositoryInfo = repoInfo
		}
	}

	// Get recent file changes
	changes, err := t.getRecentChanges()
	if err == nil {
		state.RecentChanges = changes

		// Convert changes to working files
		for _, change := range changes {
			state.WorkingFiles = append(state.WorkingFiles, WorkingFile{
				Path:   change.Path,
				Status: change.ChangeType,
			})
		}
	}

	return state, nil
}

// getGitInfo gets repository information from git.
func (t *DefaultCodeTracker) getGitInfo() (*RepositoryInfo, error) {
	// Check if we're in a git repo
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = t.WorkDir
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("not a git repository")
	}

	info := &RepositoryInfo{}

	// Get current branch
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = t.WorkDir
	if branch, err := branchCmd.Output(); err == nil {
		info.Branch = strings.TrimSpace(string(branch))
	}

	// Get current commit
	commitCmd := exec.Command("git", "rev-parse", "HEAD")
	commitCmd.Dir = t.WorkDir
	if commit, err := commitCmd.Output(); err == nil {
		info.Commit = strings.TrimSpace(string(commit))
	}

	// Get uncommitted changes
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = t.WorkDir
	if status, err := statusCmd.Output(); err == nil {
		lines := strings.Split(string(status), "\n")
		for _, line := range lines {
			if len(line) > 3 {
				// Format: XY filename or XY "filename with spaces"
				file := strings.TrimSpace(line[3:])
				if file != "" {
					info.UncommittedChanges = append(info.UncommittedChanges, file)
				}
			}
		}
	}

	return info, nil
}

// getRecentChanges gets recent file changes.
func (t *DefaultCodeTracker) getRecentChanges() ([]FileChange, error) {
	var changes []FileChange

	if t.UseGit {
		// Try to get changes from git first
		gitChanges, err := t.getGitChanges()
		if err == nil && len(gitChanges) > 0 {
			return gitChanges, nil
		}
	}

	// Fallback to filesystem scanning
	return t.getFilesystemChanges()
}

// getGitChanges gets changes from git status.
func (t *DefaultCodeTracker) getGitChanges() ([]FileChange, error) {
	var changes []FileChange

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = t.WorkDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	now := time.Now()

	for _, line := range lines {
		if len(line) < 4 {
			continue
		}

		status := line[:2]
		file := strings.TrimSpace(line[3:])

		changeType := "modified"
		switch {
		case strings.Contains(status, "A"):
			changeType = "added"
		case strings.Contains(status, "D"):
			changeType = "deleted"
		case strings.Contains(status, "R"):
			changeType = "renamed"
		case strings.Contains(status, "M"):
			changeType = "modified"
		}

		changes = append(changes, FileChange{
			Path:       file,
			ChangeType: changeType,
			Time:       now,
		})

		if len(changes) >= t.MaxFiles {
			break
		}
	}

	return changes, nil
}

// getFilesystemChanges gets recent changes by scanning filesystem.
func (t *DefaultCodeTracker) getFilesystemChanges() ([]FileChange, error) {
	var changes []FileChange
	now := time.Now()

	err := filepath.Walk(t.WorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(t.WorkDir, path)
		if err != nil {
			return nil
		}

		// Check exclude patterns
		if t.shouldExclude(relPath) {
			return nil
		}

		// Check include patterns
		if !t.shouldInclude(relPath) {
			return nil
		}

		// Check if file was modified recently (within last hour)
		if now.Sub(info.ModTime()) < time.Hour {
			changes = append(changes, FileChange{
				Path:       relPath,
				ChangeType: "modified",
				Time:       info.ModTime(),
			})
		}

		if len(changes) >= t.MaxFiles {
			return fmt.Errorf("max files reached") // Stop walking
		}

		return nil
	})

	if err != nil && err.Error() != "max files reached" {
		return nil, err
	}

	return changes, nil
}

// shouldExclude checks if a path matches exclude patterns.
func (t *DefaultCodeTracker) shouldExclude(path string) bool {
	for _, pattern := range t.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Also check if any parent directory matches
		parts := strings.Split(path, string(filepath.Separator))
		for i := range parts {
			parentPath := strings.Join(parts[:i+1], string(filepath.Separator))
			if matched, _ := filepath.Match(pattern, parentPath); matched {
				return true
			}
		}
	}
	return false
}

// shouldInclude checks if a path matches include patterns.
func (t *DefaultCodeTracker) shouldInclude(path string) bool {
	if len(t.IncludePatterns) == 0 {
		return true
	}

	for _, pattern := range t.IncludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// GitDiffCodeTracker tracks code state using git diff.
type GitDiffCodeTracker struct {
	WorkDir    string
	BaseCommit string // Compare against this commit (empty = HEAD)
}

// NewGitDiffCodeTracker creates a new GitDiffCodeTracker.
func NewGitDiffCodeTracker(baseCommit string) *GitDiffCodeTracker {
	return &GitDiffCodeTracker{
		WorkDir:    ".",
		BaseCommit: baseCommit,
	}
}

// GetCodeState returns code state based on git diff.
func (t *GitDiffCodeTracker) GetCodeState() (*CodeState, error) {
	state := &CodeState{
		WorkingFiles:  make([]WorkingFile, 0),
		RecentChanges: make([]FileChange, 0),
	}

	// Get diff stats
	base := t.BaseCommit
	if base == "" {
		base = "HEAD"
	}

	cmd := exec.Command("git", "diff", "--stat", base)
	cmd.Dir = t.WorkDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse diff output
	lines := strings.Split(string(output), "\n")
	now := time.Now()

	for _, line := range lines {
		// Parse lines like: "filename | 10 +++++-----"
		parts := strings.Split(line, "|")
		if len(parts) >= 1 {
			file := strings.TrimSpace(parts[0])
			if file != "" && !strings.Contains(file, "files changed") {
				state.RecentChanges = append(state.RecentChanges, FileChange{
					Path:       file,
					ChangeType: "modified",
					Time:       now,
				})

				state.WorkingFiles = append(state.WorkingFiles, WorkingFile{
					Path:   file,
					Status: "modified",
				})
			}
		}
	}

	return state, nil
}

// StaticCodeTracker uses a static list of files.
type StaticCodeTracker struct {
	Files []WorkingFile
}

// GetCodeState returns the static code state.
func (t *StaticCodeTracker) GetCodeState() (*CodeState, error) {
	changes := make([]FileChange, len(t.Files))
	for i, f := range t.Files {
		changes[i] = FileChange{
			Path:       f.Path,
			ChangeType: f.Status,
			Time:       time.Now(),
		}
	}

	return &CodeState{
		WorkingFiles:  t.Files,
		RecentChanges: changes,
	}, nil
}
