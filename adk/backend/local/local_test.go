/*
 * Copyright 2026 CloudWeGo Authors
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

package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/gen2brain/go-fitz"
	"github.com/stretchr/testify/assert"
)

func setupTestDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "sandbox-test")
	assert.NoError(t, err)
	return dir
}

func TestLsInfo(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	t.Run("list directory successfully", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)

		// Create test files and directories
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(""), 0644))
		assert.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))

		req := &filesystem.LsInfoRequest{Path: dir}
		files, err := s.LsInfo(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, files, 2)
		assert.Equal(t, "file1.txt", files[0].Path)
		assert.Equal(t, "subdir", files[1].Path)
	})

	t.Run("list non-existent directory", func(t *testing.T) {
		req := &filesystem.LsInfoRequest{Path: "/non-existent-dir"}
		files, err := s.LsInfo(ctx, req)
		assert.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("path is a file, not a directory", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "file.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte(""), 0644))

		req := &filesystem.LsInfoRequest{Path: filePath}
		_, err := s.LsInfo(ctx, req)
		assert.Error(t, err)
	})
}

func TestRead(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	t.Run("read file successfully", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "test.txt")
		content := "line 1\nline 2\nline 3"
		assert.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

		req := &filesystem.ReadRequest{FilePath: filePath, Offset: 2, Limit: 1}
		result, err := s.Read(ctx, req)
		assert.NoError(t, err)
		assert.Contains(t, result.Content, "line 2")
	})

	t.Run("read file from first line (offset=1)", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "test.txt")
		content := "line 1\nline 2\nline 3"
		assert.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

		req := &filesystem.ReadRequest{FilePath: filePath, Offset: 1, Limit: 1}
		result, err := s.Read(ctx, req)
		assert.NoError(t, err)
		assert.Contains(t, result.Content, "line 1")
		assert.NotContains(t, result.Content, "line 2")
	})

	t.Run("read empty file", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "empty.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte(""), 0644))

		req := &filesystem.ReadRequest{FilePath: filePath}
		result, err := s.Read(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, "", result.Content)
	})

	t.Run("read non-existent file", func(t *testing.T) {
		req := &filesystem.ReadRequest{FilePath: "/non-existent-file.txt"}
		_, err := s.Read(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("read large file with pagination", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "large.txt")

		f, err := os.Create(filePath)
		assert.NoError(t, err)
		for i := 1; i <= 1000; i++ {
			f.WriteString(fmt.Sprintf("line %d\n", i))
		}
		f.Close()

		req := &filesystem.ReadRequest{FilePath: filePath, Offset: 500, Limit: 5}
		result, err := s.Read(ctx, req)
		assert.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(result.Content), "\n")
		assert.Len(t, lines, 5)
		assert.Contains(t, lines[0], "line 500")
		assert.Contains(t, lines[4], "line 504")
	})
}

func TestWrite(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	t.Run("write new file successfully", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "newfile.txt")

		req := &filesystem.WriteRequest{FilePath: filePath, Content: "hello"}
		err := s.Write(ctx, req)
		assert.NoError(t, err)

		content, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(content))
	})

	t.Run("write to existing file", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "existing.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte("initial"), 0644))

		req := &filesystem.WriteRequest{FilePath: filePath, Content: "new content"}
		err := s.Write(ctx, req)
		assert.NoError(t, err)

	})
}

func TestEdit(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	t.Run("edit file successfully - replace one", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "test.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0644))

		req := &filesystem.EditRequest{FilePath: filePath, OldString: "world", NewString: "go"}
		err := s.Edit(ctx, req)
		assert.NoError(t, err)

		content, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, "hello go", string(content))
	})

	t.Run("edit file successfully - replace all", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "test.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte("hello world, beautiful world"), 0644))

		req := &filesystem.EditRequest{FilePath: filePath, OldString: "world", NewString: "go", ReplaceAll: true}
		err := s.Edit(ctx, req)
		assert.NoError(t, err)

		content, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, "hello go, beautiful go", string(content))
	})

	t.Run("string not found in file", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "test.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0644))

		req := &filesystem.EditRequest{FilePath: filePath, OldString: "nonexistent", NewString: "go"}
		err := s.Edit(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "string not found")
	})

	t.Run("multiple occurrences without replace_all", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "test.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte("hello world, beautiful world"), 0644))

		req := &filesystem.EditRequest{FilePath: filePath, OldString: "world", NewString: "go", ReplaceAll: false}
		err := s.Edit(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "appears multiple times")
	})
}

func TestGrepRaw(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	t.Run("grep successfully", func(t *testing.T) {
		filePath := "/tmp/test/test.txt"
		mockOutput := `{"type":"match","data":{"path":{"text":"` + filePath + `"},"line_number":2,"lines":{"text":"go\n"}}}
{"type":"match","data":{"path":{"text":"` + filePath + `"},"line_number":4,"lines":{"text":"go\n"}}}`

		mockRgBin := createMockRg(t, mockOutput, 0)
		t.Setenv("PATH", mockRgBin+":"+os.Getenv("PATH"))

		req := &filesystem.GrepRequest{Path: "/tmp/test", Pattern: "go"}
		matches, err := s.GrepRaw(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, matches, 2)
		assert.Equal(t, filePath, matches[0].Path)
		assert.Equal(t, 2, matches[0].Line)
		assert.Equal(t, "go", matches[0].Content)
	})

	t.Run("grep with glob", func(t *testing.T) {
		mockOutput := `{"type":"match","data":{"path":{"text":"/tmp/test/test.txt"},"line_number":1,"lines":{"text":"hello go\n"}}}`

		mockRgBin := createMockRg(t, mockOutput, 0)
		t.Setenv("PATH", mockRgBin+":"+os.Getenv("PATH"))

		req := &filesystem.GrepRequest{Path: "/tmp/test", Pattern: "go", Glob: "*.txt"}
		matches, err := s.GrepRaw(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.True(t, strings.HasSuffix(matches[0].Path, ".txt"))
	})

	t.Run("grep with no matches", func(t *testing.T) {
		// rg exits with code 1 when no matches found
		mockRgBin := createMockRg(t, "", 1)
		t.Setenv("PATH", mockRgBin+":"+os.Getenv("PATH"))

		req := &filesystem.GrepRequest{Path: "/tmp/test", Pattern: "nonexistent"}
		matches, err := s.GrepRaw(ctx, req)
		assert.NoError(t, err)
		assert.Empty(t, matches)
	})

	t.Run("grep with empty pattern", func(t *testing.T) {
		req := &filesystem.GrepRequest{Path: "/tmp/test", Pattern: ""}
		_, err := s.GrepRaw(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pattern is required")
	})
}

// createMockRg creates a fake "rg" script in a temp directory that outputs the given content
// and exits with the specified code. Returns the directory path to prepend to PATH.
func createMockRg(t *testing.T, output string, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\ncat <<'MOCK_EOF'\n%s\nMOCK_EOF\nexit %d\n", output, exitCode)
	rgPath := filepath.Join(dir, "rg")
	assert.NoError(t, os.WriteFile(rgPath, []byte(script), 0755))
	return dir
}

func TestGlobInfo(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	t.Run("glob successfully", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte(""), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "c.log"), []byte(""), 0644))

		req := &filesystem.GlobInfoRequest{Path: dir, Pattern: "*.txt"}
		files, err := s.GlobInfo(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, files, 2)
		assert.Equal(t, "a.txt", files[0].Path)
		assert.Equal(t, "b.txt", files[1].Path)
	})

	t.Run("glob with no matches", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)

		req := &filesystem.GlobInfoRequest{Path: dir, Pattern: "*.nonexistent"}
		files, err := s.GlobInfo(ctx, req)
		assert.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("glob recursive", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)

		assert.NoError(t, os.MkdirAll(filepath.Join(dir, "sub", "subsub"), 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "root.txt"), []byte(""), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "sub.txt"), []byte(""), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "subsub", "deep.txt"), []byte(""), 0644))

		req := &filesystem.GlobInfoRequest{Path: dir, Pattern: "**/*.txt"}
		files, err := s.GlobInfo(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, files, 3)

		// Expected paths are relative and use forward slash
		expected := []string{"root.txt", "sub/sub.txt", "sub/subsub/deep.txt"}
		var actual []string
		for _, f := range files {
			actual = append(actual, f.Path)
		}
		assert.ElementsMatch(t, expected, actual)
	})

	t.Run("glob with question mark", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(""), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "fileA.txt"), []byte(""), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "file10.txt"), []byte(""), 0644))

		req := &filesystem.GlobInfoRequest{Path: dir, Pattern: "file?.txt"}
		files, err := s.GlobInfo(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, files, 2)

		expected := []string{"file1.txt", "fileA.txt"}
		var actual []string
		for _, f := range files {
			actual = append(actual, f.Path)
		}
		assert.ElementsMatch(t, expected, actual)
	})

	t.Run("glob with brackets", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(""), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "file2.txt"), []byte(""), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "file3.txt"), []byte(""), 0644))

		req := &filesystem.GlobInfoRequest{Path: dir, Pattern: "file[13].txt"}
		files, err := s.GlobInfo(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, files, 2)

		expected := []string{"file1.txt", "file3.txt"}
		var actual []string
		for _, f := range files {
			actual = append(actual, f.Path)
		}
		assert.ElementsMatch(t, expected, actual)
	})
}

func TestPathCleaning(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	dir := setupTestDir(t)
	defer os.RemoveAll(dir)

	// Create a file in the temp directory
	filename := "test.txt"
	filePath := filepath.Join(dir, filename)
	content := "hello world"
	err = os.WriteFile(filePath, []byte(content), 0644)
	assert.NoError(t, err)

	t.Run("Read with dirty path", func(t *testing.T) {
		// Construct a dirty path: /tmp/dir/../dir/test.txt
		dirtyPath := filepath.Join(dir, "..", filepath.Base(dir), filename)
		// Ensure it's absolute
		if !filepath.IsAbs(dirtyPath) {
			dirtyPath, _ = filepath.Abs(dirtyPath)
		}

		// Verify that using the dirty path works (it should be cleaned internally)
		req := &filesystem.ReadRequest{FilePath: dirtyPath}
		res, err := s.Read(ctx, req)
		assert.NoError(t, err)
		assert.Contains(t, res.Content, content)
	})

	t.Run("Read with repeated slashes", func(t *testing.T) {
		// Construct path with repeated slashes: /tmp//dir/test.txt
		// We insert an extra slash after the directory separator
		dirtyPath := filepath.Join(dir, filename)
		// Inject double slash. On unix /a/b -> /a//b works.
		dirtyPath = "/" + strings.TrimPrefix(dirtyPath, "/")
		dirtyPath = strings.Replace(dirtyPath, "/", "//", 1)

		req := &filesystem.ReadRequest{FilePath: dirtyPath}
		res, err := s.Read(ctx, req)
		assert.NoError(t, err)
		assert.Contains(t, res.Content, content)
	})

	t.Run("Write with dirty path", func(t *testing.T) {
		newFile := "write_test.txt"
		// /tmp/dir/subdir/../write_test.txt
		dirtyPath := filepath.Join(dir, "subdir", "..", newFile)

		req := &filesystem.WriteRequest{
			FilePath: dirtyPath,
			Content:  "new content",
		}
		err := s.Write(ctx, req)
		assert.NoError(t, err)

		// Verify file exists at the clean path
		cleanPath := filepath.Join(dir, newFile)
		_, err = os.Stat(cleanPath)
		assert.NoError(t, err)
	})

	t.Run("LsInfo with dirty path", func(t *testing.T) {
		// /tmp/dir/./
		dirtyPath := filepath.Join(dir, ".")
		req := &filesystem.LsInfoRequest{Path: dirtyPath}
		files, err := s.LsInfo(ctx, req)
		assert.NoError(t, err)

		// Should find test.txt and write_test.txt
		found := false
		for _, f := range files {
			if f.Path == filename {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("Relative path allowed", func(t *testing.T) {
		prevWD, err := os.Getwd()
		assert.NoError(t, err)
		t.Cleanup(func() { _ = os.Chdir(prevWD) })
		assert.NoError(t, os.Chdir(dir))

		relativePath := filepath.Join("relative", "path.txt")
		assert.NoError(t, os.MkdirAll(filepath.Dir(relativePath), 0755))
		assert.NoError(t, os.WriteFile(relativePath, []byte(content), 0644))

		req := &filesystem.ReadRequest{FilePath: relativePath}
		res, err := s.Read(ctx, req)
		assert.NoError(t, err)
		assert.Contains(t, res.Content, content)
	})
}

func TestExecuteStreaming(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	t.Run("ExecuteStreaming with echo", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "echo line1 && echo line2 && echo line3"}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var outputs []string
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil && resp.Output != "" {
				outputs = append(outputs, strings.TrimSpace(resp.Output))
			}
		}

		assert.Len(t, outputs, 3)
		assert.Equal(t, "line1", outputs[0])
		assert.Equal(t, "line2", outputs[1])
		assert.Equal(t, "line3", outputs[2])
	})

	t.Run("ExecuteStreaming with ping", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "ping -c 3 127.0.0.1"}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var lineCount int
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil && resp.Output != "" {
				lineCount++
			}
		}

		assert.Greater(t, lineCount, 0, "should receive at least one line of output")
	})

	t.Run("ExecuteStreaming with seq command", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "seq 1 5"}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var numbers []string
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil && resp.Output != "" {
				numbers = append(numbers, strings.TrimSpace(resp.Output))
			}
		}

		assert.Len(t, numbers, 5)
		assert.Equal(t, "1", numbers[0])
		assert.Equal(t, "5", numbers[4])
	})

	t.Run("ExecuteStreaming with context cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		req := &filesystem.ExecuteRequest{Command: "seq 1 1000000"}
		sr, err := s.ExecuteStreaming(cancelCtx, req)
		assert.NoError(t, err)

		var lineCount int
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil && resp.Output != "" {
				lineCount++
				if lineCount >= 5 {
					cancel()
				}
			}
		}

		assert.GreaterOrEqual(t, lineCount, 5, "should receive at least 5 lines before cancellation")
	})

	t.Run("ExecuteStreaming with command failure", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "echo output && exit 1"}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var hasOutput bool
		var lastResp *filesystem.ExecuteResponse
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil {
				if resp.Output != "" {
					hasOutput = true
				}
				lastResp = resp
			}
		}

		assert.True(t, hasOutput, "should receive output before final response")
		assert.NotNil(t, lastResp)
		assert.NotNil(t, lastResp.ExitCode)
		assert.Equal(t, 1, *lastResp.ExitCode)
		assert.Contains(t, lastResp.Output, "non-zero code")
	})

	t.Run("ExecuteStreaming with stderr output", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "echo stdout && echo stderr >&2 && exit 1"}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var outputs []string
		var lastResp *filesystem.ExecuteResponse
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil {
				if resp.Output != "" {
					outputs = append(outputs, strings.TrimSpace(resp.Output))
				}
				lastResp = resp
			}
		}

		assert.NotNil(t, lastResp)
		assert.NotNil(t, lastResp.ExitCode)
		assert.Equal(t, 1, *lastResp.ExitCode)
		assert.Contains(t, lastResp.Output, "non-zero code")
		assert.Contains(t, lastResp.Output, "stderr")
		// stdout is streamed separately
		assert.True(t, len(outputs) >= 1)
		assert.Contains(t, outputs, "stdout")
	})

	t.Run("ExecuteStreaming with empty command", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: ""}
		_, err := s.ExecuteStreaming(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command is required")
	})

	t.Run("ExecuteStreaming with large output", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "seq 1 100"}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var lineCount int
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil && resp.Output != "" {
				lineCount++
			}
		}

		assert.Equal(t, 100, lineCount, "should receive all 100 lines")
	})

	t.Run("ExecuteStreaming with normal completion", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "echo test"}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var receivedOutput bool
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil && resp.Output != "" {
				receivedOutput = true
			}
		}

		assert.True(t, receivedOutput, "should receive output")
	})

	t.Run("ExecuteStreaming with invalid command", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "/nonexistent/command"}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var lastErr error
		for {
			resp, err := sr.Recv()
			if err != nil {
				lastErr = err
				break
			}
			if resp != nil {
				// 可能有输出
			}
		}

		assert.Error(t, lastErr, "should receive error for invalid command")
	})

	t.Run("ExecuteStreaming with no stdout output", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "true"}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var receivedResponse bool
		var exitCode *int
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil {
				receivedResponse = true
				if resp.ExitCode != nil {
					exitCode = resp.ExitCode
				}
			}
		}

		assert.True(t, receivedResponse, "should receive at least one response even with no stdout")
		assert.NotNil(t, exitCode, "should receive exit code in response")
		assert.Equal(t, 0, *exitCode, "exit code should be 0 for successful command")
	})

	t.Run("ExecuteStreaming with RunInBackendGround", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{
			Command:            "sleep 10",
			RunInBackendGround: true,
		}
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var receivedResponse bool
		var output string
		var exitCode *int
		for {
			resp, err := sr.Recv()
			if err != nil {
				break
			}
			if resp != nil {
				receivedResponse = true
				output = resp.Output
				exitCode = resp.ExitCode
			}
		}

		assert.True(t, receivedResponse, "should receive response for background command")
		assert.Contains(t, output, "background", "should indicate command started in background")
		assert.NotNil(t, exitCode, "should receive exit code")
		assert.Equal(t, 0, *exitCode, "exit code should be 0")
	})

	t.Run("ExecuteStreaming with RunInBackendGround returns immediately", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{
			Command:            "sleep 5",
			RunInBackendGround: true,
		}

		start := time.Now()
		sr, err := s.ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		for {
			_, err := sr.Recv()
			if err != nil {
				break
			}
		}
		elapsed := time.Since(start)

		assert.Less(t, elapsed, 2*time.Second, "background command should return immediately without waiting")
	})
}

func TestExecute(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	t.Run("simple echo", func(t *testing.T) {
		resp, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: "echo hello"})
		assert.NoError(t, err)
		assert.Equal(t, "hello\n", resp.Output)
		assert.NotNil(t, resp.ExitCode)
		assert.Equal(t, 0, *resp.ExitCode)
	})

	t.Run("multi-line output", func(t *testing.T) {
		resp, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: "echo line1 && echo line2 && echo line3"})
		assert.NoError(t, err)
		assert.Equal(t, "line1\nline2\nline3\n", resp.Output)
		assert.Equal(t, 0, *resp.ExitCode)
	})

	t.Run("empty command", func(t *testing.T) {
		_, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command is required")
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		resp, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: "exit 1"})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.ExitCode)
		assert.Equal(t, 1, *resp.ExitCode)
		assert.Contains(t, resp.Output, "non-zero code")
	})

	t.Run("non-zero exit code with stderr", func(t *testing.T) {
		resp, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: "echo fail >&2 && exit 2"})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.ExitCode)
		assert.Equal(t, 2, *resp.ExitCode)
		assert.Contains(t, resp.Output, "non-zero code 2")
		assert.Contains(t, resp.Output, "fail")
	})

	t.Run("command not found", func(t *testing.T) {
		resp, err := s.Execute(ctx, &filesystem.ExecuteRequest{Command: "nonexistent_command_xyz"})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.ExitCode)
		assert.NotEqual(t, 0, *resp.ExitCode)
	})

	t.Run("context cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := s.Execute(cancelCtx, &filesystem.ExecuteRequest{Command: "sleep 10"})
		assert.Error(t, err)
	})

	t.Run("context cancellation during execution", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		resp, err := s.Execute(cancelCtx, &filesystem.ExecuteRequest{Command: "sleep 10"})
		// Process killed by context cancellation produces an ExitError, returned as response
		if err != nil {
			// Non-ExitError case is also acceptable
			return
		}
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.ExitCode)
		assert.NotEqual(t, 0, *resp.ExitCode)
	})
}

func TestResolveMultiModalReadConfig(t *testing.T) {
	t.Run("zero values fall back to defaults", func(t *testing.T) {
		got := resolveMultiModalReadConfig(MultiModalReadConfig{})
		assert.Equal(t, defaultMaxImageSizeMB, got.MaxImageSizeMB)
		assert.Equal(t, defaultMaxPDFSizeMB, got.MaxPDFSizeMB)
		assert.Equal(t, defaultMaxPagedPDFSizeMB, got.MaxPagedPDFSizeMB)
		assert.Equal(t, defaultMaxPDFPagesPerRequest, got.MaxPDFPagesPerRequest)
		assert.InDelta(t, defaultPDFRenderDPI, got.PDFRenderDPI, 0.001)
	})

	t.Run("user values within caps are preserved", func(t *testing.T) {
		got := resolveMultiModalReadConfig(MultiModalReadConfig{
			MaxImageSizeMB:        15,
			MaxPDFSizeMB:          25,
			MaxPagedPDFSizeMB:     200,
			MaxPDFPagesPerRequest: 50,
			PDFRenderDPI:          200,
		})
		assert.Equal(t, 15, got.MaxImageSizeMB)
		assert.Equal(t, 25, got.MaxPDFSizeMB)
		assert.Equal(t, 200, got.MaxPagedPDFSizeMB)
		assert.Equal(t, 50, got.MaxPDFPagesPerRequest)
		assert.InDelta(t, 200.0, got.PDFRenderDPI, 0.001)
	})

	t.Run("over-cap values are clamped", func(t *testing.T) {
		got := resolveMultiModalReadConfig(MultiModalReadConfig{
			MaxImageSizeMB:        9999,
			MaxPDFSizeMB:          9999,
			MaxPagedPDFSizeMB:     9999,
			MaxPDFPagesPerRequest: 99999,
			PDFRenderDPI:          5000,
		})
		assert.Equal(t, maxConfigurableMB, got.MaxImageSizeMB)
		assert.Equal(t, maxConfigurableMB, got.MaxPDFSizeMB)
		assert.Equal(t, maxConfigurableMB, got.MaxPagedPDFSizeMB)
		assert.Equal(t, maxConfigurablePDFPagesPerRequest, got.MaxPDFPagesPerRequest)
		assert.InDelta(t, maxConfigurablePDFRenderDPI, got.PDFRenderDPI, 0.001)
	})

	t.Run("negative values fall back to defaults", func(t *testing.T) {
		got := resolveMultiModalReadConfig(MultiModalReadConfig{
			MaxImageSizeMB: -1, PDFRenderDPI: -10,
		})
		assert.Equal(t, defaultMaxImageSizeMB, got.MaxImageSizeMB)
		assert.InDelta(t, defaultPDFRenderDPI, got.PDFRenderDPI, 0.001)
	})
}

func TestParsePagesParam(t *testing.T) {
	cases := []struct {
		name      string
		pages     string
		maxPages  int
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{name: "single page", pages: "5", maxPages: 20, wantStart: 5, wantEnd: 5},
		{name: "valid range", pages: "1-3", maxPages: 20, wantStart: 1, wantEnd: 3},
		{name: "with whitespace", pages: " 2 - 4 ", maxPages: 20, wantStart: 2, wantEnd: 4},
		{name: "empty", pages: "", maxPages: 20, wantErr: true},
		{name: "open-ended", pages: "1-", maxPages: 20, wantErr: true},
		{name: "multi range", pages: "1-2-3", maxPages: 20, wantErr: true},
		{name: "non integer", pages: "a-b", maxPages: 20, wantErr: true},
		{name: "zero start", pages: "0", maxPages: 20, wantErr: true},
		{name: "end before start", pages: "5-2", maxPages: 20, wantErr: true},
		{name: "exceeds maxPages", pages: "1-30", maxPages: 20, wantErr: true},
		{name: "non-positive maxPages defaults", pages: "1-3", maxPages: 0, wantStart: 1, wantEnd: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := parsePagesParam(tc.pages, tc.maxPages)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantEnd, end)
		})
	}
}

func TestDetectImageMIME(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{name: "png", data: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}, want: "image/png"},
		{name: "jpeg", data: []byte{0xFF, 0xD8, 0xFF, 0xE0}, want: "image/jpeg"},
		{name: "gif87", data: []byte("GIF87a..."), want: "image/gif"},
		{name: "gif89", data: []byte("GIF89a..."), want: "image/gif"},
		{name: "bmp", data: []byte("BM12345"), want: "image/bmp"},
		{name: "webp", data: append([]byte("RIFF1234WEBP"), 0x00), want: "image/webp"},
		{name: "tiff-le", data: []byte{0x49, 0x49, 0x2A, 0x00, 0x00}, want: "image/tiff"},
		{name: "tiff-be", data: []byte{0x4D, 0x4D, 0x00, 0x2A, 0x00}, want: "image/tiff"},
		{name: "unknown", data: []byte("hello world"), want: ""},
		{name: "too short", data: []byte{0x89}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, detectImageMIME(tc.data))
		})
	}
}

func TestIsPDFBytes(t *testing.T) {
	assert.True(t, isPDFBytes([]byte("%PDF-1.7\n...")))
	assert.False(t, isPDFBytes([]byte("not a pdf")))
	assert.False(t, isPDFBytes([]byte("%PDF")))
}

func TestLocal_MultiModalRead_NonMedia(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	dir := setupTestDir(t)
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "hello.txt")
	assert.NoError(t, os.WriteFile(path, []byte("line1\nline2"), 0644))

	resp, err := s.MultiModalRead(ctx, &filesystem.MultiModalReadRequest{
		ReadRequest: filesystem.ReadRequest{FilePath: path},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.FileContent)
	assert.Contains(t, resp.FileContent.Content, "line1")
	assert.Empty(t, resp.Parts)
}

func TestLocal_MultiModalRead_Image(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	dir := setupTestDir(t)
	defer os.RemoveAll(dir)

	cases := []struct {
		ext      string
		header   []byte
		wantMIME string
	}{
		{ext: ".png", header: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01}, wantMIME: "image/png"},
		{ext: ".jpg", header: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, wantMIME: "image/jpeg"},
	}
	for _, tc := range cases {
		t.Run(tc.ext, func(t *testing.T) {
			path := filepath.Join(dir, "img"+tc.ext)
			assert.NoError(t, os.WriteFile(path, tc.header, 0644))
			resp, err := s.MultiModalRead(ctx, &filesystem.MultiModalReadRequest{
				ReadRequest: filesystem.ReadRequest{FilePath: path},
			})
			assert.NoError(t, err)
			assert.Len(t, resp.Parts, 1)
			assert.Equal(t, filesystem.FileContentPartTypeImage, resp.Parts[0].Type)
			assert.Equal(t, tc.wantMIME, resp.Parts[0].MIMEType)
			assert.True(t, bytes.Equal(resp.Parts[0].Data, tc.header))
		})
	}
}

func TestLocal_MultiModalRead_BadImageContent(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	dir := setupTestDir(t)
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "fake.png")
	assert.NoError(t, os.WriteFile(path, []byte("not really png"), 0644))

	_, err = s.MultiModalRead(ctx, &filesystem.MultiModalReadRequest{
		ReadRequest: filesystem.ReadRequest{FilePath: path},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a recognized image format")
}

func TestLocal_MultiModalRead_OversizeImage(t *testing.T) {
	ctx := context.Background()
	// Use a tiny limit so we can exceed it cheaply.
	s, err := NewBackend(ctx, &Config{
		MultiModalRead: MultiModalReadConfig{MaxImageSizeMB: 1},
	})
	assert.NoError(t, err)

	dir := setupTestDir(t)
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "big.png")
	// 2 MB > 1 MB limit; content doesn't need to be a real PNG since size check fires first.
	assert.NoError(t, os.WriteFile(path, make([]byte, 2*1024*1024), 0644))

	_, err = s.MultiModalRead(ctx, &filesystem.MultiModalReadRequest{
		ReadRequest: filesystem.ReadRequest{FilePath: path},
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errReadAllBytesTooLarge))
}

func TestLocal_MultiModalRead_BadPDFContent(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	dir := setupTestDir(t)
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "fake.pdf")
	assert.NoError(t, os.WriteFile(path, []byte("not a pdf"), 0644))

	_, err = s.MultiModalRead(ctx, &filesystem.MultiModalReadRequest{
		ReadRequest: filesystem.ReadRequest{FilePath: path},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid PDF")
}

func TestLocal_ReadAllBytes_NotFound(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	_, err = s.readAllBytes(ctx, "/no/such/file/here", 1024)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestLocal_ReadAllBytes_Directory(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	dir := setupTestDir(t)
	defer os.RemoveAll(dir)

	_, err = s.readAllBytes(ctx, dir, 1024)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

// minimalValidPDF is a hand-crafted single-page PDF, kept small enough to
// embed inline. Used to verify rendering plumbing (cancellation, page bounds)
// without committing a binary fixture.
const minimalValidPDF = "%PDF-1.1\n%\xa5\xb1\xeb\n\n1 0 obj\n  << /Type /Catalog\n     /Pages 2 0 R\n  >>\nendobj\n\n2 0 obj\n  << /Type /Pages\n     /Kids [3 0 R]\n     /Count 1\n     /MediaBox [0 0 100 100]\n  >>\nendobj\n\n3 0 obj\n  <<  /Type /Page\n      /Parent 2 0 R\n      /Resources << >>\n  >>\nendobj\n\nxref\n0 4\n0000000000 65535 f \n0000000018 00000 n \n0000000077 00000 n \n0000000178 00000 n \ntrailer\n  <<  /Root 1 0 R\n      /Size 4\n  >>\nstartxref\n240\n%%EOF\n"

func TestRenderPDFPagesToImages_RespectsCanceledCtx(t *testing.T) {
	doc, err := fitz.NewFromMemory([]byte(minimalValidPDF))
	if err != nil || doc.NumPage() < 1 {
		t.Skipf("minimal PDF fixture not parseable in this environment: err=%v", err)
	}
	defer doc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invocation; the loop's select must fire on first iteration.

	parts, err := renderPDFPagesToImages(ctx, doc, 1, 1, "fixture.pdf", 72)
	assert.Nil(t, parts)
	assert.ErrorIs(t, err, context.Canceled)
}
