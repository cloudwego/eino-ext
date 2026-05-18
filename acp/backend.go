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
	"fmt"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	mfs "github.com/cloudwego/eino/adk/middlewares/filesystem"
	acpproto "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
)

// Config configures NewClientToolsMiddleware.
type Config struct {
	// SessionID is the ACP session the middleware will operate on. Required.
	SessionID acpproto.SessionID
	// Conn is the agent-side ACP connection used to issue client requests. Required.
	Conn *acpconn.AgentConnection
	// Capabilities is the client capability set advertised during initialization.
	// Required: tools are enabled based on what the client supports.
	Capabilities *acpproto.ClientCapabilities

	// UseTerminalForFileTools enables terminal-backed implementations of the
	// ls, glob, and grep tools. It only takes effect when the client also
	// advertises the terminal capability; otherwise those tools stay disabled
	// because the ACP protocol does not expose corresponding filesystem methods.
	//
	// Implementation: ls runs `ls -1A`, glob enumerates with `find` and matches
	// in-process via doublestar, and grep shells out to ripgrep (`rg`). If `rg`
	// is not installed on the client side, grep calls will fail.
	UseTerminalForFileTools bool
}

// NewClientToolsMiddleware creates a ChatModelAgentMiddleware that bridges ACP client-side capabilities
// (filesystem read/write, terminal execution) to eino's filesystem tools. The ACP protocol only exposes
// read_text_file, write_text_file and terminal capabilities, so edit is always disabled; read_file,
// write_file and terminal are enabled only when the client advertises the corresponding capability.
// ls/glob/grep are disabled by default; they become available when cfg.UseTerminalForFileTools is true
// and the client advertises the terminal capability — in which case they run as shell commands.
func NewClientToolsMiddleware(ctx context.Context, cfg *Config) (adk.ChatModelAgentMiddleware, error) {
	if cfg == nil {
		return nil, errors.New("acp.NewClientToolsMiddleware: cfg is required")
	}
	if cfg.Conn == nil {
		return nil, errors.New("acp.NewClientToolsMiddleware: cfg.Conn is required")
	}
	if cfg.SessionID == "" {
		return nil, errors.New("acp.NewClientToolsMiddleware: cfg.SessionID is required")
	}

	b := &backend{conn: cfg.Conn, sessionID: cfg.SessionID}
	config := &mfs.MiddlewareConfig{
		Backend:             b,
		LsToolConfig:        &mfs.ToolConfig{Disable: true},
		ReadFileToolConfig:  &mfs.ToolConfig{Disable: true},
		WriteFileToolConfig: &mfs.ToolConfig{Disable: true},
		EditFileToolConfig:  &mfs.ToolConfig{Disable: true},
		GlobToolConfig:      &mfs.ToolConfig{Disable: true},
		GrepToolConfig:      &mfs.ToolConfig{Disable: true},
	}
	if cfg.Capabilities != nil {
		if cfg.Capabilities.Terminal {
			sh := &shell{conn: cfg.Conn, sessionID: cfg.SessionID}
			config.Shell = sh
			if cfg.UseTerminalForFileTools {
				b.shell = sh
				config.LsToolConfig = nil
				config.GlobToolConfig = nil
				config.GrepToolConfig = nil
			}
		}
		if cfg.Capabilities.FS != nil {
			if cfg.Capabilities.FS.WriteTextFile {
				config.WriteFileToolConfig = nil
			}
			if cfg.Capabilities.FS.ReadTextFile {
				config.ReadFileToolConfig = nil
			}
		}
	}
	return mfs.New(ctx, config)
}

type shell struct {
	conn      *acpconn.AgentConnection
	sessionID acpproto.SessionID
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
		_, _ = s.conn.ReleaseTerminal(ctx, acpproto.ReleaseTerminalRequest{
			SessionID:  s.sessionID,
			TerminalID: createResp.TerminalID,
		})
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

type backend struct {
	conn      *acpconn.AgentConnection
	sessionID acpproto.SessionID
	shell     *shell
}

func (b *backend) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
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

func (b *backend) Write(ctx context.Context, req *filesystem.WriteRequest) error {
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

func (b *backend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	if b.shell == nil {
		return nil, fmt.Errorf("acp client does not support ls info")
	}
	path := req.Path
	if path == "" {
		path = "."
	}
	out, err := b.runShell(ctx, "ls -1A -- "+shellQuote(path))
	if err != nil {
		return nil, fmt.Errorf("acp.shell ls path=%s: %w", path, err)
	}
	var infos []filesystem.FileInfo
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		infos = append(infos, filesystem.FileInfo{Path: line})
	}
	return infos, nil
}

func (b *backend) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	if b.shell == nil {
		return nil, fmt.Errorf("acp client does not support grep raw")
	}
	args := []string{"rg", "--line-number", "--no-heading", "--color=never"}
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

	resp, err := b.shell.Execute(ctx, &filesystem.ExecuteRequest{Command: joinShellArgs(args)})
	if err != nil {
		return nil, fmt.Errorf("acp.shell grep: %w", err)
	}
	// rg exit code 1 means "no matches" — not an error.
	if resp.ExitCode != nil && *resp.ExitCode != 0 && *resp.ExitCode != 1 {
		return nil, fmt.Errorf("acp.shell grep failed (exit %d): %s", *resp.ExitCode, resp.Output)
	}
	if resp.ExitCode != nil && *resp.ExitCode == 1 {
		return nil, nil
	}
	return parseRipgrepOutput(resp.Output), nil
}

func (b *backend) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	if b.shell == nil {
		return nil, fmt.Errorf("acp client does not support glob info")
	}
	basePath := req.Path
	if basePath == "" {
		basePath = "."
	}
	out, err := b.runShell(ctx, "find "+shellQuote(basePath)+" -type f")
	if err != nil {
		return nil, fmt.Errorf("acp.shell glob path=%s: %w", basePath, err)
	}

	isAbsolutePattern := strings.HasPrefix(req.Pattern, "/")
	var infos []filesystem.FileInfo
	for _, p := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
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

func (b *backend) Edit(_ context.Context, _ *filesystem.EditRequest) error {
	return fmt.Errorf("acp client does not support edit")
}

func (b *backend) runShell(ctx context.Context, cmd string) (string, error) {
	resp, err := b.shell.Execute(ctx, &filesystem.ExecuteRequest{Command: cmd})
	if err != nil {
		return "", err
	}
	if resp.ExitCode != nil && *resp.ExitCode != 0 {
		return "", fmt.Errorf("exit %d: %s", *resp.ExitCode, resp.Output)
	}
	return resp.Output, nil
}

// shellQuote wraps s in single quotes for safe inclusion in a sh/bash/zsh command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func joinShellArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}

// parseRipgrepOutput parses output of `rg --line-number --no-heading`. Each
// match line is "path:line:content"; each context line (when -A/-B/-C is set)
// is "path:line-content". Group separators ("--") and unparsable lines are
// skipped. Filenames containing ':' are not handled — they would be skipped.
func parseRipgrepOutput(out string) []filesystem.GrepMatch {
	var matches []filesystem.GrepMatch
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		m, ok := parseRipgrepLine(line)
		if !ok {
			continue
		}
		matches = append(matches, m)
	}
	return matches
}

func parseRipgrepLine(line string) (filesystem.GrepMatch, bool) {
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return filesystem.GrepMatch{}, false
	}
	path := line[:i]
	rest := line[i+1:]

	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j == 0 || j >= len(rest) {
		return filesystem.GrepMatch{}, false
	}
	if sep := rest[j]; sep != ':' && sep != '-' {
		return filesystem.GrepMatch{}, false
	}
	lineNum, err := strconv.Atoi(rest[:j])
	if err != nil {
		return filesystem.GrepMatch{}, false
	}
	return filesystem.GrepMatch{Path: path, Line: lineNum, Content: rest[j+1:]}, true
}
