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

package acp

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	mfs "github.com/cloudwego/eino/adk/middlewares/filesystem"
	acpproto "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
)

// NewACPClientToolsMiddleware creates a ChatModelAgentMiddleware that bridges ACP client-side capabilities
// (filesystem read/write, terminal execution) to eino's filesystem tools. It inspects the client's advertised
// capabilities and enables the corresponding tools: read_file, write_file, and terminal commands are
// proxied through the ACP connection back to the client. Tools not supported by the client are disabled.
func NewACPClientToolsMiddleware(
	ctx context.Context,
	sessionID acpproto.SessionID,
	capacities *acpproto.ClientCapabilities,
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
	if capacities != nil {
		if capacities.Terminal {
			config.Shell = &shell{conn: conn, sessionID: sessionID}
		}
		if capacities.FS != nil {
			if capacities.FS.WriteTextFile {
				config.WriteFileToolConfig = nil
			}
			if capacities.FS.ReadTextFile {
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

func (s *shell) Execute(ctx context.Context, input *filesystem.ExecuteRequest) (result *filesystem.ExecuteResponse, err error) {
	createResp, err := s.conn.CreateTerminal(ctx, acpproto.CreateTerminalRequest{
		Args:      nil, // todo: if args needed?
		Command:   input.Command,
		SessionID: s.sessionID,
	})
	if err != nil {
		return nil, err
	}

	if input.RunInBackendGround {
		return &filesystem.ExecuteResponse{
			Output:   "command started in background\n",
			ExitCode: new(int),
		}, nil
	}

	waitResp, err := s.conn.WaitForTerminalExit(ctx, acpproto.WaitForTerminalExitRequest{
		SessionID:  s.sessionID,
		TerminalID: createResp.TerminalID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait command executing: %w", err)
	}

	if waitResp.ExitCode != nil && *waitResp.ExitCode != 0 {
		code := int(*waitResp.ExitCode)
		return &filesystem.ExecuteResponse{
			Output:    waitResp.Signal,
			ExitCode:  &code,
			Truncated: false,
		}, nil
	}

	resultResp, err := s.conn.TerminalOutput(ctx, acpproto.TerminalOutputRequest{
		SessionID:  s.sessionID,
		TerminalID: createResp.TerminalID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait get command result: %w", err)
	}

	ret := &filesystem.ExecuteResponse{
		Output:    resultResp.Output,
		ExitCode:  nil,
		Truncated: resultResp.Truncated,
	}
	if resultResp.ExitStatus != nil && resultResp.ExitStatus.ExitCode != nil {
		code := int(*resultResp.ExitStatus.ExitCode)
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

func (b backend) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
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
		return nil, err
	}
	return &filesystem.FileContent{Content: resp.Content}, nil
}

func (b backend) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	_, err := b.conn.WriteTextFile(ctx, acpproto.WriteTextFileRequest{
		Content:   req.Content,
		Path:      req.FilePath,
		SessionID: b.sessionID,
	})
	return err
}

func (b backend) LsInfo(_ context.Context, _ *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	return nil, fmt.Errorf("acp client does not support ls info")
}

func (b backend) GrepRaw(_ context.Context, _ *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	return nil, fmt.Errorf("acp client does not support grep raw")
}

func (b backend) GlobInfo(_ context.Context, _ *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	return nil, fmt.Errorf("acp client does not support glob info")
}

func (b backend) Edit(_ context.Context, _ *filesystem.EditRequest) error {
	return fmt.Errorf("acp client does not support edit")
}
