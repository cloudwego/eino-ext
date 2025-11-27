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
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// Constants definition
const (
	SnippetLines     = 4
	MaxResponseLen   = 16000
	TruncatedMessage = "<response clipped><NOTE>To save on context only part of this file has been shown to you. " +
		"You should retry this tool after you have searched inside the file with `grep -n` " +
		"in order to find the line numbers of what you are looking for.</NOTE>"
)

// Command type
type Command string

const (
	ViewCommand       Command = "view"
	CreateCommand     Command = "create"
	StrReplaceCommand Command = "str_replace"
	InsertCommand     Command = "insert"
	UndoEditCommand   Command = "undo_edit"
)

const StrReplaceEditorDescription = `Custom editing tool for viewing, creating and editing files
* State is persistent across command calls and discussions with the user
* If 'path' is a file, 'view' displays the result of applying 'cat -n'. If 'path' is a directory, 'view' lists non-hidden files and directories up to 2 levels deep
* The 'create' command cannot be used if the specified 'path' already exists as a file
* If a 'command' generates a long output, it will be truncated and marked with '<response clipped>'
* The 'undo_edit' command will revert the last edit made to the file at 'path'

Notes for using the 'str_replace' command:
* The 'old_str' parameter should match EXACTLY one or more consecutive lines from the original file. Be mindful of whitespaces!
* If the 'old_str' parameter is not unique in the file, the replacement will not be performed. Make sure to include enough context in 'old_str' to make it unique
* The 'new_str' parameter should contain the edited lines that should replace the 'old_str'`

// StrReplaceEditor struct definition
type StrReplaceEditor struct {
	fileHistory map[string][]string
	operator    Operator
	info        *schema.ToolInfo
}

type StrReplaceEditorParams struct {
	Command    Command `json:"command"`
	Path       string  `json:"path"`
	FileText   *string `json:"file_text,omitempty"`
	ViewRange  []int   `json:"view_range,omitempty"`
	OldStr     *string `json:"old_str,omitempty"`
	NewStr     *string `json:"new_str,omitempty"`
	InsertLine *int    `json:"insert_line,omitempty"`
}

type EditorConfig struct {
	Operator Operator
}

// NewStrReplaceEditor creates a new editor instance
func NewStrReplaceEditor(ctx context.Context, cfg *EditorConfig) (*StrReplaceEditor, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if cfg.Operator == nil {
		return nil, errors.New("operator is required")
	}

	return &StrReplaceEditor{
		info: &schema.ToolInfo{
			Name: "str_replace_editor",
			Desc: StrReplaceEditorDescription,
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(
				&jsonschema.Schema{
					Type:     string(schema.Object),
					Required: []string{"command", "path"},
					Properties: orderedmap.New[string, *jsonschema.Schema](
						orderedmap.WithInitialData[string, *jsonschema.Schema](
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "command",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "The commands to run. Allowed options are: `view`, `create`, `str_replace`, `insert`, `undo_edit`.",
									Enum:        []any{"view", "create", "str_replace", "insert", "undo_edit"},
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "path",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "Absolute path to file or directory.",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "file_text",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "Required parameter of `create` command, with the content of the file to be created.",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "old_str",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "Required parameter of `str_replace` command containing the string in `path` to replace.",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "new_str",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "Optional parameter of `str_replace` command containing the new string (if not given, no string will be added). Required parameter of `insert` command containing the string to insert.",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "insert_line",
								Value: &jsonschema.Schema{
									Type:        string(schema.Integer),
									Description: "Required parameter of `insert` command. The `new_str` will be inserted AFTER the line `insert_line` of `path`.",
								},
							},
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "view_range",
								Value: &jsonschema.Schema{
									Type:        string(schema.Array),
									Description: "Optional parameter of `view` command when `path` points to a file. If none is given, the full file is shown. If provided, the file will be shown in the indicated line number range, e.g. [11, 12] will show lines 11 and 12. Indexing at 1 to start. Setting `[start_line, -1]` shows all lines from `start_line` to the end of the file.",
									Items: &jsonschema.Schema{
										Type: string(schema.Integer),
									},
								},
							},
						),
					),
				},
			),
		},
		fileHistory: make(map[string][]string),
		operator:    cfg.Operator,
	}, nil
}

func (e *StrReplaceEditor) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return e.info, nil
}

func (e *StrReplaceEditor) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	param := &StrReplaceEditorParams{}
	if err := json.Unmarshal([]byte(argumentsInJSON), param); err != nil {
		return "", fmt.Errorf("failed to extract input: %w", err)
	}
	return e.Execute(ctx, param)
}

// Possibly need to truncate content
func truncate(content string, truncateAfter int) string {
	if truncateAfter == 0 || len(content) <= truncateAfter {
		return content
	}
	return content[:truncateAfter] + TruncatedMessage
}

// Execute performs file operations command
func (e *StrReplaceEditor) Execute(ctx context.Context, params *StrReplaceEditorParams) (string, error) {
	// Execute appropriate command
	var result string
	var err error

	switch params.Command {
	case ViewCommand:
		result, err = e.view(ctx, params.Path, params.ViewRange)
	case CreateCommand:
		if params.FileText == nil {
			return "", errors.New("parameter `file_text` is required for create command")
		}
		err = e.operator.WriteFile(ctx, params.Path, *params.FileText)
		if err != nil {
			return "", err
		}
		e.fileHistory[params.Path] = append(e.fileHistory[params.Path], *params.FileText)
		result = fmt.Sprintf("file successfully created at: %s", params.Path)
	case StrReplaceCommand:
		if params.OldStr == nil {
			return "", errors.New("parameter `old_str` is required for str_replace command")
		}
		var newStr string
		if params.NewStr != nil {
			newStr = *params.NewStr
		}
		result, err = e.strReplace(ctx, params.Path, *params.OldStr, newStr)
	case InsertCommand:
		if params.InsertLine == nil {
			return "", errors.New("parameter `insert_line` is required for insert command")
		}
		if params.NewStr == nil {
			return "", errors.New("parameter `new_str` is required for insert command")
		}
		result, err = e.insert(ctx, params.Path, *params.InsertLine, *params.NewStr)
	case UndoEditCommand:
		result, err = e.undoEdit(ctx, params.Path)
	default:
		return "", fmt.Errorf("unrecognized command %s. Allowed commands are: view, create, str_replace, insert, undo_edit", params.Command)
	}

	if err != nil {
		return "", err
	}
	return result, nil
}

// Validate path and command combination
func (e *StrReplaceEditor) validatePath(ctx context.Context, command Command, path string) error {
	// Check if the path is absolute
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path %s is not an absolute path", path)
	}

	// Only check if the path exists for non-create commands
	if command != CreateCommand {
		exists, err := e.operator.Exists(ctx, path)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("path %s does not exist. Please provide a valid path", path)
		}

		// Check if the path is a directory
		isDir, err := e.operator.IsDirectory(ctx, path)
		if err != nil {
			return err
		}
		if isDir && command != ViewCommand {
			return fmt.Errorf("path %s is a directory, only the `view` command can be used on directories", path)
		}
	} else {
		// Check if file already exists for create command
		exists, err := e.operator.Exists(ctx, path)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("file already exists at: %s. Cannot use `create` command to overwrite files", path)
		}
	}

	return nil
}

// View file or directory content
func (e *StrReplaceEditor) view(ctx context.Context, path string, viewRange []int) (string, error) {
	// Determine if the path is a directory
	isDir, err := e.operator.IsDirectory(ctx, path)
	if err != nil {
		return "", err
	}

	if isDir {
		// Directory handling
		if len(viewRange) > 0 {
			return "", errors.New("parameter `view_range` is not allowed when `path` points to a directory")
		}
		return e.viewDirectory(ctx, path)
	} else {
		// File handling
		return e.viewFile(ctx, path, viewRange)
	}
}

// Display directory content
func (e *StrReplaceEditor) viewDirectory(ctx context.Context, path string) (string, error) {
	findCmd := []string{"find", path, "-maxdepth", "2", "-not", "-path", "*/\\.*"}

	// Use operator to execute command
	cmdOutput, err := e.operator.RunCommand(ctx, findCmd)
	if err != nil {
		return "", err
	}

	return cmdOutput.Stdout, nil
}

// Display file content, optionally specifying line range
func (e *StrReplaceEditor) viewFile(ctx context.Context, path string, viewRange []int) (string, error) {
	// Read file content
	fileContent, err := e.operator.ReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	initLine := 1

	// Apply view range if specified
	if len(viewRange) > 0 {
		if len(viewRange) != 2 {
			return "", errors.New("invalid `view_range`, it should be a list of two integers")
		}

		fileLines := strings.Split(fileContent, "\n")
		nLinesFile := len(fileLines)
		initLine, finalLine := viewRange[0], viewRange[1]

		// Validate view range
		if initLine < 1 || initLine > nLinesFile {
			return "", fmt.Errorf("invalid `view_range`: %v. Its first element `%d` should be within the file's line range: [1, %d]", viewRange, initLine, nLinesFile)
		}
		if finalLine > nLinesFile {
			return "", fmt.Errorf("invalid `view_range`: %v. Its second element `%d` should be less than the number of lines in the file: `%d`", viewRange, finalLine, nLinesFile)
		}
		if finalLine != -1 && finalLine < initLine {
			return "", fmt.Errorf("invalid `view_range`: %v. Its second element `%d` should be greater than or equal to its first `%d`", viewRange, finalLine, initLine)
		}

		// Apply range
		if finalLine == -1 {
			fileContent = strings.Join(fileLines[initLine-1:], "\n")
		} else {
			fileContent = strings.Join(fileLines[initLine-1:finalLine], "\n")
		}
	}

	// Format and return result
	return e.makeOutput(fileContent, path, initLine), nil
}

// Replace unique old string with new string in file
func (e *StrReplaceEditor) strReplace(ctx context.Context, path string, oldStr string, newStr string) (string, error) {
	// Read file content and expand tabs
	fileContent, err := e.operator.ReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	fileContent = expandTabs(fileContent)
	oldStr = expandTabs(oldStr)
	if newStr != "" {
		newStr = expandTabs(newStr)
	}

	// Check if oldStr is unique in the file
	occurrences := strings.Count(fileContent, oldStr)
	if occurrences == 0 {
		return "", fmt.Errorf("replacement not performed, old_str `%s` does not appear verbatim in %s", oldStr, path)
	} else if occurrences > 1 {
		// Find line numbers of occurrences
		fileContentLines := strings.Split(fileContent, "\n")
		var lines []int
		for idx, line := range fileContentLines {
			if strings.Contains(line, oldStr) {
				lines = append(lines, idx+1)
			}
		}
		return "", fmt.Errorf("replacement not performed. old_str `%s` appears multiple times at lines %v. please ensure it is unique", oldStr, lines)
	}

	// Replace oldStr with newStr
	newFileContent := strings.Replace(fileContent, oldStr, newStr, 1)

	// Write new content to file
	err = e.operator.WriteFile(ctx, path, newFileContent)
	if err != nil {
		return "", err
	}

	// Save original content to history
	e.fileHistory[path] = append(e.fileHistory[path], fileContent)

	// Create snippet of edited part
	parts := strings.Split(fileContent, oldStr)
	replacementLine := strings.Count(parts[0], "\n")
	startLine := max(0, replacementLine-SnippetLines)
	endLine := replacementLine + SnippetLines + strings.Count(newStr, "\n")
	newFileContentLines := strings.Split(newFileContent, "\n")
	if endLine+1 > len(newFileContentLines) {
		endLine = len(newFileContentLines) - 1
	}
	snippet := strings.Join(newFileContentLines[startLine:endLine+1], "\n")

	// Prepare success messages
	successMsg := fmt.Sprintf("File %s has been edited. ", path)
	successMsg += e.makeOutput(snippet, fmt.Sprintf("a snippet of %s", path), startLine+1)
	successMsg += "Check the changes and make sure they are as expected. Edit the file again if necessary."

	return successMsg, nil
}

// Insert text at specific line in file
func (e *StrReplaceEditor) insert(ctx context.Context, path string, insertLine int, newStr string) (string, error) {
	// Read and prepare content
	fileText, err := e.operator.ReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	fileText = expandTabs(fileText)
	newStr = expandTabs(newStr)
	fileTextLines := strings.Split(fileText, "\n")
	nLinesFile := len(fileTextLines)

	// Validate insert line
	if insertLine < 0 || insertLine > nLinesFile {
		return "", fmt.Errorf("invalid `insert_line` parameter: %d. It should be within the file's line range: [0, %d]", insertLine, nLinesFile)
	}

	// Perform insertion
	newStrLines := strings.Split(newStr, "\n")
	var newFileTextLines []string
	newFileTextLines = append(newFileTextLines, fileTextLines[:insertLine]...)
	newFileTextLines = append(newFileTextLines, newStrLines...)
	newFileTextLines = append(newFileTextLines, fileTextLines[insertLine:]...)

	// Create preview snippet
	var snippetLines []string
	startPreview := max(0, insertLine-SnippetLines)
	snippetLines = append(snippetLines, fileTextLines[startPreview:insertLine]...)
	snippetLines = append(snippetLines, newStrLines...)
	endPreview := min(nLinesFile, insertLine+SnippetLines)
	snippetLines = append(snippetLines, fileTextLines[insertLine:endPreview]...)

	// Join lines and write to file
	newFileText := strings.Join(newFileTextLines, "\n")
	snippet := strings.Join(snippetLines, "\n")

	err = e.operator.WriteFile(ctx, path, newFileText)
	if err != nil {
		return "", err
	}
	e.fileHistory[path] = append(e.fileHistory[path], fileText)

	// Prepare success messages
	successMsg := fmt.Sprintf("File %s has been edited. ", path)
	successMsg += e.makeOutput(snippet, "a snippet of the edited file", max(1, insertLine-SnippetLines+1))
	successMsg += "Check the changes and make sure they are as expected (correct indentation, no duplicate lines, etc). Edit the file again if necessary."

	return successMsg, nil
}

// Undo last edit to file
func (e *StrReplaceEditor) undoEdit(ctx context.Context, path string) (string, error) {
	if len(e.fileHistory[path]) == 0 {
		return "", fmt.Errorf("no edit history found for %s", path)
	}

	lastIdx := len(e.fileHistory[path]) - 1
	oldText := e.fileHistory[path][lastIdx]
	e.fileHistory[path] = e.fileHistory[path][:lastIdx]

	err := e.operator.WriteFile(ctx, path, oldText)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully undid last edit to %s. %s", path, e.makeOutput(oldText, path, 1)), nil
}

// Format file content to display line numbers
func (e *StrReplaceEditor) makeOutput(fileContent string, fileDescriptor string, initLine int) string {
	fileContent = truncate(fileContent, MaxResponseLen)
	fileContent = expandTabs(fileContent)

	// Add line numbers to each line
	lines := strings.Split(fileContent, "\n")
	var numberedLines []string
	for i, line := range lines {
		numberedLines = append(numberedLines, fmt.Sprintf("%6d\t%s", i+initLine, line))
	}
	numberedContent := strings.Join(numberedLines, "\n")

	return fmt.Sprintf("Here's the result of running `cat -n` on %s:\n%s\n", fileDescriptor, numberedContent)
}

// Helper function: expand tabs
func expandTabs(s string) string {
	return strings.Replace(s, "\t", "    ", -1)
}
