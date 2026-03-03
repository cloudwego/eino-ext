/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package commandline

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockFileOperator mocks the Operator interface
type MockFileOperator struct {
	mock.Mock
	files      map[string]string
	dirs       map[string]bool
	cmdOutputs []CmdOutputMap
}
type CmdOutputMap struct {
	Cmd    []string
	Output string
}

func NewMockFileOperator() *MockFileOperator {
	return &MockFileOperator{
		files:      make(map[string]string),
		dirs:       make(map[string]bool),
		cmdOutputs: make([]CmdOutputMap, 0),
	}
}

func (m *MockFileOperator) ReadFile(ctx context.Context, path string) (string, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return "", args.Error(1)
	}
	return args.String(0), args.Error(1)
}

func (m *MockFileOperator) WriteFile(ctx context.Context, path string, content string) error {
	args := m.Called(ctx, path, content)
	return args.Error(0)
}

func (m *MockFileOperator) IsDirectory(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func (m *MockFileOperator) Exists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}
func (m *MockFileOperator) RunCommand(ctx context.Context, command []string) (*CommandOutput, error) {
	args := m.Called(ctx, command)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return &CommandOutput{Stdout: args.String(0)}, args.Error(1)
}

// Helper function to set up mock file system
func (m *MockFileOperator) SetupFiles(files map[string]string, dirs map[string]bool) {
	m.files = files
	m.dirs = dirs
}

// Helper function to set up command outputs
func (m *MockFileOperator) SetupCommandOutputs(outputs []CmdOutputMap) {
	m.cmdOutputs = outputs
}

// Helper function to set up mock expectations
func (m *MockFileOperator) SetupExpectations() {
	// Setup Exists expectations
	for path := range m.files {
		m.On("Exists", mock.Anything, path).Return(true, nil)
		m.On("IsDirectory", mock.Anything, path).Return(false, nil)
		m.On("ReadFile", mock.Anything, path).Return(m.files[path], nil)
	}

	for path, isDir := range m.dirs {
		m.On("Exists", mock.Anything, path).Return(true, nil)
		m.On("IsDirectory", mock.Anything, path).Return(isDir, nil)
	}

	// Setup RunCommand expectations
	for _, cmdOutput := range m.cmdOutputs {
		m.On("RunCommand", mock.Anything, cmdOutput.Cmd).Return(cmdOutput.Output, nil)
	}

	// Setup WriteFile expectations
	m.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
}

// TestNewStrReplaceEditor tests the creation of a new editor
func TestNewStrReplaceEditor(t *testing.T) {
	mockOperator := NewMockFileOperator()
	ctx := context.Background()

	editor, err := NewStrReplaceEditor(ctx, &EditorConfig{Operator: mockOperator})

	assert.NoError(t, err)
	assert.NotNil(t, editor)
	assert.Equal(t, "str_replace_editor", editor.info.Name)
	assert.NotEmpty(t, editor.info.Desc)
}

// TestStrReplaceEditor_ValidatePath tests path validation
func TestStrReplaceEditor_ValidatePath(t *testing.T) {
	mockOperator := NewMockFileOperator()
	ctx := context.Background()

	// Setup mock
	mockOperator.On("Exists", mock.Anything, "/valid/file.txt").Return(true, nil)
	mockOperator.On("IsDirectory", mock.Anything, "/valid/file.txt").Return(false, nil)

	mockOperator.On("Exists", mock.Anything, "/valid/dir").Return(true, nil)
	mockOperator.On("IsDirectory", mock.Anything, "/valid/dir").Return(true, nil)

	mockOperator.On("Exists", mock.Anything, "/nonexistent/file.txt").Return(false, nil)

	mockOperator.On("Exists", mock.Anything, "/new/file.txt").Return(false, nil)

	// Create editor
	editor, _ := NewStrReplaceEditor(ctx, &EditorConfig{Operator: mockOperator})

	// Test valid file path
	err := editor.validatePath(ctx, ViewCommand, "/valid/file.txt")
	assert.NoError(t, err)

	// Test valid directory path with view command
	err = editor.validatePath(ctx, ViewCommand, "/valid/dir")
	assert.NoError(t, err)

	// Test valid directory path with non-view command
	err = editor.validatePath(ctx, StrReplaceCommand, "/valid/dir")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")

	// Test non-existent path
	err = editor.validatePath(ctx, ViewCommand, "/nonexistent/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	// Test relative path
	err = editor.validatePath(ctx, ViewCommand, "relative/path.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an absolute path")

	// Test create command with new file
	err = editor.validatePath(ctx, CreateCommand, "/new/file.txt")
	assert.NoError(t, err)

	// Test create command with existing file
	err = editor.validatePath(ctx, CreateCommand, "/valid/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file already exists")
}

// TestStrReplaceEditor_View tests the view command
func TestStrReplaceEditor_View(t *testing.T) {
	mockOperator := NewMockFileOperator()
	ctx := context.Background()

	// Setup mock file system
	mockOperator.SetupFiles(map[string]string{
		"/test/file.txt": "line 1\nline 2\nline 3\nline 4\nline 5",
	}, map[string]bool{
		"/test/dir": true,
	})

	// Setup command outputs
	mockOperator.SetupCommandOutputs([]CmdOutputMap{
		{
			Cmd:    []string{"find", "/test/dir", "-maxdepth", "2", "-not", "-path", "*/\\.*"},
			Output: "/test/dir\n/test/dir/file1.txt\n/test/dir/file2.txt",
		},
	})

	mockOperator.SetupExpectations()

	// Create editor
	editor, _ := NewStrReplaceEditor(ctx, &EditorConfig{Operator: mockOperator})

	// Test view file
	result, err := editor.Execute(ctx, &StrReplaceEditorParams{
		Command: ViewCommand,
		Path:    "/test/file.txt",
	})

	assert.NoError(t, err)
	assert.Contains(t, result, "line 1")
	assert.Contains(t, result, "line 5")

	// Test view file with range
	result, err = editor.Execute(ctx, &StrReplaceEditorParams{
		Command:   ViewCommand,
		Path:      "/test/file.txt",
		ViewRange: []int{2, 4},
	})

	assert.NoError(t, err)
	assert.Contains(t, result, "line 2")
	assert.Contains(t, result, "line 3")
	assert.NotContains(t, result, "line 1")
	assert.NotContains(t, result, "line 5")

	// Test view directory
	result, err = editor.Execute(ctx, &StrReplaceEditorParams{
		Command: ViewCommand,
		Path:    "/test/dir",
	})

	assert.NoError(t, err)
	assert.Contains(t, result, "/test/dir/file1.txt")
	assert.Contains(t, result, "/test/dir/file2.txt")

	// Test view directory with range (should fail)
	_, err = editor.Execute(ctx, &StrReplaceEditorParams{
		Command:   ViewCommand,
		Path:      "/test/dir",
		ViewRange: []int{1, 2},
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "view_range")
}

// TestStrReplaceEditor_Create tests the create command
func TestStrReplaceEditor_Create(t *testing.T) {
	mockOperator := NewMockFileOperator()
	ctx := context.Background()

	// Setup mock
	mockOperator.On("Exists", mock.Anything, "/test/new_file.txt").Return(false, nil)
	mockOperator.On("WriteFile", mock.Anything, "/test/new_file.txt", "test content").Return(nil)

	// Create editor
	editor, _ := NewStrReplaceEditor(ctx, &EditorConfig{Operator: mockOperator})

	// Test create file
	content := "test content"
	result, err := editor.Execute(ctx, &StrReplaceEditorParams{
		Command:  CreateCommand,
		Path:     "/test/new_file.txt",
		FileText: &content,
	})

	assert.NoError(t, err)
	assert.Contains(t, result, "successfully created")
}

// TestStrReplaceEditor_StrReplace tests the str_replace command
func TestStrReplaceEditor_StrReplace(t *testing.T) {
	mockOperator := NewMockFileOperator()
	ctx := context.Background()

	fileContent := "line 1\nline 2\nline 3\nline 4\nline 5"

	// Setup mock
	mockOperator.On("Exists", mock.Anything, "/test/file.txt").Return(true, nil)
	mockOperator.On("IsDirectory", mock.Anything, "/test/file.txt").Return(false, nil)
	mockOperator.On("ReadFile", mock.Anything, "/test/file.txt").Return(fileContent, nil)
	mockOperator.On("WriteFile", mock.Anything, "/test/file.txt", mock.Anything).Return(nil)

	// Create editor
	editor, _ := NewStrReplaceEditor(ctx, &EditorConfig{Operator: mockOperator})

	// Test str_replace
	oldStr := "line 2\nline 3"
	newStr := "replaced line 2\nreplaced line 3"
	result, err := editor.Execute(ctx, &StrReplaceEditorParams{
		Command: StrReplaceCommand,
		Path:    "/test/file.txt",
		OldStr:  &oldStr,
		NewStr:  &newStr,
	})

	assert.NoError(t, err)
	assert.Contains(t, result, "has been edited")
	assert.Contains(t, result, "replaced line 2")

	// Test str_replace without oldStr
	_, err = editor.Execute(ctx, &StrReplaceEditorParams{
		Command: StrReplaceCommand,
		Path:    "/test/file.txt",
		NewStr:  &newStr,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "old_str")

	// Test str_replace with non-existent oldStr
	nonExistentStr := "non-existent"
	_, err = editor.Execute(ctx, &StrReplaceEditorParams{
		Command: StrReplaceCommand,
		Path:    "/test/file.txt",
		OldStr:  &nonExistentStr,
		NewStr:  &newStr,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not appear verbatim")
}

// TestStrReplaceEditor_Insert tests the insert command
func TestStrReplaceEditor_Insert(t *testing.T) {
	mockOperator := NewMockFileOperator()
	ctx := context.Background()

	fileContent := "line 1\nline 2\nline 3\nline 4\nline 5"

	// Setup mock
	mockOperator.On("Exists", mock.Anything, "/test/file.txt").Return(true, nil)
	mockOperator.On("IsDirectory", mock.Anything, "/test/file.txt").Return(false, nil)
	mockOperator.On("ReadFile", mock.Anything, "/test/file.txt").Return(fileContent, nil)
	mockOperator.On("WriteFile", mock.Anything, "/test/file.txt", mock.Anything).Return(nil)

	// Create editor
	editor, _ := NewStrReplaceEditor(ctx, &EditorConfig{Operator: mockOperator})

	// Test insert
	insertLine := 2
	newStr := "inserted line"
	result, err := editor.Execute(ctx, &StrReplaceEditorParams{
		Command:    InsertCommand,
		Path:       "/test/file.txt",
		InsertLine: &insertLine,
		NewStr:     &newStr,
	})

	assert.NoError(t, err)
	assert.Contains(t, result, "has been edited")
	assert.Contains(t, result, "inserted line")

	// Test insert without insertLine
	_, err = editor.Execute(ctx, &StrReplaceEditorParams{
		Command: InsertCommand,
		Path:    "/test/file.txt",
		NewStr:  &newStr,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert_line")

	// Test insert without newStr
	_, err = editor.Execute(ctx, &StrReplaceEditorParams{
		Command:    InsertCommand,
		Path:       "/test/file.txt",
		InsertLine: &insertLine,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "new_str")

	// Test insert with invalid line
	invalidLine := 10
	_, err = editor.Execute(ctx, &StrReplaceEditorParams{
		Command:    InsertCommand,
		Path:       "/test/file.txt",
		InsertLine: &invalidLine,
		NewStr:     &newStr,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid `insert_line`")
}

// TestStrReplaceEditor_UndoEdit tests the undo_edit command
func TestStrReplaceEditor_UndoEdit(t *testing.T) {
	mockOperator := NewMockFileOperator()
	ctx := context.Background()

	fileContent := "line 1\nline 2\nline 3\nline 4\nline 5"

	// Setup mock
	mockOperator.On("Exists", mock.Anything, "/test/file.txt").Return(true, nil)
	mockOperator.On("IsDirectory", mock.Anything, "/test/file.txt").Return(false, nil)
	mockOperator.On("ReadFile", mock.Anything, "/test/file.txt").Return(fileContent, nil)
	mockOperator.On("WriteFile", mock.Anything, "/test/file.txt", mock.Anything).Return(nil)

	// Create editor
	editor, _ := NewStrReplaceEditor(ctx, &EditorConfig{Operator: mockOperator})

	// First make an edit
	oldStr := "line 2\nline 3"
	newStr := "replaced line 2\nreplaced line 3"
	_, err := editor.Execute(ctx, &StrReplaceEditorParams{
		Command: StrReplaceCommand,
		Path:    "/test/file.txt",
		OldStr:  &oldStr,
		NewStr:  &newStr,
	})

	assert.NoError(t, err)

	// Then undo the edit
	result, err := editor.Execute(ctx, &StrReplaceEditorParams{
		Command: UndoEditCommand,
		Path:    "/test/file.txt",
	})

	assert.NoError(t, err)
	assert.Contains(t, result, "Successfully undid")
	assert.Contains(t, result, "line 2")
	assert.NotContains(t, result, "replaced line 2")
}

// TestTruncate tests the truncate function
func TestTruncate(t *testing.T) {
	longContent := strings.Repeat("a", 100)

	// Test no truncation
	result := truncate(longContent, 0)
	assert.Equal(t, longContent, result)

	// Test no truncation when content is shorter
	result = truncate(longContent, 200)
	assert.Equal(t, longContent, result)

	// Test truncation
	result = truncate(longContent, 50)
	assert.Equal(t, longContent[:50]+TruncatedMessage, result)
}

// TestExpandTabs tests the expandTabs function
func TestExpandTabs(t *testing.T) {
	input := "line\twith\ttabs"
	expected := "line    with    tabs"

	result := expandTabs(input)
	assert.Equal(t, expected, result)
}

// TestStrReplaceEditor_InvalidCommand tests handling of invalid commands
func TestStrReplaceEditor_InvalidCommand(t *testing.T) {
	mockOperator := NewMockFileOperator()
	ctx := context.Background()

	// Create editor
	editor, _ := NewStrReplaceEditor(ctx, &EditorConfig{Operator: mockOperator})

	// Test invalid command
	_, err := editor.Execute(ctx, &StrReplaceEditorParams{
		Command: "invalid_command",
		Path:    "/test/file.txt",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized command")
}
