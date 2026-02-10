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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/schema"
)

const defaultRootPath = "/"

type Config struct {
	ValidateCommand func(string) error
}

type backend struct {
	validateCommand func(string) error
}

var defaultValidateCommand = func(string) error {
	return nil
}

// NewBackend creates a new local filesystem backend instance.
//
// IMPORTANT - System Compatibility:
//   - Supported: Unix/MacOS only
//   - NOT Supported: Windows (requires custom implementation of Backend)
//   - Command Execution: Uses /bin/sh by default for Execute method
//   - If /bin/sh does not meet your requirements, please implement your own Backend
func NewBackend(_ context.Context, cfg *Config) (filesystem.Backend, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	validateCommand := defaultValidateCommand
	if cfg.ValidateCommand != nil {
		validateCommand = cfg.ValidateCommand
	}

	return &backend{
		validateCommand: validateCommand,
	}, nil
}

func (s *backend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	if req.Path == "" {
		req.Path = defaultRootPath
	}

	path := filepath.Clean(req.Path)
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path must be an absolute path: %s", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied: %s", path)
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var files []filesystem.FileInfo
	for _, entry := range entries {
		files = append(files, filesystem.FileInfo{
			Path: entry.Name(),
		})
	}

	return files, nil
}

func (s *backend) Read(ctx context.Context, req *filesystem.ReadRequest) (string, error) {
	path := filepath.Clean(req.FilePath)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be an absolute path: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	if info.Size() == 0 {
		return "", nil
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 200
	}

	scanner := bufio.NewScanner(file)
	var result strings.Builder
	lineIdx := 0
	linesRead := 0

	for scanner.Scan() {
		if lineIdx >= offset {
			result.WriteString(fmt.Sprintf("%6d\t%s\n", lineIdx+1, scanner.Text()))
			linesRead++
			if linesRead >= limit {
				break
			}
		}
		lineIdx++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return result.String(), nil
}

func (s *backend) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	path := filepath.Clean(req.Path)

	var matches []filesystem.GrepMatch

	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			if os.IsPermission(err) {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() {
			return nil
		}

		if req.Glob != "" {
			matched, err := filepath.Match(req.Glob, d.Name())
			if err != nil {
				return err
			}
			if !matched {
				return nil
			}
		}

		file, err := os.Open(p)
		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			return fmt.Errorf("failed to open file %s: %w", p, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNumber := 1
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if strings.Contains(scanner.Text(), req.Pattern) {
				matches = append(matches, filesystem.GrepMatch{
					Path:    p,
					Line:    lineNumber,
					Content: scanner.Text(),
				})
			}
			lineNumber++
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to scan file %s: %w", p, err)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error during grep operation: %w", err)
	}

	return matches, nil
}

func (s *backend) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	if req.Path == "" {
		req.Path = defaultRootPath
	}
	path := filepath.Clean(req.Path)

	regex, err := globToRegex(req.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	var matches []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			if os.IsPermission(err) {
				return filepath.SkipDir
			}
			return err
		}

		relPath, err := filepath.Rel(path, p)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		relPath = filepath.ToSlash(relPath)

		if relPath == "." {
			return nil
		}

		if regex.MatchString(relPath) {
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	sort.Strings(matches)

	var files []filesystem.FileInfo
	for _, match := range matches {
		files = append(files, filesystem.FileInfo{
			Path: match,
		})
	}

	return files, nil
}

func globToRegex(pattern string) (*regexp.Regexp, error) {
	// Normalize path separators to slash for regex matching
	pattern = filepath.ToSlash(pattern)

	// 1. Quote meta characters to treat them literally by default
	// pattern: a/*.txt -> a/\*\.txt
	pattern = regexp.QuoteMeta(pattern)

	// 2. Replace escaped glob wildcards with regex equivalents
	// order matters: ** first, then *

	// ** -> .*
	// But QuoteMeta turned ** into \*\*
	// To match python glob(recursive=True), ** should match zero or more directories
	// AND the file itself if it's in the current directory.
	// So we handle **/ specifically.
	// We replace \*\*/ with (.*\/)? to match optional directory prefix.
	// Note: We need to handle both / and \ as separators in case QuoteMeta didn't escape /
	// but on Windows path separator is \ which is escaped.
	// However, we normalized everything to / in GlobInfo before matching.
	pattern = strings.ReplaceAll(pattern, "\\*\\*/", "(.*\\/)?")
	pattern = strings.ReplaceAll(pattern, "\\*\\*", ".*")

	// * -> [^/]*
	// QuoteMeta turned * into \*
	pattern = strings.ReplaceAll(pattern, "\\*", "[^/]*")

	// ? -> .
	// QuoteMeta turned ? into \?
	pattern = strings.ReplaceAll(pattern, "\\?", ".")

	// 4. Handle brackets [abc]
	// QuoteMeta escapes [ and ], so we need to unescape them
	// [ -> \[ -> [
	// ] -> \] -> ]
	// This restores the regex character class functionality
	pattern = strings.ReplaceAll(pattern, "\\[", "[")
	pattern = strings.ReplaceAll(pattern, "\\]", "]")

	// 5. Anchor the regex
	pattern = "^" + pattern + "$"

	return regexp.Compile(pattern)
}

func (s *backend) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	if !filepath.IsAbs(req.FilePath) {
		return fmt.Errorf("path must be an absolute path: %s", req.FilePath)
	}

	parentDir := filepath.Dir(req.FilePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	file, err := os.OpenFile(req.FilePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("file '%s' already exists", req.FilePath)
		}
		return fmt.Errorf("failed to open file for writing: %w", err)
	}
	defer file.Close()

	_, err = file.Write([]byte(req.Content))
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func (s *backend) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	path := filepath.Clean(req.FilePath)
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be an absolute path: %s", path)
	}

	if req.OldString == "" {
		return fmt.Errorf("old string is required")
	}

	if req.OldString == req.NewString {
		return fmt.Errorf("new string must be different from old string")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	text := string(content)
	count := strings.Count(text, req.OldString)

	if count == 0 {
		return fmt.Errorf("string not found in file: '%s'", req.OldString)
	}
	if count > 1 && !req.ReplaceAll {
		return fmt.Errorf("string '%s' appears multiple times. Use replace_all=True to replace all occurrences", req.OldString)
	}

	var newText string
	if req.ReplaceAll {
		newText = strings.Replace(text, req.OldString, req.NewString, -1)
	} else {
		newText = strings.Replace(text, req.OldString, req.NewString, 1)
	}

	return os.WriteFile(path, []byte(newText), 0644)
}

func (s *backend) ExecuteStreaming(ctx context.Context, input *filesystem.ExecuteRequest) (result *schema.StreamReader[*filesystem.ExecuteResponse], err error) {
	if input.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// SECURITY WARNING: Command Injection Risk
	// Similar to Execute method, proper validation is critical here.
	// The validateCommand function MUST sanitize input.Command to prevent injection attacks.
	// See Execute method for detailed security recommendations.
	if err := s.validateCommand(input.Command); err != nil {
		return nil, err
	}

	// WARNING: Using "/bin/sh -c" allows shell interpretation of the command string,
	// which enables command injection if input.Command is not properly validated above.
	// Consider using exec.Command(command, args...) directly without shell if possible.
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", input.Command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	sr, w := schema.Pipe[*filesystem.ExecuteResponse](100)

	go func() {
		defer func() {
			if pe := recover(); pe != nil {
				w.Send(nil, newPanicErr(pe, debug.Stack()))
				return
			}
			w.Close()
		}()

		var stderrData []byte
		stderrErr := make(chan error, 1)
		go func() {
			defer func() {
				if pe := recover(); pe != nil {
					stderrErr <- newPanicErr(pe, debug.Stack())
					return
				}
				close(stderrErr)
			}()
			var err error
			stderrData, err = io.ReadAll(stderr)
			if err != nil {
				stderrErr <- fmt.Errorf("failed to read stderr: %w", err)
			}
		}()

		scanner := bufio.NewScanner(stdout)
		hasOutput := false
		for scanner.Scan() {
			hasOutput = true
			line := scanner.Text() + "\n"
			select {
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				w.Send(nil, ctx.Err())
				return
			default:
				w.Send(&filesystem.ExecuteResponse{Output: line}, nil)
			}
		}

		if err := scanner.Err(); err != nil {
			w.Send(nil, fmt.Errorf("error reading stdout: %w", err))
			return
		}

		if err := <-stderrErr; err != nil {
			w.Send(nil, err)
			return
		}

		if err := cmd.Wait(); err != nil {
			exitCode := 0
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				exitCode = exitError.ExitCode()
			}
			if len(stderrData) > 0 {
				w.Send(nil, fmt.Errorf("command exited with non-zero code %d: %s", exitCode, string(stderrData)))
			} else {
				w.Send(nil, fmt.Errorf("command exited with non-zero code %d", exitCode))
			}
			return
		}

		if !hasOutput {
			w.Send(&filesystem.ExecuteResponse{ExitCode: new(int)}, nil)
		}

	}()

	return sr, nil
}

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}
