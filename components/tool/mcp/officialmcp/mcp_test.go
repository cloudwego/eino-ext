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

package officialmcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testImpl = &mcp.Implementation{Name: "test", Version: "v1.0.0"}

type SayHiParams struct {
	Name string `json:"name"`
}

func SayHi(ctx context.Context, req *mcp.CallToolRequest, args SayHiParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hi " + args.Name},
		},
	}, nil, nil
}

func SayHello(ctx context.Context, req *mcp.CallToolRequest, args SayHiParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hello " + args.Name},
		},
	}, nil, nil
}

func TestTool(t *testing.T) {
	ctx := context.Background()

	server := mcp.NewServer(testImpl, nil)
	client := mcp.NewClient(testImpl, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	// add tools to server
	mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)
	mcp.AddTool(server, &mcp.Tool{Name: "hello", Description: "say hello"}, SayHello)

	// get tools from client, only greet tool
	tools, err := GetTools(ctx, &Config{Cli: clientSession, ToolNameList: []string{"greet"}})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tools))
	info, err := tools[0].Info(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "greet", info.Name)

	result, err := tools[0].(tool.InvokableTool).InvokableRun(ctx, "{\"name\": \"eino\"}")
	assert.NoError(t, err)
	fmt.Println(result)
	assert.Equal(t, "{\"content\":[{\"type\":\"text\",\"text\":\"Hi eino\"}]}", result)
}

func TestToolNameMapperMetadataAndRawInvocation(t *testing.T) {
	ctx := context.Background()
	clientSession, cleanup := newTestClientSession(t, nil)
	defer cleanup()

	tools, err := GetTools(ctx, &Config{
		Cli:        clientSession,
		ServerName: "docs",
		ToolNameMapper: func(ctx context.Context, info ToolNameMapperInput) (ToolNameMapperOutput, error) {
			return ToolNameMapperOutput{
				ExposedName: "mcp__" + info.ServerName + "__" + info.Tool.Name,
				Extra:       map[string]any{"runtime": "test"},
			}, nil
		},
		ToolNameList: []string{"greet"},
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)

	info, err := tools[0].Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "mcp__docs__greet", info.Name)
	assert.Equal(t, "docs", info.Extra[ExtraMCPServerName])
	assert.Equal(t, "greet", info.Extra[ExtraMCPRawToolName])
	assert.Equal(t, "mcp__docs__greet", info.Extra[ExtraMCPExposedToolName])
	assert.Equal(t, "test", info.Extra["runtime"])
	require.NoError(t, jsonRoundTrip(info))

	result, err := tools[0].(tool.InvokableTool).InvokableRun(ctx, "{\"name\": \"eino\"}")
	require.NoError(t, err)
	assert.Equal(t, "{\"content\":[{\"type\":\"text\",\"text\":\"Hi eino\"}]}", result)
}

func TestToolNameMapperRejectsDuplicateAndReservedExtra(t *testing.T) {
	ctx := context.Background()
	clientSession, cleanup := newTestClientSession(t, nil)
	defer cleanup()

	_, err := GetTools(ctx, &Config{
		Cli: clientSession,
		ToolNameMapper: func(ctx context.Context, info ToolNameMapperInput) (ToolNameMapperOutput, error) {
			return ToolNameMapperOutput{ExposedName: "duplicate"}, nil
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate official mcp exposed tool name")

	_, err = GetTools(ctx, &Config{
		Cli: clientSession,
		ToolNameMapper: func(ctx context.Context, info ToolNameMapperInput) (ToolNameMapperOutput, error) {
			return ToolNameMapperOutput{
				ExposedName: info.Tool.Name,
				Extra:       map[string]any{ExtraMCPRawToolName: "override"},
			}, nil
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved key")
}

func TestMetadataFullAndAnnotations(t *testing.T) {
	ctx := context.Background()
	readOnly := true
	clientSession, cleanup := newTestClientSession(t, func(server *mcp.Server) {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "annotated",
			Description: "annotated tool",
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:  true,
				OpenWorldHint: &readOnly,
				Title:         "Annotated",
			},
		}, SayHi)
	})
	defer cleanup()

	tools, err := GetTools(ctx, &Config{
		Cli:          clientSession,
		ServerName:   "docs",
		ToolNameList: []string{"annotated"},
		MetadataMode: MetadataFull,
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)
	info, err := tools[0].Info(ctx)
	require.NoError(t, err)

	annotations := info.Extra[ExtraMCPAnnotations].(map[string]any)
	assert.Equal(t, true, annotations["readOnlyHint"])
	assert.Equal(t, true, annotations["openWorldHint"])
	assert.Equal(t, "Annotated", annotations["title"])
	assert.Contains(t, info.Extra, ExtraMCPRawTool)
	require.NoError(t, jsonRoundTrip(info))
}

func TestToolCallResultHandlers(t *testing.T) {
	ctx := context.Background()
	clientSession, cleanup := newTestClientSession(t, nil)
	defer cleanup()
	var order []string
	var legacyName string
	var v2Info ToolCallInfo

	tools, err := GetTools(ctx, &Config{
		Cli:        clientSession,
		ServerName: "docs",
		ToolNameMapper: func(ctx context.Context, info ToolNameMapperInput) (ToolNameMapperOutput, error) {
			return ToolNameMapperOutput{ExposedName: "mcp__docs__" + info.Tool.Name}, nil
		},
		ToolNameList: []string{"greet"},
		ToolCallResultHandler: func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
			order = append(order, "legacy")
			legacyName = name
			return result, nil
		},
		ToolCallResultHandlerV2: func(ctx context.Context, info ToolCallInfo, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
			order = append(order, "v2")
			v2Info = info
			return result, nil
		},
	})
	require.NoError(t, err)
	_, err = tools[0].(tool.InvokableTool).InvokableRun(ctx, "{\"name\": \"eino\"}")
	require.NoError(t, err)
	assert.Equal(t, []string{"legacy", "v2"}, order)
	assert.Equal(t, "mcp__docs__greet", legacyName)
	assert.Equal(t, "docs", v2Info.ServerName)
	assert.Equal(t, "greet", v2Info.RawToolName)
	assert.Equal(t, "mcp__docs__greet", v2Info.ExposedToolName)
}

func TestResultAndDescriptionPolicies(t *testing.T) {
	ctx := context.Background()
	clientSession, cleanup := newTestClientSession(t, func(server *mcp.Server) {
		mcp.AddTool(server, &mcp.Tool{Name: "long", Description: strings.Repeat("好", 8)}, func(ctx context.Context, req *mcp.CallToolRequest, args SayHiParams) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: strings.Repeat("x", 200)}},
			}, nil, nil
		})
	})
	defer cleanup()

	tools, err := GetTools(ctx, &Config{
		Cli:               clientSession,
		ToolNameList:      []string{"long"},
		DescriptionPolicy: &DescriptionPolicy{MaxChars: 5},
		ResultPolicy:      &ResultPolicy{MaxChars: 180, PreserveTailChars: 10},
	})
	require.NoError(t, err)
	info, err := tools[0].Info(ctx)
	require.NoError(t, err)
	assert.LessOrEqual(t, len([]rune(info.Desc)), 5)
	result, err := tools[0].(tool.InvokableTool).InvokableRun(ctx, "{\"name\": \"eino\"}")
	require.NoError(t, err)
	assert.Contains(t, result, "\"truncated\":true")
	assert.LessOrEqual(t, len([]rune(result)), 180)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &decoded))
}

func TestResultPolicyCanReturnServerErrorAsResult(t *testing.T) {
	ctx := context.Background()
	clientSession, cleanup := newTestClientSession(t, func(server *mcp.Server) {
		mcp.AddTool(server, &mcp.Tool{Name: "fail", Description: "fail"}, func(ctx context.Context, req *mcp.CallToolRequest, args SayHiParams) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "server error"}},
				IsError: true,
			}, nil, nil
		})
	})
	defer cleanup()

	tools, err := GetTools(ctx, &Config{Cli: clientSession, ToolNameList: []string{"fail"}})
	require.NoError(t, err)
	_, err = tools[0].(tool.InvokableTool).InvokableRun(ctx, "{\"name\": \"eino\"}")
	require.Error(t, err)
	assert.True(t, IsErrorKind(err, ErrorKindServerToolError))

	errorAsError := false
	tools, err = GetTools(ctx, &Config{
		Cli:          clientSession,
		ToolNameList: []string{"fail"},
		ResultPolicy: &ResultPolicy{ErrorAsError: &errorAsError},
	})
	require.NoError(t, err)
	result, err := tools[0].(tool.InvokableTool).InvokableRun(ctx, "{\"name\": \"eino\"}")
	require.NoError(t, err)
	assert.Contains(t, result, "server error")
}

func TestListToolsPagination(t *testing.T) {
	ctx := context.Background()
	clientSession, cleanup := newTestClientSession(t, nil, func(options *mcp.ServerOptions) {
		options.PageSize = 1
	})
	defer cleanup()

	tools, err := GetTools(ctx, &Config{Cli: clientSession})
	require.NoError(t, err)
	assert.Len(t, tools, 1)

	tools, err = GetTools(ctx, &Config{Cli: clientSession, ListToolsMode: ListToolsAllPages})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tools), 2)
}

func TestListToolsPaginationMaxPages(t *testing.T) {
	ctx := context.Background()
	clientSession, cleanup := newTestClientSession(t, nil, func(options *mcp.ServerOptions) {
		options.PageSize = 1
	})
	defer cleanup()

	_, err := GetTools(ctx, &Config{
		Cli:           clientSession,
		ListToolsMode: ListToolsAllPages,
		MaxToolPages:  1,
	})
	require.Error(t, err)
	assert.True(t, IsErrorKind(err, ErrorKindListTools))
	assert.Contains(t, err.Error(), "exceeded max tool pages")
}

func TestValidateConfigAndErrorHelpers(t *testing.T) {
	_, err := GetTools(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config is nil")

	assert.Error(t, validateConfig(&Config{MetadataMode: "unknown"}))
	assert.Error(t, validateConfig(&Config{ListToolsMode: "unknown"}))
	assert.Error(t, validateConfig(&Config{MaxToolPages: -1}))

	wrapped := &Error{Kind: ErrorKindCallTool, Err: assert.AnError}
	assert.Equal(t, assert.AnError.Error(), wrapped.Error())
	assert.ErrorIs(t, wrapped, assert.AnError)
	assert.Equal(t, "", (*Error)(nil).Error())
	assert.NoError(t, (*Error)(nil).Unwrap())
}

func TestGetToolsRejectsNilClient(t *testing.T) {
	_, err := GetTools(context.Background(), &Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")
}

func TestResultPolicyIncludesStructuredContentAndMeta(t *testing.T) {
	result := &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: "ok"}},
		StructuredContent: map[string]any{"answer": float64(42)},
		Meta:              mcp.Meta{"trace_id": "abc"},
	}

	marshaled, err := marshalToolResult(result, &ResultPolicy{
		IncludeStructuredContent: true,
		IncludeMeta:              true,
	})
	require.NoError(t, err)
	assert.Contains(t, marshaled, "\"structuredContent\":{\"answer\":42}")
	assert.Contains(t, marshaled, "\"_meta\":{\"trace_id\":\"abc\"}")
}

func TestAttack_ResultPolicyRejectsImpossibleMaxChars(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: strings.Repeat("x", 200)}},
	}

	_, err := marshalToolResult(result, &ResultPolicy{MaxChars: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds max chars")
}

func TestAttack_ResultHandlerNilResultDoesNotPanic(t *testing.T) {
	ctx := context.Background()
	clientSession, cleanup := newTestClientSession(t, nil)
	defer cleanup()

	tools, err := GetTools(ctx, &Config{
		Cli:          clientSession,
		ToolNameList: []string{"greet"},
		ToolCallResultHandler: func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
			return nil, nil
		},
	})
	require.NoError(t, err)

	var runErr error
	require.NotPanics(t, func() {
		_, runErr = tools[0].(tool.InvokableTool).InvokableRun(ctx, "{\"name\": \"eino\"}")
	})
	require.Error(t, runErr)
	assert.True(t, IsErrorKind(runErr, ErrorKindResultPolicy))
}

func newTestClientSession(t *testing.T, customize func(server *mcp.Server), options ...func(*mcp.ServerOptions)) (*mcp.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()
	serverOptions := &mcp.ServerOptions{}
	for _, option := range options {
		option(serverOptions)
	}
	server := mcp.NewServer(testImpl, serverOptions)
	client := mcp.NewClient(testImpl, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)
	mcp.AddTool(server, &mcp.Tool{Name: "hello", Description: "say hello"}, SayHello)
	if customize != nil {
		customize(server)
	}
	return clientSession, func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	}
}

func jsonRoundTrip(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var out any
	return json.Unmarshal(b, &out)
}

// stubSession is a fake ClientSession that returns canned errors, used to test
// error classification without a live transport.
type stubSession struct {
	tools      *mcp.ListToolsResult
	callErr    error
	callResult *mcp.CallToolResult
}

func (s *stubSession) ListTools(ctx context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	if s.tools != nil {
		return s.tools, nil
	}
	return &mcp.ListToolsResult{}, nil
}

func (s *stubSession) CallTool(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	return s.callResult, s.callErr
}

func newStubTool(t *testing.T, s *stubSession) tool.InvokableTool {
	t.Helper()
	return &toolHelper{cli: s, rawToolName: "greet", exposedToolName: "greet", info: &schema.ToolInfo{Name: "greet"}}
}

func TestInvokableRunClassifiesConnectionError(t *testing.T) {
	ctx := context.Background()
	// A connection-level failure (go-sdk terminal sentinel) must be tagged
	// ErrorKindConnection so callers can reconnect, not ErrorKindCallTool.
	connErr := fmt.Errorf("calling %q: %w", "tools/call", mcp.ErrConnectionClosed)
	_, err := newStubTool(t, &stubSession{callErr: connErr}).InvokableRun(ctx, "{}")
	require.Error(t, err)
	assert.True(t, IsErrorKind(err, ErrorKindConnection), "want ErrorKindConnection, got %v", err)
	assert.True(t, IsConnectionError(err))

	sessErr := fmt.Errorf("wrap: %w", mcp.ErrSessionMissing)
	_, err = newStubTool(t, &stubSession{callErr: sessErr}).InvokableRun(ctx, "{}")
	require.Error(t, err)
	assert.True(t, IsErrorKind(err, ErrorKindConnection))
}

func TestInvokableRunClassifiesProtocolErrorAsCallTool(t *testing.T) {
	ctx := context.Background()
	// A protocol-level rejection (unknown tool / invalid params) leaves the
	// session healthy: it must stay ErrorKindCallTool and NOT be treated as a
	// connection error, otherwise a model calling with bad args causes reconnect
	// churn.
	protoErr := errors.New("calling \"tools/call\": jsonrpc2: code -32602: invalid params")
	_, err := newStubTool(t, &stubSession{callErr: protoErr}).InvokableRun(ctx, "{}")
	require.Error(t, err)
	assert.True(t, IsErrorKind(err, ErrorKindCallTool))
	assert.False(t, IsErrorKind(err, ErrorKindConnection))
	assert.False(t, IsConnectionError(err))
}

func TestIsConnectionError(t *testing.T) {
	assert.False(t, IsConnectionError(nil))
	assert.False(t, IsConnectionError(errors.New("plain")))
	assert.True(t, IsConnectionError(fmt.Errorf("x: %w", mcp.ErrConnectionClosed)))
	assert.True(t, IsConnectionError(fmt.Errorf("x: %w", mcp.ErrSessionMissing)))
	assert.True(t, IsConnectionError(&Error{Kind: ErrorKindConnection}))
	assert.False(t, IsConnectionError(&Error{Kind: ErrorKindCallTool}))
}
