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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/filesystem"
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

		req := &filesystem.ReadRequest{FilePath: filePath, Offset: 1, Limit: 1}
		result, err := s.Read(ctx, req)
		assert.NoError(t, err)
		assert.Contains(t, result, "line 2")
	})

	t.Run("read empty file", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "empty.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte(""), 0644))

		req := &filesystem.ReadRequest{FilePath: filePath}
		result, err := s.Read(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, "", result)
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

		// Create a file with 1000 lines
		f, err := os.Create(filePath)
		assert.NoError(t, err)
		for i := 0; i < 1000; i++ {
			f.WriteString(fmt.Sprintf("line %d\n", i))
		}
		f.Close()

		// Read lines 500-505 (5 lines)
		// Offset is 0-indexed, so offset 500 starts at line 500 (which is the 501st line)
		req := &filesystem.ReadRequest{FilePath: filePath, Offset: 500, Limit: 5}
		result, err := s.Read(ctx, req)
		assert.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(result), "\n")
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
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
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
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "test.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte("hello\ngo\nworld\ngo"), 0644))

		req := &filesystem.GrepRequest{Path: dir, Pattern: "go"}
		matches, err := s.GrepRaw(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, matches, 2)
		assert.Equal(t, filePath, matches[0].Path)
		assert.Equal(t, 2, matches[0].Line)
		assert.Equal(t, "go", matches[0].Content)
	})

	t.Run("grep with glob", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello go"), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "test.log"), []byte("hello go"), 0644))

		req := &filesystem.GrepRequest{Path: dir, Pattern: "go", Glob: "*.txt"}
		matches, err := s.GrepRaw(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.True(t, strings.HasSuffix(matches[0].Path, ".txt"))
	})

	t.Run("grep with no matches", func(t *testing.T) {
		dir := setupTestDir(t)
		defer os.RemoveAll(dir)
		filePath := filepath.Join(dir, "test.txt")
		assert.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0644))

		req := &filesystem.GrepRequest{Path: dir, Pattern: "nonexistent"}
		matches, err := s.GrepRaw(ctx, req)
		assert.NoError(t, err)
		assert.Empty(t, matches)
	})
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
		assert.Contains(t, res, content)
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
		assert.Contains(t, res, content)
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

	t.Run("Relative path rejected", func(t *testing.T) {
		req := &filesystem.ReadRequest{FilePath: "relative/path.txt"}
		_, err := s.Read(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path must be an absolute path")
	})
}

func TestExecuteStreaming(t *testing.T) {
	ctx := context.Background()
	s, err := NewBackend(ctx, &Config{})
	assert.NoError(t, err)

	t.Run("ExecuteStreaming with echo", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "echo line1 && echo line2 && echo line3"}
		sr, err := s.(*backend).ExecuteStreaming(ctx, req)
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
		sr, err := s.(*backend).ExecuteStreaming(ctx, req)
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
		sr, err := s.(*backend).ExecuteStreaming(ctx, req)
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
		sr, err := s.(*backend).ExecuteStreaming(cancelCtx, req)
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
		sr, err := s.(*backend).ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var hasOutput bool
		var lastErr error
		for {
			resp, err := sr.Recv()
			if err != nil {
				lastErr = err
				break
			}
			if resp != nil && resp.Output != "" {
				hasOutput = true
			}
		}

		assert.True(t, hasOutput, "should receive output before error")
		assert.Error(t, lastErr, "should receive error when command fails")
		assert.Contains(t, lastErr.Error(), "non-zero code")
	})

	t.Run("ExecuteStreaming with stderr output", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "echo stdout && echo stderr >&2 && exit 1"}
		sr, err := s.(*backend).ExecuteStreaming(ctx, req)
		assert.NoError(t, err)

		var outputs []string
		var lastErr error
		for {
			resp, err := sr.Recv()
			if err != nil {
				lastErr = err
				break
			}
			if resp != nil && resp.Output != "" {
				outputs = append(outputs, strings.TrimSpace(resp.Output))
			}
		}

		assert.Len(t, outputs, 1)
		assert.Equal(t, "stdout", outputs[0])
		assert.Error(t, lastErr)
		assert.Contains(t, lastErr.Error(), "stderr")
		assert.Contains(t, lastErr.Error(), "non-zero code")
	})

	t.Run("ExecuteStreaming with empty command", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: ""}
		_, err := s.(*backend).ExecuteStreaming(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command is required")
	})

	t.Run("ExecuteStreaming with large output", func(t *testing.T) {
		req := &filesystem.ExecuteRequest{Command: "seq 1 100"}
		sr, err := s.(*backend).ExecuteStreaming(ctx, req)
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
		sr, err := s.(*backend).ExecuteStreaming(ctx, req)
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
		sr, err := s.(*backend).ExecuteStreaming(ctx, req)
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
}

func TestExecute1(t *testing.T) {
	fs := strings.Fields("ls -al")
	bs, err := exec.Command(fs[0], fs[1:]...).Output()
	assert.NoError(t, err)
	fmt.Println(string(bs))
}
