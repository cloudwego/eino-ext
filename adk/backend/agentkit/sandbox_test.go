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
	"github.com/gen2brain/go-fitz"
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
		s, err := NewSandboxToolBackend(config)

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
		s, err := NewSandboxToolBackend(config)

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
func setupTest(t *testing.T) (*SandboxTool, *httptest.Server) {
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
	sandbox, err := NewSandboxToolBackend(config)
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

	t.Run("LsInfo: Relative Path Allowed", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			lsOutput := `{"path": "file1.txt", "is_dir": false}` + "\n" + `{"path": "dir1", "is_dir": true}`
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, lsOutput, "", ""))
		}
		res, err := s.LsInfo(context.Background(), &filesystem.LsInfoRequest{Path: "relative/path"})
		require.NoError(t, err)
		require.Len(t, res, 2)
		assert.Equal(t, "file1.txt", res[0].Path)
		assert.Equal(t, "dir1", res[1].Path)
	})

	// Read Tests
	t.Run("Read: Success", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, "hello world", "", ""))
		}
		res, err := s.Read(context.Background(), &filesystem.ReadRequest{FilePath: "/data/file.txt"})
		require.NoError(t, err)
		assert.Equal(t, "hello world", res.Content)
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
			grepOutput := `[{"Path": "/data/file.txt", "Line": 1, "Content": "hello world"}]`
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
			globOutput := `[{"path": "file.go", "is_dir": false}]`
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
		resp, err := s.Execute(context.Background(), &filesystem.ExecuteRequest{Command: "exit 1"})
		require.Nil(t, err)
		assert.Contains(t, resp.Output, "command failed")
	})

	// Special character path tests — verify base64 encoding prevents template breakage.
	t.Run("LsInfo: Special Characters in Path", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, `{"path": "a.txt", "is_dir": false}`, "", ""))
		}
		specialPaths := []string{
			"/data/it's a dir",
			"/data/文档/测试",
			`/data/back\slash`,
			"/data/new\nline",
			"/data/path with spaces",
		}
		for _, p := range specialPaths {
			res, err := s.LsInfo(context.Background(), &filesystem.LsInfoRequest{Path: p})
			require.NoError(t, err, "path=%q", p)
			require.Len(t, res, 1, "path=%q", p)
		}
	})

	t.Run("GrepRaw: Special Characters in Pattern and Path", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, `[]`, "", ""))
		}
		cases := []struct {
			pattern string
			path    string
		}{
			{`\bfoo\b`, "/data/it's here"},
			{`hello'world`, "/data/文档"},
			{`line1\nline2`, `/data/back\slash`},
		}
		for _, tc := range cases {
			_, err := s.GrepRaw(context.Background(), &filesystem.GrepRequest{
				Pattern: tc.pattern,
				Path:    tc.path,
			})
			require.NoError(t, err, "pattern=%q path=%q", tc.pattern, tc.path)
		}
	})

	t.Run("Write: Special Characters in Path", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, "", "", ""))
		}
		specialPaths := []string{
			"/data/it's a file.txt",
			"/data/文档/新文件.txt",
			`/data/back\slash.txt`,
		}
		for _, p := range specialPaths {
			err := s.Write(context.Background(), &filesystem.WriteRequest{FilePath: p, Content: "content"})
			require.NoError(t, err, "path=%q", p)
		}
	})

	t.Run("Edit: Special Characters in Path", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, "1", "", ""))
		}
		specialPaths := []string{
			"/data/it's a file.txt",
			"/data/文档/新文件.txt",
			`/data/back\slash.txt`,
		}
		for _, p := range specialPaths {
			err := s.Edit(context.Background(), &filesystem.EditRequest{FilePath: p, OldString: "old", NewString: "new"})
			require.NoError(t, err, "path=%q", p)
		}
	})

	t.Run("Execute: RunInBackendGround returns immediately", func(t *testing.T) {
		mockAPIHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(createMockResponse(t, true, "command output", "", ""))
		}
		res, err := s.Execute(context.Background(), &filesystem.ExecuteRequest{
			Command:            "sleep 10",
			RunInBackendGround: true,
		})
		require.NoError(t, err)
		assert.Contains(t, res.Output, "background")
	})
}

func TestParsePagesParam(t *testing.T) {
	const dflt = defaultMaxPDFPagesPerRequest
	tests := []struct {
		name      string
		pages     string
		maxPages  int
		wantStart int
		wantEnd   int
		wantErr   string
	}{
		{name: "single page", pages: "1", maxPages: dflt, wantStart: 1, wantEnd: 1},
		{name: "single page with spaces", pages: " 3 ", maxPages: dflt, wantStart: 3, wantEnd: 3},
		{name: "range", pages: "1-5", maxPages: dflt, wantStart: 1, wantEnd: 5},
		{name: "range with spaces", pages: " 3 - 10 ", maxPages: dflt, wantStart: 3, wantEnd: 10},
		{name: "same start and end", pages: "7-7", maxPages: dflt, wantStart: 7, wantEnd: 7},
		{name: "max allowed range", pages: "1-20", maxPages: dflt, wantStart: 1, wantEnd: 20},
		{name: "custom maxPages happy path", pages: "1-5", maxPages: 5, wantStart: 1, wantEnd: 5},
		{name: "non-positive maxPages falls back to default", pages: "1-20", maxPages: 0, wantStart: 1, wantEnd: 20},

		{name: "empty string", pages: "", maxPages: dflt, wantErr: `invalid pages parameter: "" (empty)`},
		{name: "blank string", pages: "   ", maxPages: dflt, wantErr: `invalid pages parameter: "   " (empty)`},
		{name: "non-numeric", pages: "abc", maxPages: dflt, wantErr: "start page must be a positive integer"},
		{name: "zero start", pages: "0", maxPages: dflt, wantErr: "start page must be a positive integer"},
		{name: "negative start", pages: "-3", maxPages: dflt, wantErr: "start page must be a positive integer"},
		{name: "non-numeric end", pages: "1-abc", maxPages: dflt, wantErr: "end page must be a positive integer"},
		{name: "zero end", pages: "1-0", maxPages: dflt, wantErr: "end page must be a positive integer"},
		{name: "open-ended range", pages: "1-", maxPages: dflt, wantErr: "open-ended range is not supported"},
		{name: "multi range 2-2-5", pages: "2-2-5", maxPages: dflt, wantErr: "only a single range is supported"},
		{name: "multi range 1-2-3", pages: "1-2-3", maxPages: dflt, wantErr: "only a single range is supported"},
		{name: "end less than start", pages: "5-3", maxPages: dflt, wantErr: "end page 3 < start page 5"},
		{name: "exceeds page limit", pages: "1-21", maxPages: dflt, wantErr: "range spans 21 pages, exceeds limit 20"},
		{name: "exceeds custom maxPages=5", pages: "1-6", maxPages: 5, wantErr: "range spans 6 pages, exceeds limit 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parsePagesParam(tt.pages, tt.maxPages)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Contains(t, err.Error(), "invalid pages parameter:", "all errors must share the unified prefix")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStart, start)
			assert.Equal(t, tt.wantEnd, end)
		})
	}
}

func TestSplitPagesRange(t *testing.T) {
	tests := []struct {
		name      string
		pages     string
		wantStart string
		wantEnd   string
		wantRange bool
		wantErr   string
	}{
		{name: "single", pages: "1", wantStart: "1"},
		{name: "single trimmed", pages: " 5 ", wantStart: "5"},
		{name: "range", pages: "1-5", wantStart: "1", wantEnd: "5", wantRange: true},
		{name: "range trimmed", pages: " 3 - 10 ", wantStart: "3", wantEnd: "10", wantRange: true},
		{name: "empty", pages: "", wantErr: `invalid pages parameter: "" (empty)`},
		{name: "blanks only", pages: "   ", wantErr: "(empty)"},
		{name: "open ended", pages: "3-", wantErr: "open-ended range is not supported"},
		{name: "multi range", pages: "1-2-3", wantErr: "only a single range is supported"},
		{name: "multi range with spaces", pages: " 1 - 2 - 3 ", wantErr: "only a single range is supported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, e, hasRange, err := splitPagesRange(tt.pages)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStart, s)
			assert.Equal(t, tt.wantEnd, e)
			assert.Equal(t, tt.wantRange, hasRange)
		})
	}
}

func TestValidatePagesRange(t *testing.T) {
	tests := []struct {
		name     string
		start    int
		end      int
		maxPages int
		wantErr  string
	}{
		{name: "happy single", start: 1, end: 1, maxPages: 20},
		{name: "happy range", start: 1, end: 5, maxPages: 20},
		{name: "boundary equal to max", start: 1, end: 20, maxPages: 20},
		{name: "boundary maxPages 1", start: 7, end: 7, maxPages: 1},
		{name: "end < start", start: 5, end: 3, maxPages: 20, wantErr: "end page 3 < start page 5"},
		{name: "exceeds limit", start: 1, end: 21, maxPages: 20, wantErr: "range spans 21 pages, exceeds limit 20"},
		{name: "exceeds custom limit", start: 1, end: 2, maxPages: 1, wantErr: "range spans 2 pages, exceeds limit 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePagesRange(tt.start, tt.end, tt.maxPages, "raw-input")
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Contains(t, err.Error(), "invalid pages parameter:")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDetectImageMIME(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "png", data: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}, want: "image/png"},
		{name: "png too short", data: []byte{0x89, 0x50, 0x4E, 0x47}, want: ""},
		{name: "jpeg", data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, want: "image/jpeg"},
		{name: "jpeg too short", data: []byte{0xFF, 0xD8}, want: ""},
		{name: "gif87a", data: []byte("GIF87a---"), want: "image/gif"},
		{name: "gif89a", data: []byte("GIF89a---"), want: "image/gif"},
		{name: "gif88a not recognized", data: []byte("GIF88a---"), want: ""},
		{name: "bmp", data: []byte("BM" + "\x00\x00\x00\x00"), want: "image/bmp"},
		{name: "webp", data: []byte("RIFF\x00\x00\x00\x00WEBP-extra"), want: "image/webp"},
		{name: "webp wrong tag", data: []byte("RIFF\x00\x00\x00\x00WAVE-extra"), want: ""},
		{name: "tiff little endian", data: []byte{0x49, 0x49, 0x2A, 0x00, 0x00}, want: "image/tiff"},
		{name: "tiff big endian", data: []byte{0x4D, 0x4D, 0x00, 0x2A, 0x00}, want: "image/tiff"},
		{name: "empty", data: []byte{}, want: ""},
		{name: "random", data: []byte("hello world"), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectImageMIME(tt.data))
		})
	}
}

func TestIsPDFBytes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "valid pdf header", data: []byte("%PDF-1.4\n%..."), want: true},
		{name: "exactly 5 bytes", data: []byte("%PDF-"), want: true},
		{name: "missing percent", data: []byte("PDF-1.4"), want: false},
		{name: "too short", data: []byte("%PDF"), want: false},
		{name: "empty", data: []byte{}, want: false},
		{name: "random", data: []byte("not a pdf"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isPDFBytes(tt.data))
		})
	}
}

func TestResolveMultiModalReadConfig(t *testing.T) {
	t.Run("zero input → all defaults", func(t *testing.T) {
		got := resolveMultiModalReadConfig(MultiModalReadConfig{})
		assert.Equal(t, defaultMaxImageSizeMB, got.MaxImageSizeMB)
		assert.Equal(t, defaultMaxPDFSizeMB, got.MaxPDFSizeMB)
		assert.Equal(t, defaultMaxPagedPDFSizeMB, got.MaxPagedPDFSizeMB)
		assert.Equal(t, defaultMaxPDFPagesPerRequest, got.MaxPDFPagesPerRequest)
		assert.Equal(t, defaultPDFRenderDPI, got.PDFRenderDPI)
	})

	t.Run("partial override preserved", func(t *testing.T) {
		got := resolveMultiModalReadConfig(MultiModalReadConfig{
			MaxImageSizeMB: 25,
			PDFRenderDPI:   200,
		})
		assert.Equal(t, 25, got.MaxImageSizeMB)
		assert.Equal(t, 200.0, got.PDFRenderDPI)
		// untouched fields fall back to default
		assert.Equal(t, defaultMaxPDFSizeMB, got.MaxPDFSizeMB)
		assert.Equal(t, defaultMaxPagedPDFSizeMB, got.MaxPagedPDFSizeMB)
		assert.Equal(t, defaultMaxPDFPagesPerRequest, got.MaxPDFPagesPerRequest)
	})

	t.Run("negative treated as unset", func(t *testing.T) {
		got := resolveMultiModalReadConfig(MultiModalReadConfig{
			MaxImageSizeMB:        -1,
			MaxPDFSizeMB:          -100,
			MaxPagedPDFSizeMB:     -1,
			MaxPDFPagesPerRequest: -1,
			PDFRenderDPI:          -1,
		})
		assert.Equal(t, defaultMaxImageSizeMB, got.MaxImageSizeMB)
		assert.Equal(t, defaultMaxPDFSizeMB, got.MaxPDFSizeMB)
		assert.Equal(t, defaultMaxPagedPDFSizeMB, got.MaxPagedPDFSizeMB)
		assert.Equal(t, defaultMaxPDFPagesPerRequest, got.MaxPDFPagesPerRequest)
		assert.Equal(t, defaultPDFRenderDPI, got.PDFRenderDPI)
	})

	t.Run("values exceeding hard-cap are clamped", func(t *testing.T) {
		got := resolveMultiModalReadConfig(MultiModalReadConfig{
			MaxImageSizeMB:        maxConfigurableMB + 1,
			MaxPDFSizeMB:          maxConfigurableMB * 10,
			MaxPagedPDFSizeMB:     maxConfigurableMB + 1,
			MaxPDFPagesPerRequest: maxConfigurablePDFPagesPerRequest + 1,
			PDFRenderDPI:          maxConfigurablePDFRenderDPI + 1,
		})
		assert.Equal(t, maxConfigurableMB, got.MaxImageSizeMB)
		assert.Equal(t, maxConfigurableMB, got.MaxPDFSizeMB)
		assert.Equal(t, maxConfigurableMB, got.MaxPagedPDFSizeMB)
		assert.Equal(t, maxConfigurablePDFPagesPerRequest, got.MaxPDFPagesPerRequest)
		assert.Equal(t, maxConfigurablePDFRenderDPI, got.PDFRenderDPI)
	})
}

// minimalValidPDF is a hand-crafted single-page PDF kept inline so the test
// suite needs no binary fixture. fitz emits xref-repair warnings on it but
// still resolves NumPage()==1.
const minimalValidPDF = "%PDF-1.1\n%\xa5\xb1\xeb\n\n1 0 obj\n  << /Type /Catalog\n     /Pages 2 0 R\n  >>\nendobj\n\n2 0 obj\n  << /Type /Pages\n     /Kids [3 0 R]\n     /Count 1\n     /MediaBox [0 0 100 100]\n  >>\nendobj\n\n3 0 obj\n  <<  /Type /Page\n      /Parent 2 0 R\n      /Resources << >>\n  >>\nendobj\n\nxref\n0 4\n0000000000 65535 f \n0000000018 00000 n \n0000000077 00000 n \n0000000178 00000 n \ntrailer\n  <<  /Root 1 0 R\n      /Size 4\n  >>\nstartxref\n240\n%%EOF\n"

func TestIsImageExt(t *testing.T) {
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif"} {
		assert.True(t, isImageExt(ext), ext)
	}
	for _, ext := range []string{".pdf", ".txt", ".go", "", ".PNG"} {
		assert.False(t, isImageExt(ext), ext)
	}
}

func TestIsPDFExt(t *testing.T) {
	assert.True(t, isPDFExt(".pdf"))
	assert.False(t, isPDFExt(".PDF"))
	assert.False(t, isPDFExt(".txt"))
	assert.False(t, isPDFExt(""))
}

func TestNewImageContentPart(t *testing.T) {
	data := []byte("fake-image-data")
	part := newImageContentPart("image/png", data)
	assert.Equal(t, filesystem.FileContentPartTypeImage, part.Type)
	assert.Equal(t, "image/png", part.MIMEType)
	assert.Equal(t, data, part.Data)
}

func TestRenderPDFPagesToImages_Success(t *testing.T) {
	doc, err := fitz.NewFromMemory([]byte(minimalValidPDF))
	if err != nil || doc.NumPage() < 1 {
		t.Skipf("minimal PDF fixture not parseable in this environment: err=%v", err)
	}
	defer doc.Close()

	parts, err := renderPDFPagesToImages(context.Background(), doc, 1, 1, "fixture.pdf", 72)
	require.NoError(t, err)
	require.Len(t, parts, 1)
	assert.Equal(t, filesystem.FileContentPartTypeImage, parts[0].Type)
	assert.Equal(t, "image/png", parts[0].MIMEType)
	assert.NotEmpty(t, parts[0].Data)
}

func TestRenderPDFPagesToImages_DefaultDPI(t *testing.T) {
	doc, err := fitz.NewFromMemory([]byte(minimalValidPDF))
	if err != nil || doc.NumPage() < 1 {
		t.Skipf("minimal PDF fixture not parseable in this environment: err=%v", err)
	}
	defer doc.Close()

	parts, err := renderPDFPagesToImages(context.Background(), doc, 1, 1, "fixture.pdf", 0)
	require.NoError(t, err)
	require.Len(t, parts, 1)
}

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
