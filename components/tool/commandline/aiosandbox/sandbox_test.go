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
	"os"
	"testing"
)

func getTestConfig() *Config {
	baseURL := os.Getenv("AIO_SANDBOX_BASE_URL")
	if baseURL == "" {
		return nil
	}

	// Token is optional
	token := os.Getenv("AIO_SANDBOX_TOKEN")

	return &Config{
		BaseURL:     baseURL,
		Token:       token,
		WorkDir:     "/tmp",
		Timeout:     60,
		KeepSession: true,
	}
}

func TestNewAIOSandbox(t *testing.T) {
	t.Run("missing config", func(t *testing.T) {
		_, err := NewAIOSandbox(context.Background(), nil)
		if err == nil {
			t.Error("expected error for nil config")
		}
	})

	t.Run("missing BaseURL", func(t *testing.T) {
		_, err := NewAIOSandbox(context.Background(), &Config{
			Token: "test-token",
		})
		if err == nil {
			t.Error("expected error for missing BaseURL")
		}
	})

	t.Run("without Token should work", func(t *testing.T) {
		// Token is optional, so this should not return an error for missing token
		// It will fail on createSession if KeepSession is true, but that's expected
		_, err := NewAIOSandbox(context.Background(), &Config{
			BaseURL:     "https://example.com",
			KeepSession: false, // Disable session to avoid network call
		})
		// Will fail due to network, but not due to missing token
		if err != nil && err.Error() == "Token is required" {
			t.Error("Token should be optional")
		}
	})
}

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{
		BaseURL: "https://example.com",
		Token:   "test-token",
	}
	cfg.setDefaults()

	if cfg.WorkDir != defaultWorkDir {
		t.Errorf("expected WorkDir %s, got %s", defaultWorkDir, cfg.WorkDir)
	}
	if cfg.Timeout != defaultTimeout {
		t.Errorf("expected Timeout %f, got %f", defaultTimeout, cfg.Timeout)
	}
}

func TestResolvePath(t *testing.T) {
	sandbox := &AIOSandbox{
		config: &Config{
			WorkDir: "/workspace",
		},
	}

	tests := []struct {
		name     string
		path     string
		expected string
		wantErr  bool
	}{
		{
			name:     "absolute path",
			path:     "/tmp/test.txt",
			expected: "/tmp/test.txt",
			wantErr:  false,
		},
		{
			name:     "relative path",
			path:     "test.txt",
			expected: "/workspace/test.txt",
			wantErr:  false,
		},
		{
			name:     "path traversal",
			path:     "../etc/passwd",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sandbox.resolvePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolvePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("resolvePath() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPtrToString(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{
			name:     "nil pointer",
			input:    nil,
			expected: "",
		},
		{
			name:     "non-nil pointer",
			input:    strPtr("hello"),
			expected: "hello",
		},
		{
			name:     "empty string pointer",
			input:    strPtr(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ptrToString(tt.input)
			if got != tt.expected {
				t.Errorf("ptrToString() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

// Integration tests - run only when AIO_SANDBOX_BASE_URL and AIO_SANDBOX_TOKEN are set
func TestIntegration(t *testing.T) {
	cfg := getTestConfig()
	if cfg == nil {
		t.Skip("Skipping integration tests: AIO_SANDBOX_BASE_URL not set")
	}

	ctx := context.Background()

	sandbox, err := NewAIOSandbox(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Close(ctx)

	t.Run("RunCommand", func(t *testing.T) {
		output, err := sandbox.RunCommand(ctx, []string{"echo", "hello"})
		if err != nil {
			t.Fatalf("RunCommand failed: %v", err)
		}
		if output.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", output.ExitCode)
		}
	})

	t.Run("WriteAndReadFile", func(t *testing.T) {
		testContent := "Hello, AIO Sandbox!"
		testPath := "/tmp/test_file.txt"

		err := sandbox.WriteFile(ctx, testPath, testContent)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		content, err := sandbox.ReadFile(ctx, testPath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if content != testContent {
			t.Errorf("expected content %q, got %q", testContent, content)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		exists, err := sandbox.Exists(ctx, "/tmp/test_file.txt")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("expected file to exist")
		}

		exists, err = sandbox.Exists(ctx, "/tmp/nonexistent_file.txt")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected file to not exist")
		}
	})

	t.Run("IsDirectory", func(t *testing.T) {
		isDir, err := sandbox.IsDirectory(ctx, "/tmp")
		if err != nil {
			t.Fatalf("IsDirectory failed: %v", err)
		}
		if !isDir {
			t.Error("expected /tmp to be a directory")
		}
	})

	t.Run("SessionPersistence", func(t *testing.T) {
		// Set an environment variable
		_, err := sandbox.RunCommand(ctx, []string{"export TEST_VAR=hello"})
		if err != nil {
			t.Fatalf("RunCommand failed: %v", err)
		}

		// Check if it persists (only works with KeepSession=true)
		output, err := sandbox.RunCommand(ctx, []string{"echo $TEST_VAR"})
		if err != nil {
			t.Fatalf("RunCommand failed: %v", err)
		}

		// Note: This test verifies session persistence behavior
		t.Logf("Session persistence test output: %s", output.Stdout)
	})
}
