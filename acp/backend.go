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

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	mfs "github.com/cloudwego/eino/adk/middlewares/filesystem"
	acpproto "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
)

// NewClientToolsMiddleware creates a ChatModelAgentMiddleware that bridges ACP client-side capabilities
// (filesystem read/write, terminal execution) to eino's filesystem tools. The ACP protocol only exposes
// read_text_file, write_text_file and terminal capabilities, so ls/edit/glob/grep tools are always disabled;
// read_file, write_file and terminal are enabled only when the client advertises the corresponding capability.
func NewClientToolsMiddleware(
	ctx context.Context,
	sessionID acpproto.SessionID,
	capabilities *acpproto.ClientCapabilities,
	conn *acpconn.AgentConnection,
) (adk.ChatModelAgentMiddleware, error) {
	config := &mfs.MiddlewareConfig{
		Backend:             &backend{conn: conn, sessionID: sessionID},
		LsToolConfig:        &mfs.ToolConfig{Disable: true},
		ReadFileToolConfig:  &mfs.ToolConfig{Disable: true},
		WriteFileToolConfig: &mfs.ToolConfig{Disable: true},
		EditFileToolConfig:  &mfs.ToolConfig{Disable: true},
		GlobToolConfig:      &mfs.ToolConfig{Disable: true},
		GrepToolConfig:      &mfs.ToolConfig{Disable: true},
	}
	if capabilities != nil {
		if capabilities.Terminal {
			config.Shell = &shell{conn: conn, sessionID: sessionID}
		}
		if capabilities.FS != nil {
			if capabilities.FS.WriteTextFile {
				config.WriteFileToolConfig = nil
			}
			if capabilities.FS.ReadTextFile {
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

func (b *backend) LsInfo(_ context.Context, _ *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	return nil, fmt.Errorf("acp client does not support ls info")
}

func (b *backend) GrepRaw(_ context.Context, _ *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	return nil, fmt.Errorf("acp client does not support grep raw")
}

func (b *backend) GlobInfo(_ context.Context, _ *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	return nil, fmt.Errorf("acp client does not support glob info")
}

func (b *backend) Edit(_ context.Context, _ *filesystem.EditRequest) error {
	return fmt.Errorf("acp client does not support edit")
}
