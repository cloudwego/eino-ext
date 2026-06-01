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

package einoacp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/filesystem"
	acpproto "github.com/eino-contrib/acp"
)

// discardLogger is a slog.Logger that drops every record. It keeps test output
// clean while still exercising the warn/info/debug paths inside Backend.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubConn returns a minimal ACPConn implementation that is safe to embed in
// a Backend as long as no RPC method is actually invoked. Used for
// Config.validate / NewBackend bookkeeping tests.
func stubConn() ACPConn {
	return &mockConn{}
}

// --- Config.validate ---

func TestConfigValidate(t *testing.T) {
	conn := stubConn()
	caps := &acpproto.ClientCapabilities{}

	cases := []struct {
		name    string
		cfg     *Config
		wantErr string // substring; "" means must succeed
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: "cfg is required",
		},
		{
			name:    "missing Conn",
			cfg:     &Config{SessionID: "s", Capabilities: caps},
			wantErr: "cfg.Conn is required",
		},
		{
			name:    "missing SessionID",
			cfg:     &Config{Conn: conn, Capabilities: caps},
			wantErr: "cfg.SessionID is required",
		},
		{
			name:    "missing Capabilities",
			cfg:     &Config{Conn: conn, SessionID: "s"},
			wantErr: "cfg.Capabilities is required",
		},
		{
			name:    "valid",
			cfg:     &Config{Conn: conn, SessionID: "s", Capabilities: caps},
			wantErr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

// --- NewBackend ---

func TestNewBackend_PropagatesCapabilities(t *testing.T) {
	conn := stubConn()

	t.Run("fs both enabled, terminal disabled", func(t *testing.T) {
		b, err := NewBackend(&Config{
			Conn:      conn,
			SessionID: "s1",
			Capabilities: &acpproto.ClientCapabilities{
				FS:       &acpproto.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
				Terminal: false,
			},
			Logger: discardLogger(),
		})
		if err != nil {
			t.Fatalf("NewBackend: %v", err)
		}
		if !b.hasReadFS || !b.hasWriteFS {
			t.Fatalf("expected hasReadFS && hasWriteFS to be true, got %v %v", b.hasReadFS, b.hasWriteFS)
		}
		if b.shell != nil {
			t.Fatalf("expected nil shell when terminal capability is off")
		}
	})

	t.Run("terminal advertised but UseTerminalForFileTools off", func(t *testing.T) {
		b, err := NewBackend(&Config{
			Conn:                    conn,
			SessionID:               "s2",
			Capabilities:            &acpproto.ClientCapabilities{Terminal: true},
			UseTerminalForFileTools: false,
			Logger:                  discardLogger(),
		})
		if err != nil {
			t.Fatalf("NewBackend: %v", err)
		}
		if b.shell != nil {
			t.Fatalf("expected nil shell when UseTerminalForFileTools is false")
		}
	})

	t.Run("terminal + UseTerminalForFileTools wires shell", func(t *testing.T) {
		b, err := NewBackend(&Config{
			Conn:                    conn,
			SessionID:               "s3",
			Capabilities:            &acpproto.ClientCapabilities{Terminal: true},
			UseTerminalForFileTools: true,
			Logger:                  discardLogger(),
		})
		if err != nil {
			t.Fatalf("NewBackend: %v", err)
		}
		if b.shell == nil {
			t.Fatalf("expected shell to be wired")
		}
		if b.shell.sessionID != "s3" {
			t.Fatalf("shell.sessionID = %q, want s3", b.shell.sessionID)
		}
	})

	t.Run("nil FS capability leaves flags false", func(t *testing.T) {
		b, err := NewBackend(&Config{
			Conn:         conn,
			SessionID:    "s4",
			Capabilities: &acpproto.ClientCapabilities{}, // FS == nil
		})
		if err != nil {
			t.Fatalf("NewBackend: %v", err)
		}
		if b.hasReadFS || b.hasWriteFS {
			t.Fatalf("expected fs flags to default to false")
		}
	})

	t.Run("invalid config bubbles up", func(t *testing.T) {
		_, err := NewBackend(&Config{}) // missing everything
		if err == nil {
			t.Fatal("expected error from invalid config")
		}
	})
}

// --- Capability-missing gates ---

// newBackendNoShell builds a Backend with no shell wired so we can assert that
// the terminal-backed tools surface ErrCapabilityMissing without ever touching
// the underlying ACPConn.
func newBackendNoShell(t *testing.T) *Backend {
	t.Helper()
	b, err := NewBackend(&Config{
		Conn:         stubConn(),
		SessionID:    "test-session",
		Capabilities: &acpproto.ClientCapabilities{}, // no fs, no terminal
	})
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	return b
}

func TestBackend_LsInfo_NoShell(t *testing.T) {
	b := newBackendNoShell(t)
	_, err := b.LsInfo(context.Background(), &filesystem.LsInfoRequest{Path: "."})
	if !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("expected ErrCapabilityMissing, got %v", err)
	}
}

func TestBackend_GrepRaw_NoShell(t *testing.T) {
	b := newBackendNoShell(t)
	_, err := b.GrepRaw(context.Background(), &filesystem.GrepRequest{Pattern: "x"})
	if !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("expected ErrCapabilityMissing, got %v", err)
	}
}

func TestBackend_GlobInfo_NoShell(t *testing.T) {
	b := newBackendNoShell(t)
	_, err := b.GlobInfo(context.Background(), &filesystem.GlobInfoRequest{Pattern: "*.go"})
	if !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("expected ErrCapabilityMissing, got %v", err)
	}
}

// --- Backend.Edit early-return semantics ---

func TestBackend_Edit_EmptyOldString(t *testing.T) {
	b := newBackendNoShell(t)
	err := b.Edit(context.Background(), &filesystem.EditRequest{
		FilePath:  "f.txt",
		OldString: "",
		NewString: "x",
	})
	if err == nil || !strings.Contains(err.Error(), "oldString must be non-empty") {
		t.Fatalf("expected oldString validation error, got %v", err)
	}
}

func TestBackend_Edit_NoOpReturnsNil(t *testing.T) {
	// When OldString == NewString, Edit must short-circuit BEFORE the fs
	// capability check, otherwise harmless no-op edits would surface as
	// errors when running against clients that don't expose fs.
	b := newBackendNoShell(t)
	err := b.Edit(context.Background(), &filesystem.EditRequest{
		FilePath:  "f.txt",
		OldString: "same",
		NewString: "same",
	})
	if err != nil {
		t.Fatalf("expected nil for no-op edit, got %v", err)
	}
}

func TestBackend_Edit_RequiresFSCapability(t *testing.T) {
	b := newBackendNoShell(t)
	err := b.Edit(context.Background(), &filesystem.EditRequest{
		FilePath:  "f.txt",
		OldString: "a",
		NewString: "b",
	})
	if !errors.Is(err, ErrCapabilityMissing) {
		t.Fatalf("expected ErrCapabilityMissing, got %v", err)
	}
}

// --- shellQuote / joinShellArgs ---

func TestShellQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "''"},
		{"plain", "'plain'"},
		{"with space", "'with space'"},
		{"it's", `'it'\''s'`}, // single quote becomes '\''
		{"a'b'c", `'a'\''b'\''c'`},
		{"$(echo pwn)", "'$(echo pwn)'"}, // shell metacharacters stay literal
	}
	for _, c := range cases {
		got, err := shellQuote(c.in)
		if err != nil {
			t.Errorf("shellQuote(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("shellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJoinShellArgs(t *testing.T) {
	got, err := joinShellArgs([]string{"grep", "-e", "foo bar", "."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "'grep' '-e' 'foo bar' '.'"
	if got != want {
		t.Fatalf("joinShellArgs = %q, want %q", got, want)
	}

	// Empty input must produce empty string, not panic.
	if got, err := joinShellArgs(nil); err != nil {
		t.Fatalf("joinShellArgs(nil) unexpected error: %v", err)
	} else if got != "" {
		t.Fatalf("joinShellArgs(nil) = %q, want empty", got)
	}
}

// --- parseRipgrepJSONOutput ---

func TestParseRipgrepJSONOutput_Empty(t *testing.T) {
	matches, err := parseRipgrepJSONOutput("", discardLogger())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %d", len(matches))
	}
}

func TestParseRipgrepJSONOutput_Valid(t *testing.T) {
	// Simulate a minimal rg --json stream: one "begin" event (ignored) and
	// one "match" event (kept). Trailing newline on lines.text is trimmed.
	out := strings.Join([]string{
		`{"type":"begin","data":{"path":{"text":"main.go"}}}`,
		`{"type":"match","data":{"path":{"text":"main.go"},"line_number":42,"lines":{"text":"hello\n"}}}`,
	}, "\n")

	matches, err := parseRipgrepJSONOutput(out, discardLogger())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := []filesystem.GrepMatch{{Path: "main.go", Line: 42, Content: "hello"}}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("matches = %#v, want %#v", matches, want)
	}
}

func TestParseRipgrepJSONOutput_AllFailedReturnsError(t *testing.T) {
	// Non-JSON output simulates an incompatible rg version that does not
	// emit JSON at all. We expect an error rather than silently returning
	// an empty slice.
	out := "this is not json\nneither is this\n"
	_, err := parseRipgrepJSONOutput(out, discardLogger())
	if err == nil {
		t.Fatal("expected error when every output line fails to parse")
	}
	if !strings.Contains(err.Error(), "failed JSON parsing") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParseRipgrepJSONOutput_PartialFailureLogged(t *testing.T) {
	// One bad line + two valid lines (one match, one begin): parsing must
	// succeed and skip the bad line. Failure rate 1/3 < 50%.
	out := strings.Join([]string{
		`not json`,
		`{"type":"begin","data":{"path":{"text":"a.go"}}}`,
		`{"type":"match","data":{"path":{"text":"a.go"},"line_number":1,"lines":{"text":"x"}}}`,
	}, "\n")
	matches, err := parseRipgrepJSONOutput(out, discardLogger())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(matches) != 1 || matches[0].Path != "a.go" {
		t.Fatalf("matches = %#v", matches)
	}
}

// --- parsePosixGrepOutput ---

func TestParsePosixGrepOutput_Basic(t *testing.T) {
	out := strings.Join([]string{
		"main.go:10:func Foo() {",
		"util.go:3:package util",
	}, "\n")

	got := parsePosixGrepOutput(out)
	want := []filesystem.GrepMatch{
		{Path: "main.go", Line: 10, Content: "func Foo() {"},
		{Path: "util.go", Line: 3, Content: "package util"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParsePosixGrepOutput_ContextAndSeparators(t *testing.T) {
	// `--` separates context groups; context lines use `-` instead of `:`
	// after the line number. Both should flow through, preserving line
	// numbers, while `--` itself is dropped.
	out := strings.Join([]string{
		"f.go-9-// before",
		"f.go:10:hit",
		"f.go-11-// after",
		"--",
		"g.go:5:hit",
	}, "\n")

	got := parsePosixGrepOutput(out)
	want := []filesystem.GrepMatch{
		{Path: "f.go", Line: 9, Content: "// before"},
		{Path: "f.go", Line: 10, Content: "hit"},
		{Path: "f.go", Line: 11, Content: "// after"},
		{Path: "g.go", Line: 5, Content: "hit"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParsePosixGrepOutput_SkipsMalformed(t *testing.T) {
	// Lines without the expected `path:lineno:...` shape (e.g. "Binary file
	// foo matches") are skipped silently rather than producing zero-row
	// noise in the result.
	out := strings.Join([]string{
		"Binary file foo matches",
		"f.go:1:ok",
		"",
		"--",
	}, "\n")
	got := parsePosixGrepOutput(out)
	want := []filesystem.GrepMatch{
		{Path: "f.go", Line: 1, Content: "ok"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParsePosixGrepOutput_NongreedyPathWithColon(t *testing.T) {
	// The parser splits on the FIRST `<sep><digits><sep>` boundary.
	// Content containing additional colons and digits should not confuse it.
	out := "main.go:7:url := \"http://example.com:8080/\""
	got := parsePosixGrepOutput(out)
	want := []filesystem.GrepMatch{
		{Path: "main.go", Line: 7, Content: `url := "http://example.com:8080/"`},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParsePosixGrepOutput_PathContainingColonDigits(t *testing.T) {
	// Path like "vendor/pkg:v2/file.go" should parse correctly — the first
	// valid <sep><digits><sep> after "v2" is `:10:`.
	out := "vendor/pkg:v2/file.go:10:func Init() {"
	got := parsePosixGrepOutput(out)
	// Note: "v2" is not a valid line number boundary because 'v' precedes '2',
	// so the parser correctly finds `:10:` as the first <sep><digits><sep>.
	// However, our left-to-right scan finds the first separator at index of first
	// ':' -> checks if digits follow. "v2/file.go" doesn't start with digits.
	// Actually `:` at position 10 (after "vendor/pkg"), next char is 'v' — not a digit.
	// So it skips, finds next `:` after "v2/file.go", digits "10", then ':'. Correct!
	want := []filesystem.GrepMatch{
		{Path: "vendor/pkg:v2/file.go", Line: 10, Content: "func Init() {"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParsePosixGrepOutput_Empty(t *testing.T) {
	if got := parsePosixGrepOutput(""); len(got) != 0 {
		t.Fatalf("expected no matches, got %#v", got)
	}
	if got := parsePosixGrepOutput("\n--\n\n"); len(got) != 0 {
		t.Fatalf("expected no matches, got %#v", got)
	}
}

func TestParsePosixGrepLine_AdversarialPath(t *testing.T) {
	// Paths containing `:N:` substrings are a known limitation. Document the
	// current best-effort behavior so future changes have a regression baseline.
	cases := []struct {
		name    string
		line    string
		wantOK  bool
		path    string
		lineno  int
		content string
	}{
		{
			// Path "prefix:1:rest.go" contains `:1:` which the parser will
			// match first. This is a known incorrect split — we document it here.
			name:    "path with :1: prefix splits incorrectly (known limitation)",
			line:    "prefix:1:rest.go:10:actual content",
			wantOK:  true,
			path:    "prefix",
			lineno:  1,
			content: "rest.go:10:actual content",
		},
		{
			// Path "vendor/pkg:v2/file.go" — `:v2` is not digits, so the
			// parser correctly skips it and finds `:10:`.
			name:    "path with :v2/ correctly finds :10:",
			line:    "vendor/pkg:v2/file.go:10:func Init() {",
			wantOK:  true,
			path:    "vendor/pkg:v2/file.go",
			lineno:  10,
			content: "func Init() {",
		},
		{
			// Content containing `:digits:` should not confuse the parser
			// because we find the FIRST valid boundary scanning left-to-right.
			name:    "content with :8080: is not confused",
			line:    "main.go:7:url := \"http://example.com:8080/\"",
			wantOK:  true,
			path:    "main.go",
			lineno:  7,
			content: "url := \"http://example.com:8080/\"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, lineno, content, ok := parsePosixGrepLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if path != tc.path || lineno != tc.lineno || content != tc.content {
				t.Fatalf("got (%q, %d, %q), want (%q, %d, %q)",
					path, lineno, content, tc.path, tc.lineno, tc.content)
			}
		})
	}
}

// --- detectRipgrep caching ---

func TestDetectRipgrep_CachesAfterFirstProbe(t *testing.T) {
	// Verify cached state is honored without re-probing.
	b := &Backend{}
	b.rgState.Store(rgUnavailable)
	if b.detectRipgrep(context.Background()) {
		t.Fatal("expected cached false to be returned, got true")
	}

	b2 := &Backend{}
	b2.rgState.Store(rgAvailable)
	if !b2.detectRipgrep(context.Background()) {
		t.Fatal("expected cached true to be returned, got false")
	}
}

// --- Mock-based tests using ACPConn interface ---

// mockConn implements ACPConn for unit testing Backend methods.
type mockConn struct {
	readTextFile        func(ctx context.Context, req acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error)
	writeTextFile       func(ctx context.Context, req acpproto.WriteTextFileRequest) (acpproto.WriteTextFileResponse, error)
	createTerminal      func(ctx context.Context, req acpproto.CreateTerminalRequest) (acpproto.CreateTerminalResponse, error)
	waitForTerminalExit func(ctx context.Context, req acpproto.WaitForTerminalExitRequest) (acpproto.WaitForTerminalExitResponse, error)
	terminalOutput      func(ctx context.Context, req acpproto.TerminalOutputRequest) (acpproto.TerminalOutputResponse, error)
	releaseTerminal     func(ctx context.Context, req acpproto.ReleaseTerminalRequest) (acpproto.ReleaseTerminalResponse, error)
}

func (m *mockConn) ReadTextFile(ctx context.Context, req acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error) {
	if m.readTextFile != nil {
		return m.readTextFile(ctx, req)
	}
	return acpproto.ReadTextFileResponse{}, errors.New("ReadTextFile not mocked")
}

func (m *mockConn) WriteTextFile(ctx context.Context, req acpproto.WriteTextFileRequest) (acpproto.WriteTextFileResponse, error) {
	if m.writeTextFile != nil {
		return m.writeTextFile(ctx, req)
	}
	return acpproto.WriteTextFileResponse{}, errors.New("WriteTextFile not mocked")
}

func (m *mockConn) CreateTerminal(ctx context.Context, req acpproto.CreateTerminalRequest) (acpproto.CreateTerminalResponse, error) {
	if m.createTerminal != nil {
		return m.createTerminal(ctx, req)
	}
	return acpproto.CreateTerminalResponse{}, errors.New("CreateTerminal not mocked")
}

func (m *mockConn) WaitForTerminalExit(ctx context.Context, req acpproto.WaitForTerminalExitRequest) (acpproto.WaitForTerminalExitResponse, error) {
	if m.waitForTerminalExit != nil {
		return m.waitForTerminalExit(ctx, req)
	}
	return acpproto.WaitForTerminalExitResponse{}, errors.New("WaitForTerminalExit not mocked")
}

func (m *mockConn) TerminalOutput(ctx context.Context, req acpproto.TerminalOutputRequest) (acpproto.TerminalOutputResponse, error) {
	if m.terminalOutput != nil {
		return m.terminalOutput(ctx, req)
	}
	return acpproto.TerminalOutputResponse{}, errors.New("TerminalOutput not mocked")
}

func (m *mockConn) ReleaseTerminal(ctx context.Context, req acpproto.ReleaseTerminalRequest) (acpproto.ReleaseTerminalResponse, error) {
	if m.releaseTerminal != nil {
		return m.releaseTerminal(ctx, req)
	}
	return acpproto.ReleaseTerminalResponse{}, nil
}

// newMockBackend creates a Backend with a mock connection and shell wired up.
func newMockBackend(mc *mockConn) *Backend {
	logger := discardLogger()
	s := &shell{conn: mc, sessionID: "test-session", logger: logger}
	return &Backend{
		conn:       mc,
		sessionID:  "test-session",
		shell:      s,
		hasReadFS:  true,
		hasWriteFS: true,
	}
}

// terminalMock returns a mockConn that simulates a shell command producing the
// given output and exit code. Used by LsInfo/GlobInfo/GrepRaw tests.
func terminalMock(output string, exitCode int, truncated bool) *mockConn {
	ec := int64(exitCode)
	return &mockConn{
		createTerminal: func(_ context.Context, req acpproto.CreateTerminalRequest) (acpproto.CreateTerminalResponse, error) {
			return acpproto.CreateTerminalResponse{TerminalID: "t1"}, nil
		},
		waitForTerminalExit: func(_ context.Context, _ acpproto.WaitForTerminalExitRequest) (acpproto.WaitForTerminalExitResponse, error) {
			return acpproto.WaitForTerminalExitResponse{ExitCode: &ec}, nil
		},
		terminalOutput: func(_ context.Context, _ acpproto.TerminalOutputRequest) (acpproto.TerminalOutputResponse, error) {
			return acpproto.TerminalOutputResponse{
				Output:     output,
				Truncated:  truncated,
				ExitStatus: &acpproto.TerminalExitStatus{ExitCode: &ec},
			}, nil
		},
		releaseTerminal: func(_ context.Context, _ acpproto.ReleaseTerminalRequest) (acpproto.ReleaseTerminalResponse, error) {
			return acpproto.ReleaseTerminalResponse{}, nil
		},
	}
}

// --- Backend.Read / Write with mock ---

func TestBackend_Read_WithMock(t *testing.T) {
	mc := &mockConn{
		readTextFile: func(_ context.Context, req acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error) {
			if req.Path != "/foo/bar.txt" {
				t.Errorf("unexpected path: %s", req.Path)
			}
			if req.SessionID != "test-session" {
				t.Errorf("unexpected session: %s", req.SessionID)
			}
			return acpproto.ReadTextFileResponse{Content: "hello world"}, nil
		},
	}
	b := newMockBackend(mc)
	fc, err := b.Read(context.Background(), &filesystem.ReadRequest{FilePath: "/foo/bar.txt"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if fc.Content != "hello world" {
		t.Fatalf("got content %q, want %q", fc.Content, "hello world")
	}
}

func TestBackend_Read_Error(t *testing.T) {
	mc := &mockConn{
		readTextFile: func(_ context.Context, _ acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error) {
			return acpproto.ReadTextFileResponse{}, errors.New("file not found")
		},
	}
	b := newMockBackend(mc)
	_, err := b.Read(context.Background(), &filesystem.ReadRequest{FilePath: "/missing.txt"})
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("expected error containing 'file not found', got %v", err)
	}
}

func TestBackend_Write_WithMock(t *testing.T) {
	var captured acpproto.WriteTextFileRequest
	mc := &mockConn{
		writeTextFile: func(_ context.Context, req acpproto.WriteTextFileRequest) (acpproto.WriteTextFileResponse, error) {
			captured = req
			return acpproto.WriteTextFileResponse{}, nil
		},
	}
	b := newMockBackend(mc)
	err := b.Write(context.Background(), &filesystem.WriteRequest{FilePath: "/out.txt", Content: "data"})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if captured.Path != "/out.txt" || captured.Content != "data" {
		t.Fatalf("unexpected captured request: %+v", captured)
	}
}

// --- Backend.LsInfo with mock ---

func TestBackend_LsInfo_WithMock(t *testing.T) {
	mc := terminalMock("file1.txt\ndir1/\nfile2.go\n", 0, false)
	b := newMockBackend(mc)
	infos, err := b.LsInfo(context.Background(), &filesystem.LsInfoRequest{Path: "/tmp"})
	if err != nil {
		t.Fatalf("LsInfo: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(infos), infos)
	}
	if infos[0].Path != "file1.txt" || infos[0].IsDir {
		t.Errorf("infos[0] = %+v", infos[0])
	}
	if infos[1].Path != "dir1" || !infos[1].IsDir {
		t.Errorf("infos[1] = %+v", infos[1])
	}
	if infos[2].Path != "file2.go" || infos[2].IsDir {
		t.Errorf("infos[2] = %+v", infos[2])
	}
}

func TestBackend_LsInfo_EmptyDir(t *testing.T) {
	mc := terminalMock("", 0, false)
	b := newMockBackend(mc)
	infos, err := b.LsInfo(context.Background(), &filesystem.LsInfoRequest{Path: "."})
	if err != nil {
		t.Fatalf("LsInfo: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(infos))
	}
}

// --- Backend.GlobInfo with mock ---

func TestBackend_GlobInfo_WithMock(t *testing.T) {
	// find output includes the base path and nested files
	output := ".\n./main.go\n./sub/util.go\n./sub/readme.md\n"
	mc := terminalMock(output, 0, false)
	b := newMockBackend(mc)
	infos, err := b.GlobInfo(context.Background(), &filesystem.GlobInfoRequest{Pattern: "**/*.go", Path: "."})
	if err != nil {
		t.Fatalf("GlobInfo: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(infos), infos)
	}
	paths := []string{infos[0].Path, infos[1].Path}
	if paths[0] != "main.go" || paths[1] != "sub/util.go" {
		t.Fatalf("unexpected paths: %v", paths)
	}
}

func TestBackend_GlobInfo_PartialFailure(t *testing.T) {
	// Simulate `find` encountering permission-denied on some subdirectories:
	// exit code 1 but stdout still has valid results.
	output := ".\n./main.go\n./sub/util.go\n"
	mc := terminalMock(output, 1, false)
	b := newMockBackend(mc)
	infos, err := b.GlobInfo(context.Background(), &filesystem.GlobInfoRequest{Pattern: "**/*.go", Path: "."})
	if err != nil {
		t.Fatalf("GlobInfo should succeed on exit code 1 with valid output, got: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(infos), infos)
	}
}

func TestBackend_GlobInfo_HardFailure(t *testing.T) {
	// Exit code >= 2 is treated as a hard error.
	mc := terminalMock("", 2, false)
	b := newMockBackend(mc)
	_, err := b.GlobInfo(context.Background(), &filesystem.GlobInfoRequest{Pattern: "**/*.go", Path: "."})
	if err == nil {
		t.Fatal("expected error on exit code 2")
	}
	if !errors.Is(err, ErrShellNonZeroExit) {
		t.Fatalf("expected ErrShellNonZeroExit, got %v", err)
	}
}

// --- Backend.Edit with mock ---

func TestBackend_Edit_WithMock_Success(t *testing.T) {
	mc := &mockConn{
		readTextFile: func(_ context.Context, _ acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error) {
			return acpproto.ReadTextFileResponse{Content: "aaa bbb ccc"}, nil
		},
		writeTextFile: func(_ context.Context, req acpproto.WriteTextFileRequest) (acpproto.WriteTextFileResponse, error) {
			if req.Content != "aaa xxx ccc" {
				t.Errorf("unexpected content: %q", req.Content)
			}
			return acpproto.WriteTextFileResponse{}, nil
		},
	}
	b := newMockBackend(mc)
	err := b.Edit(context.Background(), &filesystem.EditRequest{
		FilePath: "f.go", OldString: "bbb", NewString: "xxx",
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
}

func TestBackend_Edit_WithMock_NotFound(t *testing.T) {
	mc := &mockConn{
		readTextFile: func(_ context.Context, _ acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error) {
			return acpproto.ReadTextFileResponse{Content: "hello"}, nil
		},
	}
	b := newMockBackend(mc)
	err := b.Edit(context.Background(), &filesystem.EditRequest{
		FilePath: "f.go", OldString: "missing", NewString: "x",
	})
	if !errors.Is(err, ErrOldStringNotFound) {
		t.Fatalf("expected ErrOldStringNotFound, got %v", err)
	}
}

func TestBackend_Edit_WithMock_Ambiguous(t *testing.T) {
	mc := &mockConn{
		readTextFile: func(_ context.Context, _ acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error) {
			return acpproto.ReadTextFileResponse{Content: "aa bb aa"}, nil
		},
	}
	b := newMockBackend(mc)
	err := b.Edit(context.Background(), &filesystem.EditRequest{
		FilePath: "f.go", OldString: "aa", NewString: "xx", ReplaceAll: false,
	})
	if !errors.Is(err, ErrAmbiguousReplace) {
		t.Fatalf("expected ErrAmbiguousReplace, got %v", err)
	}
}

func TestBackend_Edit_WithMock_ReplaceAll(t *testing.T) {
	var written string
	mc := &mockConn{
		readTextFile: func(_ context.Context, _ acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error) {
			return acpproto.ReadTextFileResponse{Content: "aa bb aa"}, nil
		},
		writeTextFile: func(_ context.Context, req acpproto.WriteTextFileRequest) (acpproto.WriteTextFileResponse, error) {
			written = req.Content
			return acpproto.WriteTextFileResponse{}, nil
		},
	}
	b := newMockBackend(mc)
	err := b.Edit(context.Background(), &filesystem.EditRequest{
		FilePath: "f.go", OldString: "aa", NewString: "xx", ReplaceAll: true,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if written != "xx bb xx" {
		t.Fatalf("unexpected written content: %q", written)
	}
}

// --- runShell ExitCode==nil (C2 fix) ---

func TestBackend_RunShell_NilExitCode(t *testing.T) {
	// Simulate a process killed by signal: ExitCode is nil in both wait and output responses.
	mc := &mockConn{
		createTerminal: func(_ context.Context, _ acpproto.CreateTerminalRequest) (acpproto.CreateTerminalResponse, error) {
			return acpproto.CreateTerminalResponse{TerminalID: "t1"}, nil
		},
		waitForTerminalExit: func(_ context.Context, _ acpproto.WaitForTerminalExitRequest) (acpproto.WaitForTerminalExitResponse, error) {
			return acpproto.WaitForTerminalExitResponse{Signal: "SIGKILL"}, nil
		},
		terminalOutput: func(_ context.Context, _ acpproto.TerminalOutputRequest) (acpproto.TerminalOutputResponse, error) {
			return acpproto.TerminalOutputResponse{
				Output:    "partial output before kill\n[terminated by signal: SIGKILL]",
				Truncated: false,
			}, nil
		},
		releaseTerminal: func(_ context.Context, _ acpproto.ReleaseTerminalRequest) (acpproto.ReleaseTerminalResponse, error) {
			return acpproto.ReleaseTerminalResponse{}, nil
		},
	}
	b := newMockBackend(mc)
	_, err := b.runShell(context.Background(), "sleep 999")
	if err == nil {
		t.Fatal("expected error when ExitCode is nil")
	}
	if !errors.Is(err, ErrShellNonZeroExit) {
		t.Fatalf("expected ErrShellNonZeroExit, got %v", err)
	}
	if !strings.Contains(err.Error(), "terminated without exit code") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestBackend_RunShell_Success(t *testing.T) {
	mc := terminalMock("output line\n", 0, false)
	b := newMockBackend(mc)
	result, err := b.runShell(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("runShell: %v", err)
	}
	if result.Output != "output line\n" {
		t.Fatalf("unexpected output: %q", result.Output)
	}
}

func TestBackend_RunShell_NonZeroExit(t *testing.T) {
	mc := terminalMock("error msg", 1, false)
	b := newMockBackend(mc)
	_, err := b.runShell(context.Background(), "false")
	if !errors.Is(err, ErrShellNonZeroExit) {
		t.Fatalf("expected ErrShellNonZeroExit, got %v", err)
	}
}

// --- GrepRaw with mock (ripgrep path) ---

func TestBackend_GrepRaw_Ripgrep(t *testing.T) {
	rgOutput := strings.Join([]string{
		`{"type":"begin","data":{"path":{"text":"main.go"}}}`,
		`{"type":"match","data":{"path":{"text":"main.go"},"line_number":5,"lines":{"text":"func hello()\n"}}}`,
	}, "\n")
	mc := terminalMock(rgOutput, 0, false)
	b := newMockBackend(mc)
	// Force rg available
	b.rgState.Store(rgAvailable)

	matches, err := b.GrepRaw(context.Background(), &filesystem.GrepRequest{Pattern: "hello", Path: "."})
	if err != nil {
		t.Fatalf("GrepRaw: %v", err)
	}
	if len(matches) != 1 || matches[0].Path != "main.go" || matches[0].Line != 5 {
		t.Fatalf("unexpected matches: %+v", matches)
	}
}

func TestBackend_GrepRaw_PosixFallback(t *testing.T) {
	posixOutput := "main.go:10:func Foo() {"
	mc := terminalMock(posixOutput, 0, false)
	b := newMockBackend(mc)
	// Force rg unavailable
	b.rgState.Store(rgUnavailable)

	matches, err := b.GrepRaw(context.Background(), &filesystem.GrepRequest{Pattern: "Foo", Path: "."})
	if err != nil {
		t.Fatalf("GrepRaw: %v", err)
	}
	if len(matches) != 1 || matches[0].Path != "main.go" || matches[0].Line != 10 {
		t.Fatalf("unexpected matches: %+v", matches)
	}
}

func TestBackend_GrepRaw_NoMatches(t *testing.T) {
	mc := terminalMock("", 1, false)
	b := newMockBackend(mc)
	b.rgState.Store(rgAvailable)

	matches, err := b.GrepRaw(context.Background(), &filesystem.GrepRequest{Pattern: "nonexistent", Path: "."})
	if err != nil {
		t.Fatalf("GrepRaw: %v", err)
	}
	if matches != nil {
		t.Fatalf("expected nil matches, got %+v", matches)
	}
}

// --- parseRipgrepJSONOutput majority failure (H2 fix) ---

func TestParseRipgrepJSONOutput_MajorityFailure(t *testing.T) {
	// 3 bad lines + 1 good line = 75% failure rate → should error
	out := strings.Join([]string{
		"not json 1",
		"not json 2",
		"not json 3",
		`{"type":"match","data":{"path":{"text":"a.go"},"line_number":1,"lines":{"text":"x"}}}`,
	}, "\n")
	_, err := parseRipgrepJSONOutput(out, discardLogger())
	if err == nil {
		t.Fatal("expected error when majority of lines fail parsing")
	}
	if !strings.Contains(err.Error(), "3/4") {
		t.Fatalf("expected error mentioning failure ratio, got: %v", err)
	}
}

func TestParseRipgrepJSONOutput_MinorityFailureSucceeds(t *testing.T) {
	// 1 bad line + 3 good lines = 25% failure rate → should succeed
	out := strings.Join([]string{
		"not json",
		`{"type":"match","data":{"path":{"text":"a.go"},"line_number":1,"lines":{"text":"x"}}}`,
		`{"type":"match","data":{"path":{"text":"b.go"},"line_number":2,"lines":{"text":"y"}}}`,
		`{"type":"begin","data":{"path":{"text":"c.go"}}}`,
	}, "\n")
	matches, err := parseRipgrepJSONOutput(out, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

// --- shellQuote with newline ---

func TestShellQuote_Newline(t *testing.T) {
	got, err := shellQuote("a\nb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "'a\nb'"
	if got != want {
		t.Errorf("shellQuote(%q) = %q, want %q", "a\nb", got, want)
	}
}

// --- truncateStr ---

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("short", 10); got != "short" {
		t.Errorf("truncateStr short: %q", got)
	}
	if got := truncateStr("hello world", 5); got != "hello..." {
		t.Errorf("truncateStr long: %q", got)
	}
}
