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

package aiosandbox

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cloudwego/eino-ext/components/tool/commandline"

	sandboxsdk "github.com/agent-infra/sandbox-sdk-go"
	"github.com/agent-infra/sandbox-sdk-go/client"
	"github.com/agent-infra/sandbox-sdk-go/option"
)

const (
	defaultWorkDir = "/tmp"
	defaultTimeout = 60.0
)

// Config defines the configuration for AIO Sandbox.
type Config struct {
	// BaseURL is the AIO Sandbox API endpoint.
	// Required.
	BaseURL string

	// Token is the authentication token for AIO Sandbox API.
	// Optional. If not provided, requests will be made without authentication.
	Token string

	// WorkDir is the working directory inside the sandbox.
	// Default: "/tmp"
	WorkDir string

	// Timeout is the default command execution timeout in seconds.
	// Default: 60
	Timeout float64

	// KeepSession indicates whether to reuse shell sessions for stateful execution.
	// When enabled, environment variables and working directory changes persist across commands.
	// Default: true
	KeepSession bool
}

func (c *Config) setDefaults() {
	if c.WorkDir == "" {
		c.WorkDir = defaultWorkDir
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
}

// AIOSandbox implements commandline.Operator interface using AIO Sandbox API.
// It provides a remote sandboxed environment for executing commands and file operations.
type AIOSandbox struct {
	config    *Config
	client    *client.Client
	sessionID string
	mu        sync.RWMutex
}

// NewAIOSandbox creates a new AIO Sandbox operator.
//
// Example:
//
//	sandbox, err := aiosandbox.NewAIOSandbox(ctx, &aiosandbox.Config{
//	    BaseURL:     "https://api.aio-sandbox.com",
//	    Token:       "your-api-token",
//	    WorkDir:     "/workspace",
//	    Timeout:     120,
//	    KeepSession: true,
//	})
func NewAIOSandbox(ctx context.Context, config *Config) (*AIOSandbox, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.BaseURL == "" {
		return nil, fmt.Errorf("BaseURL is required")
	}

	// Copy config to avoid external mutation
	cfg := *config
	cfg.setDefaults()

	// Parse the base URL to extract query parameters
	parsedURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid BaseURL: %w", err)
	}

	// Build base URL without query parameters
	baseURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)

	opts := []option.RequestOption{
		option.WithBaseURL(baseURL),
	}

	// Add query parameters if present in the original URL
	if len(parsedURL.RawQuery) > 0 {
		opts = append(opts, option.WithQueryParameters(parsedURL.Query()))
	}

	// Set up authentication header if token is provided
	if cfg.Token != "" {
		authHeader := http.Header{}
		authHeader.Set("Authorization", "Bearer "+cfg.Token)
		opts = append(opts, option.WithHTTPHeader(authHeader))
	}

	c := client.NewClient(opts...)

	sandbox := &AIOSandbox{
		config: &cfg,
		client: c,
	}

	// Create initial session if KeepSession is enabled
	if cfg.KeepSession {
		if err := sandbox.createSession(ctx); err != nil {
			return nil, fmt.Errorf("failed to create initial session: %w", err)
		}
	}

	return sandbox, nil
}

// createSession creates a new shell session.
func (s *AIOSandbox) createSession(ctx context.Context) error {
	resp, err := s.client.Shell.CreateSession(ctx, &sandboxsdk.ShellCreateSessionRequest{
		ExecDir: sandboxsdk.String(s.config.WorkDir),
	})
	if err != nil {
		return err
	}

	data := resp.GetData()
	if data == nil {
		return fmt.Errorf("empty response data")
	}

	s.mu.Lock()
	s.sessionID = data.GetSessionId()
	s.mu.Unlock()
	return nil
}

// RunCommand executes a command in the sandbox.
// Implements commandline.Operator interface.
func (s *AIOSandbox) RunCommand(ctx context.Context, command []string) (*commandline.CommandOutput, error) {
	cmd := strings.Join(command, " ")
	if cmd == "" {
		return nil, fmt.Errorf("command is empty")
	}

	req := &sandboxsdk.ShellExecRequest{
		Command: cmd,
		ExecDir: sandboxsdk.String(s.config.WorkDir),
		Timeout: sandboxsdk.Float64(s.config.Timeout),
	}

	// Use existing session if available
	s.mu.RLock()
	sessionID := s.sessionID
	s.mu.RUnlock()

	if s.config.KeepSession && sessionID != "" {
		req.Id = sandboxsdk.String(sessionID)
	}

	resp, err := s.client.Shell.ExecCommand(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("shell exec failed: %w", err)
	}

	data := resp.GetData()
	if data == nil {
		return nil, fmt.Errorf("empty response data")
	}

	// Update session ID if changed
	if s.config.KeepSession && data.GetSessionId() != "" {
		s.mu.Lock()
		s.sessionID = data.GetSessionId()
		s.mu.Unlock()
	}

	output := &commandline.CommandOutput{
		Stdout: ptrToString(data.GetOutput()),
	}

	// Map status to exit code and stderr
	status := data.GetStatus()
	switch status {
	case sandboxsdk.BashCommandStatusCompleted:
		if data.GetExitCode() != nil {
			output.ExitCode = *data.GetExitCode()
		}
	case sandboxsdk.BashCommandStatusHardTimeout, sandboxsdk.BashCommandStatusNoChangeTimeout:
		output.ExitCode = 124 // standard timeout exit code
		output.Stderr = fmt.Sprintf("command timeout: %s", status)
	case sandboxsdk.BashCommandStatusTerminated:
		output.ExitCode = 137 // 128 + SIGKILL(9)
		output.Stderr = "command was terminated"
	case sandboxsdk.BashCommandStatusRunning:
		// Command is still running (async mode)
		output.Stderr = "command is still running"
	default:
		output.Stderr = fmt.Sprintf("unexpected status: %s", status)
	}

	return output, nil
}

// ReadFile reads file content from the sandbox.
// Implements commandline.Operator interface.
func (s *AIOSandbox) ReadFile(ctx context.Context, path string) (string, error) {
	resolvedPath, err := s.resolvePath(path)
	if err != nil {
		return "", err
	}

	resp, err := s.client.File.ReadFile(ctx, &sandboxsdk.FileReadRequest{
		File: resolvedPath,
	})
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}

	data := resp.GetData()
	if data == nil {
		return "", fmt.Errorf("empty response data")
	}

	return data.GetContent(), nil
}

// WriteFile writes content to a file in the sandbox.
// Implements commandline.Operator interface.
func (s *AIOSandbox) WriteFile(ctx context.Context, path string, content string) error {
	resolvedPath, err := s.resolvePath(path)
	if err != nil {
		return err
	}

	_, err = s.client.File.WriteFile(ctx, &sandboxsdk.FileWriteRequest{
		File:    resolvedPath,
		Content: content,
	})
	if err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	return nil
}

// IsDirectory checks if the path is a directory.
// Implements commandline.Operator interface.
func (s *AIOSandbox) IsDirectory(ctx context.Context, path string) (bool, error) {
	resolvedPath, err := s.resolvePath(path)
	if err != nil {
		return false, err
	}

	resp, err := s.client.File.ListPath(ctx, &sandboxsdk.FileListRequest{
		Path:      resolvedPath,
		Recursive: sandboxsdk.Bool(false),
	})
	if err != nil {
		// Path doesn't exist or is not a directory
		return false, nil
	}

	// If we can list it without error, it's a directory
	return resp.GetData() != nil, nil
}

// Exists checks if the path exists.
// Implements commandline.Operator interface.
func (s *AIOSandbox) Exists(ctx context.Context, path string) (bool, error) {
	resolvedPath, err := s.resolvePath(path)
	if err != nil {
		return false, err
	}

	// Try to read file info
	_, err = s.client.File.ReadFile(ctx, &sandboxsdk.FileReadRequest{
		File:      resolvedPath,
		StartLine: sandboxsdk.Int(0),
		EndLine:   sandboxsdk.Int(1),
	})
	if err == nil {
		return true, nil
	}

	// Check if it's a directory
	isDir, _ := s.IsDirectory(ctx, path)
	return isDir, nil
}

// Close terminates the shell session and releases resources.
func (s *AIOSandbox) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sessionID != "" {
		// Kill any running processes in the session
		_, _ = s.client.Shell.KillProcess(ctx, &sandboxsdk.ShellKillProcessRequest{
			Id: s.sessionID,
		})
		s.sessionID = ""
	}

	return nil
}

// SetWorkDir updates the working directory for subsequent operations.
func (s *AIOSandbox) SetWorkDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.WorkDir = dir
}

// GetSessionID returns the current shell session ID.
func (s *AIOSandbox) GetSessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID
}

// resolvePath safely resolves a path, preventing path traversal attacks.
func (s *AIOSandbox) resolvePath(path string) (string, error) {
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains potentially unsafe pattern")
	}

	if filepath.IsAbs(path) {
		return path, nil
	}

	return filepath.Join(s.config.WorkDir, path), nil
}

// ptrToString safely converts a string pointer to string.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Compile-time check that AIOSandbox implements Operator interface
var _ commandline.Operator = (*AIOSandbox)(nil)
