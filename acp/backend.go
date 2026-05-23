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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	mfs "github.com/cloudwego/eino/adk/middlewares/filesystem"
	acpproto "github.com/eino-contrib/acp"
)

// Sentinel errors for structured error handling by callers.
var (
	// ErrShellNonZeroExit is returned when a shell command exits with a non-zero code.
	ErrShellNonZeroExit = errors.New("acp.shell: non-zero exit")
	// ErrCapabilityMissing is returned when the client does not advertise the required capability.
	ErrCapabilityMissing = errors.New("acp: client capability not supported")
	// ErrOldStringNotFound is returned when Edit cannot locate the oldString in the file.
	ErrOldStringNotFound = errors.New("acp.edit: oldString not found")
	// ErrAmbiguousReplace is returned when multiple occurrences exist but ReplaceAll is false.
	ErrAmbiguousReplace = errors.New("acp.edit: ambiguous replacement (set ReplaceAll)")
	// ErrFileTooLarge is returned when Edit is attempted on a file exceeding maxEditFileSize.
	ErrFileTooLarge = errors.New("acp.edit: file too large for in-memory edit")
)

const (
	// releaseTerminalTimeout is how long we wait for ReleaseTerminal after the
	// command finishes (using a background context so cancellation won't prevent cleanup).
	releaseTerminalTimeout = 5 * time.Second

	// maxLogCommandLen is the maximum length of a command string included in log messages.
	maxLogCommandLen = 200

	// findMaxDepth is the -maxdepth value passed to `find` in GlobInfo.
	findMaxDepth = 20

	// maxEditFileSize is the maximum file size (in bytes) that Edit will process
	// in memory. Files larger than this are rejected to prevent OOM.
	maxEditFileSize = 32 * 1024 * 1024 // 32 MiB

	// rgJSONMaxFailedRatio is the maximum ratio of unparsable lines in `rg --json`
	// output before we treat the entire result as an error. This guards against
	// incompatible rg versions that emit non-JSON output while still tolerating
	// occasional malformed lines (e.g. from binary file matches or locale issues).
	rgJSONMaxFailedRatio = 0.5
)

// ACPConn is the minimal interface for ACP client-side RPC calls used by
// Backend and shell. It is satisfied by *acpconn.AgentConnection (from the
// github.com/eino-contrib/acp/conn package). Callers may provide alternative
// implementations for testing or proxying.
type ACPConn interface {
	ReadTextFile(ctx context.Context, req acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error)
	WriteTextFile(ctx context.Context, req acpproto.WriteTextFileRequest) (acpproto.WriteTextFileResponse, error)
	CreateTerminal(ctx context.Context, req acpproto.CreateTerminalRequest) (acpproto.CreateTerminalResponse, error)
	WaitForTerminalExit(ctx context.Context, req acpproto.WaitForTerminalExitRequest) (acpproto.WaitForTerminalExitResponse, error)
	TerminalOutput(ctx context.Context, req acpproto.TerminalOutputRequest) (acpproto.TerminalOutputResponse, error)
	ReleaseTerminal(ctx context.Context, req acpproto.ReleaseTerminalRequest) (acpproto.ReleaseTerminalResponse, error)
}

// Config configures NewClientToolsMiddleware.
type Config struct {
	// SessionID is the ACP session the middleware will operate on. Required.
	SessionID acpproto.SessionID
	// Conn is the agent-side ACP connection used to issue client requests. Required.
	// Any implementation of ACPConn is accepted; *conn.AgentConnection (from
	// github.com/eino-contrib/acp/conn) satisfies this interface and is the
	// typical production choice.
	Conn ACPConn
	// Capabilities is the client capability set advertised during initialization.
	// Required: tools are enabled based on what the client supports.
	Capabilities *acpproto.ClientCapabilities

	// UseTerminalForFileTools enables terminal-backed implementations of the
	// ls, glob, and grep tools. It only takes effect when the client
	// also advertises the terminal capability; otherwise those tools stay
	// disabled because the ACP protocol does not expose corresponding
	// filesystem methods.
	//
	// Note: edit always requires fs capability (ReadTextFile + WriteTextFile).
	//
	// Implementation: ls runs `ls -1A`, glob enumerates with `find` and matches
	// in-process via doublestar, grep prefers ripgrep (`rg --json`) and
	// transparently falls back to POSIX `grep -RnE` when `rg` is not
	// installed on the client side.
	UseTerminalForFileTools bool

	// Logger is an optional structured logger for non-fatal diagnostics (e.g.
	// ReleaseTerminal failures). If nil, slog.Default() is used.
	Logger *slog.Logger
}

func (c *Config) validate() error {
	if c == nil {
		return errors.New("acp.NewClientToolsMiddleware: cfg is required")
	}
	if c.Conn == nil {
		return errors.New("acp.NewClientToolsMiddleware: cfg.Conn is required")
	}
	if c.SessionID == "" {
		return errors.New("acp.NewClientToolsMiddleware: cfg.SessionID is required")
	}
	if c.Capabilities == nil {
		return errors.New("acp.NewClientToolsMiddleware: cfg.Capabilities is required")
	}
	return nil
}

// NewClientToolsMiddleware creates a ChatModelAgentMiddleware that bridges ACP client-side capabilities
// (filesystem read/write, terminal execution) to eino's filesystem tools. The ACP protocol only exposes
// read_text_file, write_text_file and terminal capabilities, so read_file and write_file are enabled
// only when the client advertises the corresponding capability. ls/glob/grep/edit are disabled by
// default; they become available when cfg.UseTerminalForFileTools is true and the client advertises
// the terminal capability — in which case they run as shell commands.
func NewClientToolsMiddleware(ctx context.Context, cfg *Config) (adk.ChatModelAgentMiddleware, error) {
	b, err := NewBackend(cfg)
	if err != nil {
		return nil, err
	}

	config := &mfs.MiddlewareConfig{
		Backend:             b,
		LsToolConfig:        &mfs.ToolConfig{Disable: true},
		ReadFileToolConfig:  &mfs.ToolConfig{Disable: true},
		WriteFileToolConfig: &mfs.ToolConfig{Disable: true},
		EditFileToolConfig:  &mfs.ToolConfig{Disable: true},
		GlobToolConfig:      &mfs.ToolConfig{Disable: true},
		GrepToolConfig:      &mfs.ToolConfig{Disable: true},
	}

	if cfg.Capabilities.Terminal {
		// The Shell field is independent of UseTerminalForFileTools: even if
		// callers don't want fs tools backed by terminal, the shell tool
		// itself is still exposed when the client supports it.
		logger := cfg.Logger
		if logger == nil {
			logger = slog.Default()
		}
		config.Shell = &shell{conn: cfg.Conn, sessionID: cfg.SessionID, logger: logger}
		if cfg.UseTerminalForFileTools {
			config.LsToolConfig = nil
			config.GlobToolConfig = nil
			config.GrepToolConfig = nil
		}
	}
	if b.hasReadFS {
		config.ReadFileToolConfig = nil
	}
	if b.hasWriteFS {
		config.WriteFileToolConfig = nil
	}
	if b.hasReadFS && b.hasWriteFS {
		config.EditFileToolConfig = nil
	}

	return mfs.New(ctx, config)
}

type shell struct {
	conn      ACPConn
	sessionID acpproto.SessionID
	logger    *slog.Logger
}

func (s *shell) Execute(ctx context.Context, input *filesystem.ExecuteRequest) (*filesystem.ExecuteResponse, error) {
	if input.RunInBackendGround {
		// Background execution would require a session-scoped handle for later release,
		// which is not modeled at this layer yet. Reject explicitly rather than leak terminals.
		return nil, errors.New("acp.shell: background execution is not supported over ACP")
	}

	createResp, err := s.conn.CreateTerminal(ctx, acpproto.CreateTerminalRequest{
		Command:   input.Command,
		SessionID: s.sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("acp.createTerminal session=%s: %w", s.sessionID, err)
	}
	defer func() {
		// Use a background context so that ReleaseTerminal still succeeds even
		// when the caller's ctx has been cancelled (e.g. user stop / timeout).
		bg, bgCancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTerminalTimeout)
		defer bgCancel()
		if _, rerr := s.conn.ReleaseTerminal(bg, acpproto.ReleaseTerminalRequest{
			SessionID:  s.sessionID,
			TerminalID: createResp.TerminalID,
		}); rerr != nil {
			cmd := input.Command
			if len(cmd) > maxLogCommandLen {
				cmd = cmd[:maxLogCommandLen] + "..."
			}
			s.logger.WarnContext(ctx, "acp.releaseTerminal failed",
				"session", s.sessionID, "terminal", createResp.TerminalID, "command", cmd, "err", rerr)
		}
	}()

	waitResp, err := s.conn.WaitForTerminalExit(ctx, acpproto.WaitForTerminalExitRequest{
		SessionID:  s.sessionID,
		TerminalID: createResp.TerminalID,
	})
	if err != nil {
		return nil, fmt.Errorf("acp.waitForTerminalExit session=%s: %w", s.sessionID, err)
	}

	outResp, err := s.conn.TerminalOutput(ctx, acpproto.TerminalOutputRequest{
		SessionID:  s.sessionID,
		TerminalID: createResp.TerminalID,
	})
	if err != nil {
		return nil, fmt.Errorf("acp.terminalOutput session=%s: %w", s.sessionID, err)
	}

	output := outResp.Output
	if waitResp.Signal != "" {
		output += fmt.Sprintf("\n[terminated by signal: %s]", waitResp.Signal)
	}

	ret := &filesystem.ExecuteResponse{
		Output:    output,
		ExitCode:  nil,
		Truncated: outResp.Truncated,
	}
	if outResp.ExitStatus != nil && outResp.ExitStatus.ExitCode != nil {
		code := int(*outResp.ExitStatus.ExitCode)
		ret.ExitCode = &code
	} else if waitResp.ExitCode != nil {
		code := int(*waitResp.ExitCode)
		ret.ExitCode = &code
	}

	return ret, nil
}

// Backend implements the mfs.Backend interface on top of an ACP agent
// connection, bridging filesystem and shell tool calls to ACP client-side
// capabilities (read_text_file / write_text_file / terminal). It is exported
// so callers that build their own filesystem middleware (instead of using
// NewClientToolsMiddleware) can plug it directly into mfs.MiddlewareConfig.
type Backend struct {
	conn       ACPConn
	sessionID  acpproto.SessionID
	shell      *shell
	hasReadFS  bool
	hasWriteFS bool

	// rgState stores the ripgrep probe result as an atomic int32 (rgProbeState).
	// Probing uses rgOnce-style synchronization: the first goroutine to find
	// state==unprobed wins the "probe race" via CAS and executes the RPC;
	// concurrent goroutines that arrive during probing fall back to POSIX grep
	// rather than blocking. Once determined, the result is cached permanently
	// (available/unavailable). Transient transport errors leave state as unprobed
	// so the next caller retries.
	rgState atomic.Int32

	// rgProbeMu serializes concurrent probe attempts so only one RPC flies at
	// a time. Goroutines that can't acquire it immediately return false (POSIX
	// fallback) rather than blocking.
	rgProbeMu sync.Mutex
}

// NewBackend constructs a Backend from the same Config used by
// NewClientToolsMiddleware. The returned Backend reflects the capabilities
// advertised by the client:
//
//   - Read/Write are enabled when cfg.Capabilities.FS.ReadTextFile /
//     WriteTextFile are set; otherwise calling them returns an error from the
//     underlying ACP RPC.
//   - LsInfo/GlobInfo/GrepRaw (terminal-backed implementations) are only
//     functional when cfg.Capabilities.Terminal is true AND
//     cfg.UseTerminalForFileTools is set; otherwise they return an error.
//   - Edit requires both ReadTextFile and WriteTextFile fs capabilities;
//     it performs a read-modify-write cycle via the fs API.
func NewBackend(cfg *Config) (*Backend, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	b := &Backend{conn: cfg.Conn, sessionID: cfg.SessionID}

	if cfg.Capabilities.Terminal && cfg.UseTerminalForFileTools {
		b.shell = &shell{conn: cfg.Conn, sessionID: cfg.SessionID, logger: logger}
	}
	if cfg.Capabilities.FS != nil {
		b.hasReadFS = cfg.Capabilities.FS.ReadTextFile
		b.hasWriteFS = cfg.Capabilities.FS.WriteTextFile
	}

	return b, nil
}

func (b *Backend) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	var limit, line *int64
	if req.Limit != 0 {
		tmp := int64(req.Limit)
		limit = &tmp
	}
	if req.Offset > 1 {
		tmp := int64(req.Offset)
		line = &tmp
	}
	resp, err := b.conn.ReadTextFile(ctx, acpproto.ReadTextFileRequest{
		Limit:     limit,
		Line:      line,
		Path:      req.FilePath,
		SessionID: b.sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("acp.readTextFile session=%s path=%s: %w", b.sessionID, req.FilePath, err)
	}
	return &filesystem.FileContent{Content: resp.Content}, nil
}

func (b *Backend) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	_, err := b.conn.WriteTextFile(ctx, acpproto.WriteTextFileRequest{
		Content:   req.Content,
		Path:      req.FilePath,
		SessionID: b.sessionID,
	})
	if err != nil {
		return fmt.Errorf("acp.writeTextFile session=%s path=%s: %w", b.sessionID, req.FilePath, err)
	}
	return nil
}

func (b *Backend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	if b.shell == nil {
		return nil, fmt.Errorf("%w: ls info", ErrCapabilityMissing)
	}
	dir := req.Path
	if dir == "" {
		dir = "."
	}
	quotedDir, err := shellQuote(dir)
	if err != nil {
		return nil, fmt.Errorf("acp.shell ls path=%s: %w", dir, err)
	}
	result, err := b.runShell(ctx, "ls -1Ap -- "+quotedDir)
	if err != nil {
		return nil, fmt.Errorf("acp.shell ls path=%s: %w", dir, err)
	}
	if result.Truncated {
		b.shell.logger.WarnContext(ctx, "acp.shell ls: output truncated, returning partial results", "path", dir)
	}
	var infos []filesystem.FileInfo
	for _, line := range strings.Split(strings.TrimRight(result.Output, "\n"), "\n") {
		if line == "" {
			continue
		}
		info := filesystem.FileInfo{}
		if strings.HasSuffix(line, "/") {
			info.Path = strings.TrimSuffix(line, "/")
			info.IsDir = true
		} else {
			info.Path = line
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (b *Backend) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	if b.shell == nil {
		return nil, fmt.Errorf("%w: grep raw", ErrCapabilityMissing)
	}
	if b.detectRipgrep(ctx) {
		return b.grepWithRipgrep(ctx, req)
	}
	return b.grepWithPosix(ctx, req)
}

// rgProbeState represents the three-state probe result for ripgrep availability.
type rgProbeState = int32

const (
	rgUnprobed    rgProbeState = iota // not yet probed
	rgAvailable                       // rg is installed
	rgUnavailable                     // rg is confirmed not installed
)

// detectRipgrep probes for ripgrep availability on the client side using
// `command -v rg`. The result is cached once determined definitively (rg found
// or confirmed absent). If the probe fails due to a transient transport error
// (e.g. ctx cancelled), the state remains "unprobed" so the next call retries.
//
// Concurrency: uses a non-blocking TryLock so that concurrent grep calls don't
// serialize behind a slow probe RPC. If another goroutine is already probing,
// latecomers fall back to POSIX grep for that single call rather than waiting.
func (b *Backend) detectRipgrep(ctx context.Context) bool {
	// Fast path: already probed.
	state := b.rgState.Load()
	if state != rgUnprobed {
		return state == rgAvailable
	}

	// Slow path: try to become the probe goroutine. If someone else is already
	// probing, fall back to POSIX grep for this call (non-blocking).
	if !b.rgProbeMu.TryLock() {
		return false
	}
	defer b.rgProbeMu.Unlock()

	// Double-check after acquiring the lock: another goroutine may have finished
	// probing between our Load() and TryLock().
	state = b.rgState.Load()
	if state != rgUnprobed {
		return state == rgAvailable
	}

	resp, err := b.shell.Execute(ctx, &filesystem.ExecuteRequest{Command: "command -v rg >/dev/null 2>&1"})
	if err != nil {
		// Transport error — don't cache, retry next time.
		b.shell.logger.DebugContext(ctx, "acp.shell grep: rg detection probe failed, will retry", "err", err)
		return false
	}
	if resp.ExitCode != nil && *resp.ExitCode == 0 {
		b.rgState.Store(rgAvailable)
		return true
	}
	b.shell.logger.InfoContext(ctx, "acp.shell grep: ripgrep (rg) not found on client, falling back to POSIX grep")
	b.rgState.Store(rgUnavailable)
	return false
}

func (b *Backend) grepWithRipgrep(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	args := []string{"rg", "--json"}
	if req.CaseInsensitive {
		args = append(args, "--ignore-case")
	}
	if req.EnableMultiline {
		args = append(args, "--multiline", "--multiline-dotall")
	}
	if req.FileType != "" {
		args = append(args, "--type", req.FileType)
	}
	if req.Glob != "" {
		args = append(args, "--glob", req.Glob)
	}
	if req.BeforeLines > 0 {
		args = append(args, "--before-context", strconv.Itoa(req.BeforeLines))
	}
	if req.AfterLines > 0 {
		args = append(args, "--after-context", strconv.Itoa(req.AfterLines))
	}
	args = append(args, "--", req.Pattern)
	if req.Path != "" {
		args = append(args, req.Path)
	} else {
		args = append(args, ".")
	}

	cmd, err := joinShellArgs(args)
	if err != nil {
		return nil, fmt.Errorf("acp.shell grep: %w", err)
	}
	resp, err := b.shell.Execute(ctx, &filesystem.ExecuteRequest{Command: cmd})
	if err != nil {
		return nil, fmt.Errorf("acp.shell grep: %w", err)
	}
	// rg exit code 1 means "no matches" — not an error.
	if resp.ExitCode != nil && *resp.ExitCode != 0 && *resp.ExitCode != 1 {
		return nil, fmt.Errorf("acp.shell grep (exit %d): %w: %s", *resp.ExitCode, ErrShellNonZeroExit, resp.Output)
	}
	if resp.ExitCode != nil && *resp.ExitCode == 1 {
		return nil, nil
	}
	if resp.Truncated {
		b.shell.logger.WarnContext(ctx, "acp.shell grep: output truncated, returning partial results")
	}
	return parseRipgrepJSONOutput(resp.Output, b.shell.logger)
}

// grepWithPosix is the fallback used when ripgrep (rg) is not installed on
// the client. It shells out to POSIX `grep -RnE` (with widely-supported BSD/GNU
// extensions: -R recursive, -n line numbers, -E extended regex, -H force path
// prefix, --include for glob filtering, -A/-B for context).
//
// Limitations vs. ripgrep:
//   - EnableMultiline is rejected: POSIX grep operates line-by-line, and
//     silently ignoring this flag would change match semantics.
//   - FileType is best-effort: POSIX grep has no built-in type registry, so
//     the flag is ignored with a warning. Callers wanting strict type filtering
//     should install rg or pass an equivalent Glob.
func (b *Backend) grepWithPosix(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	if req.EnableMultiline {
		return nil, errors.New("acp.shell grep: EnableMultiline requires ripgrep (rg); not supported by POSIX grep fallback")
	}
	if req.FileType != "" {
		b.shell.logger.WarnContext(ctx, "acp.shell grep: FileType is ignored in POSIX grep fallback (rg not installed)", "fileType", req.FileType)
	}
	args := []string{"grep", "-RHnE", "--no-messages"}
	if req.CaseInsensitive {
		args = append(args, "-i")
	}
	if req.BeforeLines > 0 {
		args = append(args, "-B", strconv.Itoa(req.BeforeLines))
	}
	if req.AfterLines > 0 {
		args = append(args, "-A", strconv.Itoa(req.AfterLines))
	}
	if req.Glob != "" {
		args = append(args, "--include="+req.Glob)
	}
	args = append(args, "-e", req.Pattern)
	if req.Path != "" {
		args = append(args, req.Path)
	} else {
		args = append(args, ".")
	}

	cmd, err := joinShellArgs(args)
	if err != nil {
		return nil, fmt.Errorf("acp.shell grep: %w", err)
	}
	resp, err := b.shell.Execute(ctx, &filesystem.ExecuteRequest{Command: cmd})
	if err != nil {
		return nil, fmt.Errorf("acp.shell grep: %w", err)
	}
	// grep exit codes: 0 = matches found, 1 = no matches, >=2 = error.
	if resp.ExitCode != nil && *resp.ExitCode >= 2 {
		return nil, fmt.Errorf("acp.shell grep (exit %d): %w: %s", *resp.ExitCode, ErrShellNonZeroExit, resp.Output)
	}
	if resp.ExitCode != nil && *resp.ExitCode == 1 {
		return nil, nil
	}
	if resp.Truncated {
		b.shell.logger.WarnContext(ctx, "acp.shell grep: output truncated, returning partial results")
	}
	return parsePosixGrepOutput(resp.Output), nil
}

func (b *Backend) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	if b.shell == nil {
		return nil, fmt.Errorf("%w: glob info", ErrCapabilityMissing)
	}
	basePath := req.Path
	if basePath == "" {
		basePath = "."
	}
	basePath = path.Clean(basePath)

	// We don't use runShell here because `find` may exit with code 1 when it
	// encounters permission-denied directories, while still producing valid
	// output on stdout. runShell treats any non-zero exit as a hard error,
	// which would discard legitimate results.
	quotedBase, err := shellQuote(basePath)
	if err != nil {
		return nil, fmt.Errorf("acp.shell glob path=%s: %w", basePath, err)
	}
	cmd := fmt.Sprintf("find %s -maxdepth %d 2>/dev/null", quotedBase, findMaxDepth)
	resp, err := b.shell.Execute(ctx, &filesystem.ExecuteRequest{Command: cmd})
	if err != nil {
		return nil, fmt.Errorf("acp.shell glob path=%s: %w", basePath, err)
	}
	// find exit codes: 0 = success, 1 = partial failure (e.g. permission denied
	// on some subdirectories but stdout still has results). Only treat exit >= 2
	// or nil (signal-killed) as hard errors.
	if resp.ExitCode == nil {
		return nil, fmt.Errorf("acp.shell glob path=%s: %w: process terminated without exit code",
			basePath, ErrShellNonZeroExit)
	}
	if *resp.ExitCode >= 2 {
		return nil, fmt.Errorf("acp.shell glob path=%s (exit %d): %w: %s",
			basePath, *resp.ExitCode, ErrShellNonZeroExit, resp.Output)
	}
	if resp.Truncated {
		b.shell.logger.WarnContext(ctx, "acp.shell glob: output truncated, returning partial results", "path", basePath)
	}

	isAbsolutePattern := strings.HasPrefix(req.Pattern, "/")
	var infos []filesystem.FileInfo
	for _, p := range strings.Split(strings.TrimRight(resp.Output, "\n"), "\n") {
		if p == "" {
			continue
		}
		var matchPath, resultPath string
		if isAbsolutePattern {
			matchPath = p
			resultPath = p
		} else {
			rel := p
			if basePath == "." {
				rel = strings.TrimPrefix(rel, "./")
			} else {
				rel = strings.TrimPrefix(rel, basePath)
				rel = strings.TrimPrefix(rel, "/")
			}
			matchPath = rel
			resultPath = rel
		}
		matched, err := doublestar.Match(req.Pattern, matchPath)
		if err != nil {
			return nil, fmt.Errorf("acp.shell glob: invalid pattern %q: %w", req.Pattern, err)
		}
		if matched {
			infos = append(infos, filesystem.FileInfo{Path: resultPath})
		}
	}
	return infos, nil
}

func (b *Backend) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	if req.OldString == "" {
		return errors.New("oldString must be non-empty")
	}
	if req.OldString == req.NewString {
		return nil
	}

	if !(b.hasReadFS && b.hasWriteFS) {
		return fmt.Errorf("%w: edit requires fs.ReadTextFile and fs.WriteTextFile", ErrCapabilityMissing)
	}
	return b.editViaFS(ctx, req)
}

// readFull reads an entire file via the ACP ReadTextFile RPC (no offset/limit).
// Used by both Read (when no offset) and Edit for the read-modify-write cycle.
func (b *Backend) readFull(ctx context.Context, filePath string) (string, error) {
	resp, err := b.conn.ReadTextFile(ctx, acpproto.ReadTextFileRequest{
		Path:      filePath,
		SessionID: b.sessionID,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (b *Backend) editViaFS(ctx context.Context, req *filesystem.EditRequest) error {
	content, err := b.readFull(ctx, req.FilePath)
	if err != nil {
		return fmt.Errorf("acp.edit read %s: %w", req.FilePath, err)
	}
	if len(content) > maxEditFileSize {
		return fmt.Errorf("acp.edit %s: file is %d bytes: %w", req.FilePath, len(content), ErrFileTooLarge)
	}
	count := strings.Count(content, req.OldString)
	switch {
	case count == 0:
		return fmt.Errorf("acp.edit %s: %w", req.FilePath, ErrOldStringNotFound)
	case count > 1 && !req.ReplaceAll:
		return fmt.Errorf("acp.edit %s: %d occurrences found: %w", req.FilePath, count, ErrAmbiguousReplace)
	}
	if req.ReplaceAll {
		content = strings.ReplaceAll(content, req.OldString, req.NewString)
	} else {
		content = strings.Replace(content, req.OldString, req.NewString, 1)
	}
	_, err = b.conn.WriteTextFile(ctx, acpproto.WriteTextFileRequest{
		Path:      req.FilePath,
		Content:   content,
		SessionID: b.sessionID,
	})
	if err != nil {
		return fmt.Errorf("acp.edit write %s: %w", req.FilePath, err)
	}
	return nil
}

// shellResult holds the output of a shell command execution along with
// metadata about whether the output was truncated.
type shellResult struct {
	Output    string
	Truncated bool
}

func (b *Backend) runShell(ctx context.Context, cmd string) (*shellResult, error) {
	resp, err := b.shell.Execute(ctx, &filesystem.ExecuteRequest{Command: cmd})
	if err != nil {
		return nil, err
	}
	if resp.ExitCode == nil {
		// Process was terminated by signal or exited without a code — treat as error.
		return nil, fmt.Errorf("%w: process terminated without exit code (output=%q)",
			ErrShellNonZeroExit, truncateStr(resp.Output, maxLogCommandLen))
	}
	if *resp.ExitCode != 0 {
		return nil, fmt.Errorf("%w: exit %d: %s", ErrShellNonZeroExit, *resp.ExitCode, resp.Output)
	}
	return &shellResult{
		Output:    resp.Output,
		Truncated: resp.Truncated,
	}, nil
}

// truncateStr shortens s to at most maxLen bytes, appending "..." if truncated.
// It avoids splitting multi-byte UTF-8 characters by backing off to a valid rune boundary.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Back off to avoid cutting a multi-byte rune in half.
	for maxLen > 0 && !utf8.ValidString(s[:maxLen]) {
		maxLen--
	}
	return s[:maxLen] + "..."
}

// shellQuote wraps s in single quotes for safe inclusion in a sh/bash/zsh command.
// It rejects strings containing null bytes, which shells cannot represent in arguments.
func shellQuote(s string) (string, error) {
	if strings.ContainsRune(s, 0) {
		return "", errors.New("acp.shell: argument contains null byte")
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'", nil
}

func joinShellArgs(args []string) (string, error) {
	quoted := make([]string, len(args))
	for i, a := range args {
		q, err := shellQuote(a)
		if err != nil {
			return "", err
		}
		quoted[i] = q
	}
	return strings.Join(quoted, " "), nil
}

// rgJSONMatch represents a single match entry from `rg --json` output.
type rgJSONMatch struct {
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

// parseRipgrepJSONOutput parses `rg --json` output format. Each line is a JSON
// object; we only extract entries with type=="match". This correctly handles
// filenames containing ':' or other special characters.
// Returns an error if the output is non-empty but every line fails to parse
// (e.g. incompatible rg version).
func parseRipgrepJSONOutput(out string, logger *slog.Logger) ([]filesystem.GrepMatch, error) {
	var matches []filesystem.GrepMatch
	var totalLines, failedLines int
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		totalLines++
		var entry rgJSONMatch
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			failedLines++
			logger.Warn("acp.grep: failed to parse rg JSON line", "err", err, "line", line)
			continue
		}
		if entry.Type != "match" {
			continue
		}
		content := entry.Data.Lines.Text
		// rg --json includes trailing newline in lines.text; trim it.
		content = strings.TrimSuffix(content, "\n")
		matches = append(matches, filesystem.GrepMatch{
			Path:    entry.Data.Path.Text,
			Line:    entry.Data.LineNumber,
			Content: content,
		})
	}
	if totalLines > 0 && failedLines == totalLines {
		return nil, fmt.Errorf("acp.grep: all %d output lines failed JSON parsing (possible rg version incompatibility)", totalLines)
	}
	if failedLines > 0 && float64(failedLines)/float64(totalLines) >= rgJSONMaxFailedRatio {
		return nil, fmt.Errorf("acp.grep: %d/%d output lines failed JSON parsing (possible rg version incompatibility)", failedLines, totalLines)
	}
	return matches, nil
}

// parsePosixGrepOutput parses `grep -RHn` style output where each match line
// is `<path>:<lineno>:<content>` and context lines use `-` as the separator.
// `--` group separators emitted by -A/-B are skipped. Lines that don't match
// the expected shape are skipped (best-effort), since `grep --no-messages` may
// still emit content from binary files in unusual encodings.
//
// To handle paths containing colons, we search from the right for the last
// occurrence of `:<digits>:` or `:<digits>-` boundary, which more reliably
// identifies the line-number field.
func parsePosixGrepOutput(out string) []filesystem.GrepMatch {
	var matches []filesystem.GrepMatch
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" || line == "--" {
			continue
		}
		filePath, lineno, content, ok := parsePosixGrepLine(line)
		if !ok {
			continue
		}
		matches = append(matches, filesystem.GrepMatch{
			Path:    filePath,
			Line:    lineno,
			Content: content,
		})
	}
	return matches
}

// parsePosixGrepLine parses a single line of grep -RHn output. The format is:
//
//	<path><sep><lineno><sep><content>
//
// where <sep> is ':' (match line) or '-' (context line from -A/-B).
//
// Algorithm: scan left-to-right for the first <sep><digits><sep> boundary.
// Everything before the leading separator is the path; everything after the
// trailing separator is the content. This correctly handles paths containing
// ':' (e.g. "vendor/pkg:v2/file.go") as long as the path does not contain a
// substring matching :<positive-integer><sep> before the real line number.
//
// Known limitation: if the path itself contains a `:N:` or `:N-` pattern where
// N is a positive integer (e.g. "prefix:1:rest.go"), the parser will split incorrectly.
// POSIX grep has no NUL-delimited output mode (`-Z` is GNU-only), so this is
// an inherent best-effort trade-off.
func parsePosixGrepLine(line string) (filePath string, lineno int, content string, ok bool) {
	n := len(line)
	// Find first occurrence of <sep><digits><sep> scanning left-to-right.
	i := 0
	for i < n {
		// Find next ':' or '-' separator.
		sep1 := -1
		for j := i; j < n; j++ {
			if line[j] == ':' || line[j] == '-' {
				sep1 = j
				break
			}
		}
		if sep1 < 0 || sep1+1 >= n {
			return "", 0, "", false
		}
		// Check if digits follow.
		digitStart := sep1 + 1
		digitEnd := digitStart
		for digitEnd < n && line[digitEnd] >= '0' && line[digitEnd] <= '9' {
			digitEnd++
		}
		if digitEnd == digitStart || digitEnd >= n {
			// No digits found after this separator, or digits run to end of line.
			i = sep1 + 1
			continue
		}
		// Check that digits are followed by another separator.
		if line[digitEnd] != ':' && line[digitEnd] != '-' {
			i = sep1 + 1
			continue
		}
		// We have a valid <sep><digits><sep> at sep1..digitEnd.
		num, err := strconv.Atoi(line[digitStart:digitEnd])
		if err != nil || num <= 0 {
			i = sep1 + 1
			continue
		}
		filePath = line[:sep1]
		content = line[digitEnd+1:]
		return filePath, num, content, true
	}
	return "", 0, "", false
}
