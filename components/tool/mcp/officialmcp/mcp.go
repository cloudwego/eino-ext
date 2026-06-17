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
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/eino-contrib/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const (
	ExtraMCPServerName      = "mcp.server_name"
	ExtraMCPRawToolName     = "mcp.raw_tool_name"
	ExtraMCPExposedToolName = "mcp.exposed_tool_name"
	ExtraMCPAnnotations     = "mcp.annotations"
	ExtraMCPRawTool         = "mcp.raw_tool"
)

type MetadataMode string

const (
	MetadataBasic MetadataMode = "basic"
	MetadataFull  MetadataMode = "full"
)

type ListToolsMode string

const (
	ListToolsSinglePage ListToolsMode = "single_page"
	ListToolsAllPages   ListToolsMode = "all_pages"
)

const defaultMaxToolPages = 20

type ToolNameMapper func(ctx context.Context, info ToolNameMapperInput) (ToolNameMapperOutput, error)

type ToolNameMapperInput struct {
	ServerName string
	Tool       mcp.Tool
}

type ToolNameMapperOutput struct {
	ExposedName string
	Extra       map[string]any
}

type ToolCallInfo struct {
	ServerName      string
	RawToolName     string
	ExposedToolName string
	Tool            mcp.Tool
}

type ResultPolicy struct {
	MaxChars                 int
	PreserveTailChars        int
	IncludeStructuredContent bool
	IncludeMeta              bool
	ErrorAsError             *bool
}

type DescriptionPolicy struct {
	MaxChars int
}

type ErrorKind string

const (
	ErrorKindListTools       ErrorKind = "list_tools"
	ErrorKindSchemaConvert   ErrorKind = "schema_convert"
	ErrorKindCallTool        ErrorKind = "call_tool"
	ErrorKindConnection      ErrorKind = "connection"
	ErrorKindServerToolError ErrorKind = "server_tool_error"
	ErrorKindResultPolicy    ErrorKind = "result_policy"
)

type Error struct {
	Kind            ErrorKind
	ServerName      string
	RawToolName     string
	ExposedToolName string
	Err             error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("official mcp error: %s", e.Kind)
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsErrorKind(err error, kind ErrorKind) bool {
	var e *Error
	return errors.As(err, &e) && e.Kind == kind
}

// IsConnectionError reports whether err stems from a connection-level failure
// (the session is dead and must be rebuilt), as opposed to a protocol-level
// rejection (unknown tool, invalid params) or an application-level tool error
// (result.IsError), on both of which the session remains usable.
//
// It matches the go-sdk's terminal sentinels as well as officialmcp's own
// ErrorKindConnection, so callers can use it on either a raw CallTool/ListTools
// error or an officialmcp *Error.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if IsErrorKind(err, ErrorKindConnection) {
		return true
	}
	return errors.Is(err, mcp.ErrConnectionClosed) || errors.Is(err, mcp.ErrSessionMissing)
}

// ClientSession is the anti-corruption boundary between officialmcp and the
// go-sdk session. It is the subset of *mcp.ClientSession that the tools require.
// *mcp.ClientSession satisfies it directly, so a raw session can still be passed
// as Cli. A reconnecting wrapper (see the session sub-package) can also implement
// it to rebuild the underlying session on connection-level failures, transparently
// to the tools.
type ClientSession interface {
	ListTools(ctx context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error)
	CallTool(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error)
	Ping(ctx context.Context, params *mcp.PingParams) error
	Close() error
}

type Config struct {
	// Cli is the MCP client session. *mcp.ClientSession satisfies this directly;
	// for transparent reconnection on connection-level failures, pass a
	// session.ReconnectingSession. ref: https://github.com/modelcontextprotocol/go-sdk?tab=readme-ov-file#tools
	// Notice: should Initialize with server before use
	Cli ClientSession

	ServerName string

	// ToolNameList specifies which tools to fetch from MCP server
	// If empty, all available tools will be fetched
	ToolNameList []string

	Cursor        string
	ListToolsMode ListToolsMode
	MaxToolPages  int

	ToolNameMapper ToolNameMapper
	MetadataMode   MetadataMode

	DescriptionPolicy *DescriptionPolicy
	ResultPolicy      *ResultPolicy

	// ToolCallResultHandler is a function that processes the result after a tool call completes
	// It can be used for custom processing of tool call results
	// If nil, no additional processing will be performed
	ToolCallResultHandler func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error)

	// ToolCallResultHandlerV2 receives stable raw/exposed MCP tool identity.
	// If both handlers are set, the legacy handler runs first, then V2.
	ToolCallResultHandlerV2 func(ctx context.Context, info ToolCallInfo, result *mcp.CallToolResult) (*mcp.CallToolResult, error)
}

func GetTools(ctx context.Context, conf *Config) ([]tool.BaseTool, error) {
	if conf == nil {
		return nil, errors.New("official mcp config is nil")
	}
	if conf.Cli == nil {
		return nil, errors.New("official mcp client is nil")
	}
	if err := validateConfig(conf); err != nil {
		return nil, err
	}

	tools, err := listTools(ctx, conf)
	if err != nil {
		return nil, err
	}

	nameSet := make(map[string]struct{})
	for _, name := range conf.ToolNameList {
		nameSet[name] = struct{}{}
	}

	exposedNameSet := make(map[string]struct{})
	ret := make([]tool.BaseTool, 0, len(tools))
	for _, t := range tools {
		if len(conf.ToolNameList) > 0 {
			if _, ok := nameSet[t.Name]; !ok {
				continue
			}
		}
		rawToolName := t.Name
		exposedToolName := rawToolName
		mapperExtra := map[string]any(nil)
		if conf.ToolNameMapper != nil {
			out, err := conf.ToolNameMapper(ctx, ToolNameMapperInput{
				ServerName: conf.ServerName,
				Tool:       *t,
			})
			if err != nil {
				return nil, fmt.Errorf("map official mcp tool name fail, tool name: %s: %w", rawToolName, err)
			}
			if out.ExposedName == "" {
				return nil, fmt.Errorf("map official mcp tool name fail, tool name: %s: exposed tool name is empty", rawToolName)
			}
			exposedToolName = out.ExposedName
			mapperExtra = out.Extra
		}
		if _, ok := exposedNameSet[exposedToolName]; ok {
			return nil, fmt.Errorf("duplicate official mcp exposed tool name: %s", exposedToolName)
		}
		exposedNameSet[exposedToolName] = struct{}{}

		marshaledInputSchema, err := sonic.Marshal(t.InputSchema)
		if err != nil {
			return nil, &Error{Kind: ErrorKindSchemaConvert, ServerName: conf.ServerName, RawToolName: rawToolName, ExposedToolName: exposedToolName, Err: fmt.Errorf("conv official mcp tool input schema fail(marshal): %w, tool name: %s", err, rawToolName)}
		}
		inputSchema := &jsonschema.Schema{}
		err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
		if err != nil {
			return nil, &Error{Kind: ErrorKindSchemaConvert, ServerName: conf.ServerName, RawToolName: rawToolName, ExposedToolName: exposedToolName, Err: fmt.Errorf("conv official mcp tool input schema fail(unmarshal): %w, tool name: %s", err, rawToolName)}
		}

		extra, err := buildToolInfoExtra(conf, t, rawToolName, exposedToolName, mapperExtra)
		if err != nil {
			return nil, err
		}

		ret = append(ret, &toolHelper{
			cli: conf.Cli,
			info: &schema.ToolInfo{
				Name:        exposedToolName,
				Desc:        applyDescriptionPolicy(t.Description, conf.DescriptionPolicy),
				Extra:       extra,
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(inputSchema),
			},
			serverName:              conf.ServerName,
			rawToolName:             rawToolName,
			exposedToolName:         exposedToolName,
			rawTool:                 *t,
			toolCallResultHandler:   conf.ToolCallResultHandler,
			toolCallResultHandlerV2: conf.ToolCallResultHandlerV2,
			resultPolicy:            conf.ResultPolicy,
		})
	}

	return ret, nil
}

type toolHelper struct {
	cli                     ClientSession
	info                    *schema.ToolInfo
	serverName              string
	rawToolName             string
	exposedToolName         string
	rawTool                 mcp.Tool
	toolCallResultHandler   func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error)
	toolCallResultHandlerV2 func(ctx context.Context, info ToolCallInfo, result *mcp.CallToolResult) (*mcp.CallToolResult, error)
	resultPolicy            *ResultPolicy
}

func (m *toolHelper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return m.info, nil
}

func (m *toolHelper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	result, err := m.cli.CallTool(ctx, &mcp.CallToolParams{
		Name:      m.rawToolName,
		Arguments: json.RawMessage(argumentsInJSON),
	})
	if err != nil {
		kind := ErrorKindCallTool
		if IsConnectionError(err) {
			kind = ErrorKindConnection
		}
		return "", &Error{Kind: kind, ServerName: m.serverName, RawToolName: m.rawToolName, ExposedToolName: m.exposedToolName, Err: fmt.Errorf("failed to call official mcp tool: %w", err)}
	}
	if result == nil {
		return "", &Error{Kind: ErrorKindResultPolicy, ServerName: m.serverName, RawToolName: m.rawToolName, ExposedToolName: m.exposedToolName, Err: errors.New("official mcp tool result is nil")}
	}

	if m.toolCallResultHandler != nil {
		result, err = m.toolCallResultHandler(ctx, m.exposedToolName, result)
		if err != nil {
			return "", fmt.Errorf("failed to execute official mcp tool call result handler: %w", err)
		}
		if result == nil {
			return "", &Error{Kind: ErrorKindResultPolicy, ServerName: m.serverName, RawToolName: m.rawToolName, ExposedToolName: m.exposedToolName, Err: errors.New("official mcp tool call result handler returned nil result")}
		}
	}
	if m.toolCallResultHandlerV2 != nil {
		result, err = m.toolCallResultHandlerV2(ctx, ToolCallInfo{
			ServerName:      m.serverName,
			RawToolName:     m.rawToolName,
			ExposedToolName: m.exposedToolName,
			Tool:            m.rawTool,
		}, result)
		if err != nil {
			return "", fmt.Errorf("failed to execute official mcp tool call result handler v2: %w", err)
		}
		if result == nil {
			return "", &Error{Kind: ErrorKindResultPolicy, ServerName: m.serverName, RawToolName: m.rawToolName, ExposedToolName: m.exposedToolName, Err: errors.New("official mcp tool call result handler v2 returned nil result")}
		}
	}

	marshaledResult, err := marshalToolResult(result, m.resultPolicy)
	if err != nil {
		return "", &Error{Kind: ErrorKindResultPolicy, ServerName: m.serverName, RawToolName: m.rawToolName, ExposedToolName: m.exposedToolName, Err: fmt.Errorf("failed to marshal official mcp tool result: %w", err)}
	}
	if result.IsError && shouldReturnError(m.resultPolicy) {
		return "", &Error{Kind: ErrorKindServerToolError, ServerName: m.serverName, RawToolName: m.rawToolName, ExposedToolName: m.exposedToolName, Err: fmt.Errorf("failed to call official mcp tool, mcp server return error: %s", marshaledResult)}
	}

	return marshaledResult, nil
}

func validateConfig(conf *Config) error {
	switch conf.MetadataMode {
	case "", MetadataBasic, MetadataFull:
	default:
		return fmt.Errorf("unknown official mcp metadata mode: %s", conf.MetadataMode)
	}
	switch conf.ListToolsMode {
	case "", ListToolsSinglePage, ListToolsAllPages:
	default:
		return fmt.Errorf("unknown official mcp list tools mode: %s", conf.ListToolsMode)
	}
	if conf.MaxToolPages < 0 {
		return fmt.Errorf("official mcp max tool pages must not be negative: %d", conf.MaxToolPages)
	}
	return nil
}

func listTools(ctx context.Context, conf *Config) ([]*mcp.Tool, error) {
	mode := conf.ListToolsMode
	if mode == "" {
		mode = ListToolsSinglePage
	}
	maxPages := conf.MaxToolPages
	if maxPages == 0 {
		maxPages = defaultMaxToolPages
	}
	cursor := conf.Cursor
	seenCursors := map[string]struct{}{}
	var tools []*mcp.Tool
	for page := 0; ; page++ {
		if page >= maxPages {
			return nil, &Error{Kind: ErrorKindListTools, ServerName: conf.ServerName, Err: fmt.Errorf("list official mcp tools fail: exceeded max tool pages: %d", maxPages)}
		}
		if cursor != "" {
			if _, ok := seenCursors[cursor]; ok {
				return nil, &Error{Kind: ErrorKindListTools, ServerName: conf.ServerName, Err: fmt.Errorf("list official mcp tools fail: repeated cursor: %s", cursor)}
			}
			seenCursors[cursor] = struct{}{}
		}
		listResults, err := conf.Cli.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			kind := ErrorKindListTools
			if IsConnectionError(err) {
				kind = ErrorKindConnection
			}
			return nil, &Error{Kind: kind, ServerName: conf.ServerName, Err: fmt.Errorf("list official mcp tools fail: %w", err)}
		}
		tools = append(tools, listResults.Tools...)
		if mode == ListToolsSinglePage || listResults.NextCursor == "" {
			return tools, nil
		}
		cursor = listResults.NextCursor
	}
}

func buildToolInfoExtra(conf *Config, t *mcp.Tool, rawToolName, exposedToolName string, mapperExtra map[string]any) (map[string]any, error) {
	extra := make(map[string]any, len(mapperExtra)+5)
	for k, v := range mapperExtra {
		if isReservedMCPExtraKey(k) {
			return nil, fmt.Errorf("official mcp tool mapper extra conflicts with reserved key: %s", k)
		}
		extra[k] = v
	}
	extra[ExtraMCPServerName] = conf.ServerName
	extra[ExtraMCPRawToolName] = rawToolName
	extra[ExtraMCPExposedToolName] = exposedToolName
	if t.Annotations != nil {
		extra[ExtraMCPAnnotations] = projectAnnotations(t.Annotations)
	}
	if conf.MetadataMode == MetadataFull {
		rawTool, err := toJSONCompatible(t)
		if err != nil {
			return nil, fmt.Errorf("conv official mcp raw tool metadata fail: %w, tool name: %s", err, rawToolName)
		}
		extra[ExtraMCPRawTool] = rawTool
	}
	return extra, nil
}

func isReservedMCPExtraKey(key string) bool {
	switch key {
	case ExtraMCPServerName, ExtraMCPRawToolName, ExtraMCPExposedToolName, ExtraMCPAnnotations, ExtraMCPRawTool:
		return true
	default:
		return false
	}
}

func projectAnnotations(a *mcp.ToolAnnotations) map[string]any {
	ret := map[string]any{}
	if a.DestructiveHint != nil {
		ret["destructiveHint"] = *a.DestructiveHint
	}
	if a.IdempotentHint {
		ret["idempotentHint"] = a.IdempotentHint
	}
	if a.OpenWorldHint != nil {
		ret["openWorldHint"] = *a.OpenWorldHint
	}
	if a.ReadOnlyHint {
		ret["readOnlyHint"] = a.ReadOnlyHint
	}
	if a.Title != "" {
		ret["title"] = a.Title
	}
	return ret
}

func toJSONCompatible(v any) (map[string]any, error) {
	b, err := sonic.Marshal(v)
	if err != nil {
		return nil, err
	}
	var ret map[string]any
	if err := sonic.Unmarshal(b, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func applyDescriptionPolicy(desc string, policy *DescriptionPolicy) string {
	if policy == nil || policy.MaxChars <= 0 {
		return desc
	}
	return truncateString(desc, policy.MaxChars, 0)
}

func marshalToolResult(result *mcp.CallToolResult, policy *ResultPolicy) (string, error) {
	if policy == nil {
		return sonic.MarshalString(result)
	}
	out := map[string]any{
		"content": result.Content,
	}
	if result.IsError {
		out["isError"] = result.IsError
	}
	if policy.IncludeStructuredContent && result.StructuredContent != nil {
		out["structuredContent"] = result.StructuredContent
	}
	if policy.IncludeMeta && len(result.Meta) > 0 {
		out["_meta"] = result.Meta
	}
	marshaled, err := sonic.MarshalString(out)
	if err != nil {
		return "", err
	}
	if policy.MaxChars > 0 && utf8.RuneCountInString(marshaled) > policy.MaxChars {
		return marshalTruncatedToolResult(marshaled, policy.MaxChars, policy.PreserveTailChars)
	}
	return marshaled, nil
}

func marshalTruncatedToolResult(original string, maxChars, preserveTailChars int) (string, error) {
	originalChars := utf8.RuneCountInString(original)
	payloadChars := maxChars
	for payloadChars >= 0 {
		payload := ""
		if payloadChars > 0 {
			payload = truncateString(original, payloadChars, preserveTailChars)
		}
		out := map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": payload,
				},
			},
			"_meta": map[string]any{
				"truncated":     true,
				"originalChars": originalChars,
				"returnedChars": maxChars,
			},
		}
		marshaled, err := sonic.MarshalString(out)
		if err != nil {
			return "", err
		}
		if maxChars <= 0 || utf8.RuneCountInString(marshaled) <= maxChars {
			return marshaled, nil
		}
		payloadChars -= utf8.RuneCountInString(marshaled) - maxChars
		if payloadChars < 0 {
			payloadChars = 0
		}
		if payload == "" {
			return "", fmt.Errorf("truncated official mcp tool result exceeds max chars: max=%d actual=%d", maxChars, utf8.RuneCountInString(marshaled))
		}
	}
	return "", fmt.Errorf("truncated official mcp tool result exceeds max chars: max=%d", maxChars)
}

func shouldReturnError(policy *ResultPolicy) bool {
	return policy == nil || policy.ErrorAsError == nil || *policy.ErrorAsError
}

func truncateString(s string, maxChars, preserveTailChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	marker := fmt.Sprintf("...[truncated original_chars=%d returned_chars=%d]...", utf8.RuneCountInString(s), maxChars)
	markerChars := utf8.RuneCountInString(marker)
	if markerChars >= maxChars {
		return firstRunes(marker, maxChars)
	}
	tailChars := preserveTailChars
	if tailChars < 0 {
		tailChars = 0
	}
	remaining := maxChars - markerChars
	if tailChars > remaining {
		tailChars = remaining
	}
	headChars := remaining - tailChars
	return firstRunes(s, headChars) + marker + lastRunes(s, tailChars)
}

func firstRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if n == 0 {
			break
		}
		b.WriteRune(r)
		n--
	}
	return b.String()
}

func lastRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[len(rs)-n:])
}
