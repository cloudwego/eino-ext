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

package agentkit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewArkSandbox tests the constructor for ArkSandbox.
func TestNewArkSandbox(t *testing.T) {
	t.Run("Success: ValidConfig", func(t *testing.T) {
		config := &Config{
			AccessKeyID:      "test-ak",
			SecretAccessKey:  "test-sk",
			ToolID:           "test-tool",
			UserSessionID:    "test-session",
			Region:           RegionOfBeijing,
			SessionTTL:       3600,
			ExecutionTimeout: 60,
		}
		ss, err := NewSandboxToolBackend(config)
		s := ss.(*sandboxToolBackend)
		require.NoError(t, err)
		require.NotNil(t, s)
		assert.Equal(t, "test-ak", s.accessKeyID)
		assert.Equal(t, "test-sk", s.secretAccessKey)
		assert.Equal(t, "test-tool", s.toolID)
		assert.Equal(t, "test-session", s.userSessionID)
		assert.Equal(t, RegionOfBeijing, s.region)
		assert.Equal(t, regionOfBeijingBaseURL, s.baseURL)
		assert.Equal(t, 3600, s.sessionTTL)
		assert.Equal(t, 60, s.executionTimeout)
	})

	t.Run("Success: Defaults", func(t *testing.T) {
		config := &Config{
			AccessKeyID:     "test-ak",
			SecretAccessKey: "test-sk",
			ToolID:          "test-tool",
			UserSessionID:   "test-session",
		}
		ss, err := NewSandboxToolBackend(config)
		s := ss.(*sandboxToolBackend)
		require.NoError(t, err)
		require.NotNil(t, s)
		assert.Equal(t, RegionOfBeijing, s.region)
		assert.Equal(t, regionOfBeijingBaseURL, s.baseURL)
		assert.Equal(t, 0, s.sessionTTL)
		assert.Equal(t, 0, s.executionTimeout)
	})

	t.Run("Failure: MissingRequiredFields", func(t *testing.T) {
		baseConfig := &Config{
			AccessKeyID:     "test-ak",
			SecretAccessKey: "test-sk",
			ToolID:          "test-tool",
			UserSessionID:   "test-session",
		}
		testCases := []struct {
			name        string
			config      *Config
			expectedErr string
		}{
			{"MissingAccessKey", &Config{SecretAccessKey: baseConfig.SecretAccessKey, ToolID: baseConfig.ToolID, UserSessionID: baseConfig.UserSessionID}, "AccessKeyID is required"},
			{"MissingSecretKey", &Config{AccessKeyID: baseConfig.AccessKeyID, ToolID: baseConfig.ToolID, UserSessionID: baseConfig.UserSessionID}, "SecretAccessKey is required"},
			{"MissingToolID", &Config{AccessKeyID: baseConfig.AccessKeyID, SecretAccessKey: baseConfig.SecretAccessKey, UserSessionID: baseConfig.UserSessionID}, "ToolID is required"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := NewSandboxToolBackend(tc.config)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			})
		}
	})

	t.Run("Failure: InvalidRegion", func(t *testing.T) {
		config := &Config{
			AccessKeyID:     "test-ak",
			SecretAccessKey: "test-sk",
			ToolID:          "test-tool",
			UserSessionID:   "test-session",
			Region:          "invalid-region",
		}
		_, err := NewSandboxToolBackend(config)
		require.Error(t, err)
		assert.Equal(t, "invalid region: invalid-region", err.Error())
	})
}

// mockAPIHandler is a mutable handler for the mock server.
var mockAPIHandler http.HandlerFunc

// setupTest creates a mock server and an ArkSandbox client configured to use it.
func setupTest(t *testing.T) (*sandboxToolBackend, *httptest.Server) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mockAPIHandler != nil {
			mockAPIHandler(w, r)
		} else {
			http.Error(w, "mockAPIHandler not set", http.StatusInternalServerError)
		}
	}))

	config := &Config{
		AccessKeyID:     "test-ak",
		SecretAccessKey: "test-sk",
		ToolID:          "test-tool",
		UserSessionID:   "test-session",
		HTTPClient:      server.Client(),
	}
	ss, err := NewSandboxToolBackend(config)
	sandbox := ss.(*sandboxToolBackend)
	require.NoError(t, err)
	sandbox.baseURL = server.URL // Override to point to the mock server

	return sandbox, server
}

// createMockResponse is a helper to build a valid JSON response for the mock API.
func createMockResponse(t *testing.T, success bool, outputText, eName, eValue string) []byte {
	type mockOutputData struct {
		Text   string `json:"text"`
		EName  string `json:"ename"`
		EValue string `json:"evalue"`
	}
	type mockResultData struct {
		Outputs []mockOutputData `json:"outputs"`
	}
	type mockResult struct {
		Success bool           `json:"success"`
		Data    mockResultData `json:"data"`
	}

	resData := mockResult{
		Success: success,
		Data: mockResultData{
			Outputs: []mockOutputData{
				{Text: outputText, EName: eName, EValue: eValue},
			},
		},
	}
	resDataBytes, err := json.Marshal(resData)
	require.NoError(t, err)

	finalRes := response{
		Result: struct {
			Result string `json:"result"`
		}{Result: string(resDataBytes)},
	}
	finalResBytes, err := json.Marshal(finalRes)
	require.NoError(t, err)

	return finalResBytes
}

func TestArkSandbox_FileSystemMethods(t *testing.T) {
	s, server := setupTest(t)
	defer server.Close()

	// LsInfo Tests
	t.Run("LsInfo: Success", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			lsOutput := `{"path": "file1.txt", "is_dir": false}` + "\n" + `{"path": "dir1", "is_dir": true}`
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, lsOutput, "", ""))
		}
		res, err := s.LsInfo(context.Background(), &filesystem.LsInfoRequest{Path: "/data"})
		require.NoError(t, err)
		require.Len(t, res, 2)
		assert.Equal(t, "file1.txt", res[0].Path)
		assert.Equal(t, "dir1", res[1].Path)
	})

	t.Run("LsInfo: Failure - Script Error", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, false, "Permission denied", "PermissionError", "permission denied"))
		}
		_, err := s.LsInfo(context.Background(), &filesystem.LsInfoRequest{Path: "/root"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ls script exited with non-zero code -1: Permission denied")
	})

	t.Run("LsInfo: Failure - Invalid Path", func(t *testing.T) {
		_, err := s.LsInfo(context.Background(), &filesystem.LsInfoRequest{Path: "relative/path"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path must be an absolute path")
	})

	// Read Tests
	t.Run("Read: Success", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, "hello world", "", ""))
		}
		res, err := s.Read(context.Background(), &filesystem.ReadRequest{FilePath: "/data/file.txt"})
		require.NoError(t, err)
		assert.Equal(t, "hello world", res)
	})

	t.Run("Read: Failure - API Error", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		_, err := s.Read(context.Background(), &filesystem.ReadRequest{FilePath: "/data/file.txt"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status code 500")
	})

	// GrepRaw Tests
	t.Run("GrepRaw: Success", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			grepOutput := `{"Path": "/data/file.txt", "Line": 1, "Content": "hello world"}`
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, grepOutput, "", ""))
		}
		res, err := s.GrepRaw(context.Background(), &filesystem.GrepRequest{Pattern: "hello"})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "/data/file.txt", res[0].Path)
		assert.Equal(t, 1, res[0].Line)
	})

	// GlobInfo Tests
	t.Run("GlobInfo: Success", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			globOutput := `{"path": "file.go", "is_dir": false}`
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, globOutput, "", ""))
		}
		res, err := s.GlobInfo(context.Background(), &filesystem.GlobInfoRequest{Pattern: "*.go"})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "file.go", res[0].Path)
	})

	// Write Tests
	t.Run("Write: Success", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, "", "", ""))
		}
		err := s.Write(context.Background(), &filesystem.WriteRequest{FilePath: "/data/new.txt", Content: "new content"})
		require.NoError(t, err)
	})

	t.Run("Write: Failure - Script Error", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, false, "File exists", "", ""))
		}
		err := s.Write(context.Background(), &filesystem.WriteRequest{FilePath: "/data/new.txt", Content: "new content"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write script exited with non-zero code -1: File exists")
	})

	// Edit Tests
	t.Run("Edit: Success", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, "1", "", "")) // Output is the count of replacements
		}
		err := s.Edit(context.Background(), &filesystem.EditRequest{FilePath: "/data/file.txt", OldString: "old", NewString: "new"})
		require.NoError(t, err)
	})

	t.Run("Edit: Failure - Validation", func(t *testing.T) {
		err := s.Edit(context.Background(), &filesystem.EditRequest{FilePath: "/data/file.txt", OldString: "same", NewString: "same"})
		require.Error(t, err)
		assert.Equal(t, "new string must be different from old string", err.Error())
	})

	// Execute Tests
	t.Run("Execute: Success", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, "command output", "", ""))
		}
		res, err := s.Execute(context.Background(), &filesystem.ExecuteRequest{Command: "echo hello"})
		require.NoError(t, err)
		assert.Equal(t, "command output", res.Output)
	})

	t.Run("Execute: Failure - Empty Command", func(t *testing.T) {
		_, err := s.Execute(context.Background(), &filesystem.ExecuteRequest{Command: ""})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command is required")
	})

	t.Run("Execute: Failure - Script Error", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, false, "command failed", "Error", "1"))
		}
		_, err := s.Execute(context.Background(), &filesystem.ExecuteRequest{Command: "exit 1"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command exited with non-zero code -1: command failed")
	})
}
