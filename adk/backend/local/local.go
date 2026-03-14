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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/schema"
)

const defaultRootPath = "/"

type Config struct {
	ValidateCommand func(string) error
}

type Local struct {
	validateCommand func(string) error
}

var defaultValidateCommand = func(string) error {
	return nil
}

// NewBackend creates a new local filesystem Local instance.
//
// IMPORTANT - System Compatibility:
//   - Supported: Unix/MacOS only
//   - NOT Supported: Windows (requires custom implementation of filesystem.Backend)
//   - Command Execution: Uses /bin/sh by default for Execute method
//   - If /bin/sh does not meet your requirements, please implement your own filesystem.Backend
func NewBackend(_ context.Context, cfg *Config) (*Local, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	validateCommand := defaultValidateCommand
	if cfg.ValidateCommand != nil {
		validateCommand = cfg.ValidateCommand
	}

	return &Local{
		validateCommand: validateCommand,
	}, nil
}

func (s *Local) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	path := filepath.Clean(req.Path)
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

	var files []filesystem.FileInfo
	for _, entry := range entries {
		files = append(files, filesystem.FileInfo{
			Path: entry.Name(),
		})
	}

	return files, nil
}

func (s *Local) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	path := filepath.Clean(req.FilePath)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.Size() == 0 {
		return &filesystem.FileContent{}, nil
	}

	offset := req.Offset
	if offset <= 0 {
		offset = 1
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 2000
	}

	reader := bufio.NewReader(file)
	var result strings.Builder
	lineNum := 1
	linesRead := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if line != "" {
			if lineNum >= offset {
				result.WriteString(line)
				linesRead++
				if linesRead >= limit {
					break
				}
			}
			lineNum++
		}
		if err != nil {
			if err != io.EOF {
				return nil, fmt.Errorf("error reading file: %w", err)
			}
			break
		}
	}

	return &filesystem.FileContent{
		Content: strings.TrimSuffix(result.String(), "\n"),
	}, nil
}

type rgJSON struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		LineNumber int `json:"line_number"`
		Lines      struct {
			Text string `json:"text"`
		} `json:"lines"`
	} `json:"data"`
}

func (s *Local) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	if req.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	path := filepath.Clean(req.Path)

	cmd := []string{"rg", "--json"}
	if req.CaseInsensitive {
		cmd = append(cmd, "-i")
	}
	if req.EnableMultiline {
		cmd = append(cmd, "-U", "--multiline-dotall")
	}
	if req.FileType != "" {
		cmd = append(cmd, "--type", req.FileType)
	} else if req.Glob != "" {
		cmd = append(cmd, "--glob", req.Glob)
	}
	if req.AfterLines > 0 {
		cmd = append(cmd, "-A", fmt.Sprintf("%d", req.AfterLines))
	}
	if req.BeforeLines > 0 {
		cmd = append(cmd, "-B", fmt.Sprintf("%d", req.BeforeLines))
	}

	cmd = append(cmd, "-e", req.Pattern, "--", path)

	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	output, err := execCmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("ripgrep (rg) is not installed or not in PATH. Please install it: https://github.com/BurntSushi/ripgrep#installation")
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return []filesystem.GrepMatch{}, nil
			}
			return nil, fmt.Errorf("ripgrep failed with exit code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute ripgrep: %w", err)
	}

	var matches []filesystem.GrepMatch
	if len(output) == 0 {
		return matches, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var data rgJSON
	for _, line := range lines {
		data = rgJSON{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		if data.Type == "match" || data.Type == "context" {
			matchPath := data.Data.Path.Text
			if req.FileType != "" && req.Glob != "" {
				matched, _ := doublestar.Match(req.Glob, matchPath)
				if !matched {
					matched, _ = doublestar.Match(req.Glob, filepath.Base(matchPath))
				}
				if !matched {
					continue
				}
			}
			matches = append(matches, filesystem.GrepMatch{
				Path:    matchPath,
				Line:    data.Data.LineNumber,
				Content: strings.TrimRight(data.Data.Lines.Text, "\n"),
			})
		}
	}

	return matches, nil
}

func (s *Local) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	if req.Path == "" {
		req.Path = defaultRootPath
	}
	path := filepath.Clean(req.Path)

	var matches []string
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

		relPath, err := filepath.Rel(path, p)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		relPath = filepath.ToSlash(relPath)

		if relPath == "." {
			return nil
		}

		matched, _ := doublestar.Match(req.Pattern, relPath)
		if matched {
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

func (s *Local) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	path := filepath.Clean(req.FilePath)

	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for writing: %w", err)
	}
	defer file.Close()

	_, err = file.Write([]byte(req.Content))
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func (s *Local) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	path := filepath.Clean(req.FilePath)
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

func (s *Local) ExecuteStreaming(ctx context.Context, input *filesystem.ExecuteRequest) (result *schema.StreamReader[*filesystem.ExecuteResponse], err error) {
	if input.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	if err := s.validateCommand(input.Command); err != nil {
		return nil, err
	}

	cmd, stdout, stderr, err := s.initStreamingCmd(ctx, input.Command)
	if err != nil {
		return nil, err
	}

	sr, w := schema.Pipe[*filesystem.ExecuteResponse](100)

	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		go sendErrorAndClose(w, fmt.Errorf("failed to start command: %w", err))
		return sr, nil
	}

	if input.RunInBackendGround {
		s.runCmdInBackground(ctx, cmd, stdout, stderr, w)
		return sr, nil
	}

	go s.streamCmdOutput(ctx, cmd, stdout, stderr, w)

	return sr, nil
}

// initStreamingCmd creates command with stdout and stderr pipes.
func (s *Local) initStreamingCmd(ctx context.Context, command string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	return cmd, stdout, stderr, nil
}

// runCmdInBackground executes command in background without waiting for completion.
// The caller controls timeout/cancellation via ctx.Done().
func (s *Local) runCmdInBackground(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.ReadCloser, w *schema.StreamWriter[*filesystem.ExecuteResponse]) {
	go func() {
		defer func() {
			if pe := recover(); pe != nil {
				_ = cmd.Process.Kill()
			}
			_ = stdout.Close()
			_ = stderr.Close()
		}()

		done := make(chan struct{})
		go func() {
			drainPipesConcurrently(stdout, stderr)
			_ = cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			_ = cmd.Process.Kill()
		}
	}()

	go func() {
		defer w.Close()
		w.Send(&filesystem.ExecuteResponse{Output: "command started in background\n", ExitCode: new(int)}, nil)
	}()
}

// drainPipesConcurrently consumes stdout and stderr concurrently to prevent pipe blocking.
func drainPipesConcurrently(stdout, stderr io.Reader) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.Discard, stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.Discard, stderr)
	}()
	wg.Wait()
}

// streamCmdOutput handles streaming command output to the writer.
func (s *Local) streamCmdOutput(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.ReadCloser, w *schema.StreamWriter[*filesystem.ExecuteResponse]) {
	defer func() {
		if pe := recover(); pe != nil {
			w.Send(nil, newPanicErr(pe, debug.Stack()))
			return
		}
		w.Close()
	}()

	stderrData, stderrErr := s.readStderrAsync(stderr)

	hasOutput, err := s.streamStdout(ctx, cmd, stdout, w)
	if err != nil {
		w.Send(nil, err)
		return
	}

	if stdError := <-stderrErr; stdError != nil {
		w.Send(nil, stdError)
		return
	}

	s.handleCmdCompletion(cmd, stderrData, hasOutput, w)
}

// readStderrAsync reads stderr in a separate goroutine.
func (s *Local) readStderrAsync(stderr io.Reader) (*[]byte, <-chan error) {
	stderrData := new([]byte)
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
		*stderrData, err = io.ReadAll(stderr)
		if err != nil {
			stderrErr <- fmt.Errorf("failed to read stderr: %w", err)
		}
	}()

	return stderrData, stderrErr
}

// streamStdout streams stdout line by line to the writer.
func (s *Local) streamStdout(ctx context.Context, cmd *exec.Cmd, stdout io.Reader, w *schema.StreamWriter[*filesystem.ExecuteResponse]) (bool, error) {
	scanner := bufio.NewScanner(stdout)
	hasOutput := false

	for scanner.Scan() {
		hasOutput = true
		line := scanner.Text() + "\n"
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return hasOutput, ctx.Err()
		default:
			w.Send(&filesystem.ExecuteResponse{Output: line}, nil)
		}
	}

	if err := scanner.Err(); err != nil {
		return hasOutput, fmt.Errorf("error reading stdout: %w", err)
	}

	return hasOutput, nil
}

// handleCmdCompletion handles command completion and sends final response.
func (s *Local) handleCmdCompletion(cmd *exec.Cmd, stderrData *[]byte, hasOutput bool, w *schema.StreamWriter[*filesystem.ExecuteResponse]) {
	if err := cmd.Wait(); err != nil {
		exitCode := 0
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}
		if len(*stderrData) > 0 {
			w.Send(nil, fmt.Errorf("command exited with non-zero code %d: %s", exitCode, string(*stderrData)))
			return
		}
		w.Send(nil, fmt.Errorf("command exited with non-zero code %d", exitCode))
		return
	}

	if !hasOutput {
		w.Send(&filesystem.ExecuteResponse{ExitCode: new(int)}, nil)
	}
}

// sendErrorAndClose sends an error to the stream and closes it.
func sendErrorAndClose(w *schema.StreamWriter[*filesystem.ExecuteResponse], err error) {
	defer w.Close()
	w.Send(nil, err)
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
