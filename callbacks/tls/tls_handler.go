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

package tls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	sem_ai "github.com/cloudwego/eino-ext/callbacks/tls/semconv"
	"github.com/cloudwego/eino-ext/libs/acl/opentelemetry"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelsemconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

const scopeName = "github.com/cloudwego/eino-ext/callbacks/tls"

type TLSConfig struct {
	// TLSEndpoint is the TLS OTLP endpoint.
	// Example: "tls-cn-beijing.volces.com:4317"
	TLSEndpoint string

	// AppName is the application name shown in TLS.
	AppName string

	// TLSOTLPHeader contains OTLP export headers.
	TLSOTLPHeader map[string]string

	// TLSOTLPHeadersStr is the comma-separated OTLP headers string.
	TLSOTLPHeadersStr string

	// Release is the version or release identifier.
	Release string

	// TLSExporterEnabled emits the span names and attributes expected by the
	// TLS LogApp dashboard schema. It is opt-in to preserve the
	// existing generic TLS callback behavior for current users.
	TLSExporterEnabled bool

	// TLSLog* configures the TLS Producer SendLogs transport used by the TLS
	// integration. When TLSLogTopicID is empty, the existing OTLP transport is
	// used even if TLSExporterEnabled is enabled.
	TLSLogEndpoint        string
	TLSLogRegion          string
	TLSLogTopicID         string
	TLSLogAPIKey          string
	TLSLogAccessKeyID     string
	TLSLogAccessKeySecret string
}

func NewTLSCallbackHandler(config ...*TLSConfig) (handler callbacks.Handler, shutdown func(ctx context.Context) error, err error) {
	cfg, err := resolveTLSConfig(config...)
	if err != nil {
		return nil, nil, err
	}

	return buildTLSCallbackHandler(cfg, newOptions())
}

func NewTLSCallbackHandlerWithOptions(cfg *TLSConfig, opts ...Option) (handler callbacks.Handler, shutdown func(ctx context.Context) error, err error) {
	if cfg == nil {
		cfg, err = LoadTLSConfigFromEnv()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load TLS config from environment: %w", err)
		}
	} else if err = ValidateTLSConfig(cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to validate TLS config: %w", err)
	}

	return buildTLSCallbackHandler(cfg, newOptions(opts...))
}

// NewTLSCallbackHandlerFromEnv keeps the original API shape for callers already passing a prepared config.
func NewTLSCallbackHandlerFromEnv(cfg *TLSConfig) (handler callbacks.Handler, shutdown func(ctx context.Context) error, err error) {
	return NewTLSCallbackHandlerWithOptions(cfg)
}

// TLSCallbackHandler implements callbacks.Handler.
type TLSCallbackHandler struct {
	OtelProvider       *opentelemetry.OtelProvider
	AppName            string
	Release            string
	Tracer             trace.Tracer
	dataParser         CallbackDataParser
	TLSExporterEnabled bool
}

// TLSHandler extends TLSCallbackHandler with manual span controls.
type TLSHandler struct {
	*TLSCallbackHandler
}

func NewTLSHandler(config ...*TLSConfig) (*TLSHandler, func(ctx context.Context) error, error) {
	cfg, err := resolveTLSConfig(config...)
	if err != nil {
		return nil, nil, err
	}

	return NewTLSHandlerWithOptions(cfg)
}

func NewTLSHandlerWithOptions(cfg *TLSConfig, opts ...Option) (*TLSHandler, func(ctx context.Context) error, error) {
	handler, shutdown, err := NewTLSCallbackHandlerWithOptions(cfg, opts...)
	if err != nil {
		return nil, shutdown, err
	}

	tlsHandler, ok := handler.(*TLSCallbackHandler)
	if !ok {
		return nil, shutdown, errors.New("failed to cast handler to *TLSCallbackHandler")
	}

	return &TLSHandler{TLSCallbackHandler: tlsHandler}, shutdown, nil
}

// NewTLSHandlerFromEnv keeps the original API shape for callers already passing a prepared config.
func NewTLSHandlerFromEnv(cfg *TLSConfig) (*TLSHandler, func(ctx context.Context) error, error) {
	return NewTLSHandlerWithOptions(cfg)
}

type RequestInfo struct {
	mu    sync.RWMutex
	Model string
}

func (r *RequestInfo) setModel(model string) {
	if r == nil || model == "" {
		return
	}
	r.mu.Lock()
	r.Model = model
	r.mu.Unlock()
}

func (r *RequestInfo) model() string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Model
}

type TLSStateKey struct{}

type tlsRootStateKey struct{}

type TLSState struct {
	StartTime   time.Time
	Span        trace.Span
	RequestInfo *RequestInfo
	IsRootNode  bool
	root        *tlsRootState
	// syntheticRoot is populated when a direct Eino model/tool callback has no
	// enclosing graph callback. TLS Lens expects an agent.turn root in that
	// case, so the callback creates one around the component span.
	syntheticRoot trace.Span
}

// tlsRootState collects turn-level fields that LLM Lens dashboard cards read
// from the agent.turn span, while the individual model spans retain their
// own request-level values.
type tlsRootState struct {
	span trace.Span
	mu   sync.Mutex

	// streamMu protects streamOutputPending and streamOutputDone.  Stream
	// callbacks parse their output in goroutines, while an enclosing graph can
	// finish before a child model's final chunk has been aggregated.  Keep the
	// root span open until all registered child stream callbacks have completed.
	streamMu            sync.Mutex
	streamOutputPending int
	streamOutputDone    chan struct{}

	modelRequests       int
	inputTokens         int
	outputTokens        int
	totalTokens         int
	cacheRead           int
	reasoning           int
	lastModelSpan       trace.SpanContext
	modelSpanByToolCall map[string]trace.SpanContext
	hasModelInput       bool
}

// beginStreamOutput registers an asynchronous stream-output parser and
// returns its idempotent completion callback.
func (r *tlsRootState) beginStreamOutput() func() {
	if r == nil {
		return func() {}
	}

	r.streamMu.Lock()
	if r.streamOutputPending == 0 {
		r.streamOutputDone = make(chan struct{})
	}
	r.streamOutputPending++
	r.streamMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			r.streamMu.Lock()
			defer r.streamMu.Unlock()
			if r.streamOutputPending == 0 {
				return
			}
			r.streamOutputPending--
			if r.streamOutputPending == 0 && r.streamOutputDone != nil {
				close(r.streamOutputDone)
			}
		})
	}
}

func (r *tlsRootState) waitForStreamOutputs(ctx context.Context, timeout time.Duration) {
	if r == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.streamMu.Lock()
	pending, done := r.streamOutputPending, r.streamOutputDone
	r.streamMu.Unlock()
	if pending == 0 || done == nil {
		return
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-ctx.Done():
		log.Printf("stream output did not finish before callback context was canceled")
	case <-timer.C:
		log.Printf("stream output did not finish within %s; ending root span without its remaining attributes", timeout)
	}
}

// tlsLensMessage is the Codex/Aiden-compatible message envelope consumed by
// the TLS session view. The generic Eino callback model uses a top-level
// Content field; TLS Lens expects text to be contained in parts.
type tlsLensMessage struct {
	Role       string               `json:"role"`
	Parts      []tlsLensMessagePart `json:"parts,omitempty"`
	Name       string               `json:"name,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

// tlsLensMessagesPayload is the common Input/Output value shape used by the
// TLS trace-detail renderer.  Keep the complete, structured transcript in a
// `messages` field for both streaming and non-streaming model callbacks.  In
// particular, do not expose Eino's callback transport wrappers (`stream`) or
// the OpenAI-style response wrapper (`choices`) as the primary UI payload.
type tlsLensMessagesPayload struct {
	Messages []tlsLensMessage `json:"messages,omitempty"`
}

type tlsLensMessagePart struct {
	Type      string `json:"type"`
	Content   any    `json:"content,omitempty"`
	ImageURL  any    `json:"image_url,omitempty"`
	AudioURL  any    `json:"audio_url,omitempty"`
	VideoURL  any    `json:"video_url,omitempty"`
	FileURL   any    `json:"file_url,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments any    `json:"arguments,omitempty"`
	Result    any    `json:"result,omitempty"`
}

func (r *tlsRootState) addModelUsage(tags map[string]any) {
	if r == nil || r.span == nil {
		return
	}
	input, hasInput := intTag(tags, sem_ai.GEN_AI_USAGE_INPUT_TOKENS)
	output, hasOutput := intTag(tags, sem_ai.GEN_AI_USAGE_OUTPUT_TOKENS)
	total, hasTotal := intTag(tags, sem_ai.GEN_AI_USAGE_TOTAL_TOKENS)
	cacheRead, hasCacheRead := intTag(tags, sem_ai.GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS_V2)
	reasoning, hasReasoning := intTag(tags, sem_ai.GEN_AI_USAGE_REASONING_OUTPUT_TOKENS)
	if !hasInput && !hasOutput && !hasTotal && !hasCacheRead && !hasReasoning {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.modelRequests++
	r.inputTokens += input
	r.outputTokens += output
	r.totalTokens += total
	r.cacheRead += cacheRead
	r.reasoning += reasoning
	r.span.SetAttributes(
		attribute.Int(sem_ai.GEN_AI_USAGE_INPUT_TOKENS, r.inputTokens),
		attribute.Int(sem_ai.GEN_AI_USAGE_OUTPUT_TOKENS, r.outputTokens),
		attribute.Int(sem_ai.GEN_AI_USAGE_TOTAL_TOKENS, r.totalTokens),
		attribute.Int(sem_ai.GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS_V2, r.cacheRead),
		attribute.Int(sem_ai.GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS, r.cacheRead),
		attribute.Int(sem_ai.GEN_AI_USAGE_CACHED_TOKENS, r.cacheRead),
		attribute.Int(sem_ai.GEN_AI_USAGE_CACHE_CREATE_INPUT_TOKENS, 0),
		attribute.Int(sem_ai.GEN_AI_USAGE_CACHE_CREATION_INPUT_TOKENS, 0),
		attribute.Int(sem_ai.GEN_AI_USAGE_REASONING_OUTPUT_TOKENS, r.reasoning),
	)
	model, _ := tags[sem_ai.GEN_AI_REQUEST_MODEL].(string)
	if model == "" {
		model, _ = tags[sem_ai.GEN_AI_RESPONSE_MODEL].(string)
	}
	if model != "" {
		r.span.SetAttributes(attribute.String(sem_ai.GEN_AI_REQUEST_MODEL, model))
	}
}

// copyModelPresentation copies the trace-level fields the TraceTableV2 and
// trace-detail views read from the agent.turn span. The first model input is
// the trace input, while the latest model output is the trace output.
func (r *tlsRootState) copyModelPresentation(tags map[string]any) {
	if r == nil || r.span == nil || len(tags) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rootTags := make(map[string]any, 7)
	if !r.hasModelInput {
		for _, key := range []string{
			sem_ai.GEN_AI_INPUT,
			sem_ai.GEN_AI_INPUT_MESSAGES,
			sem_ai.GEN_AI_REQUEST_MODEL,
			sem_ai.GEN_AI_REQUEST_TOOL_DEFINITIONS,
		} {
			if value, ok := tags[key]; ok {
				rootTags[key] = value
			}
		}
		if _, ok := rootTags[sem_ai.GEN_AI_INPUT]; ok {
			r.hasModelInput = true
		}
		// The full message array remains on gen_ai.input.messages for trace
		// drill-down.  For the session list, input.value should be readable at
		// a glance, so keep only the most recent user-message content there.
		if content := latestUserMessageContent(tags); content != "" {
			rootTags[sem_ai.GEN_AI_INPUT] = content
		}
	}
	for _, key := range []string{
		sem_ai.GEN_AI_OUTPUT,
		sem_ai.GEN_AI_OUTPUT_MESSAGES,
		sem_ai.GEN_AI_RESPONSE_MODEL,
		sem_ai.GEN_AI_RESPONSE_FINISH_REASON,
	} {
		if value, ok := tags[key]; ok {
			rootTags[key] = value
		}
	}
	// Keep the full provider response on the llm.request span, but make the
	// root session row readable just like the Codex and Claude integrations.
	if content := latestLensMessageContent(tags[sem_ai.GEN_AI_OUTPUT_MESSAGES], "assistant"); content != "" {
		rootTags[sem_ai.GEN_AI_OUTPUT] = content
	}
	setSpanAttributesFromTags(r.span, rootTags)
}

func latestUserMessageContent(tags map[string]any) string {
	if content := latestLensMessageContent(tags[sem_ai.GEN_AI_INPUT_MESSAGES], "user"); content != "" {
		return content
	}
	const promptPrefix = sem_ai.GEN_AI_PROMPT + "."
	const roleSuffix = ".role"
	latestIndex := -1
	latestContent := ""
	for key, value := range tags {
		if !strings.HasPrefix(key, promptPrefix) || !strings.HasSuffix(key, roleSuffix) {
			continue
		}
		messageRole, ok := value.(string)
		if !ok || !strings.EqualFold(messageRole, "user") {
			continue
		}
		indexText := strings.TrimSuffix(strings.TrimPrefix(key, promptPrefix), roleSuffix)
		index, err := strconv.Atoi(indexText)
		if err != nil || index <= latestIndex {
			continue
		}
		content, _ := tags[promptPrefix+indexText+".content"].(string)
		if content == "" {
			continue
		}
		latestIndex = index
		latestContent = content
	}
	return latestContent
}

func latestLensMessageContent(raw any, role string) string {
	if raw == nil {
		return ""
	}
	if messages, ok := raw.([]*sem_ai.ModelMessage); ok {
		return latestLensMessageContent(toTLSLensMessages(messages), role)
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	var messages []tlsLensMessage
	if err := json.Unmarshal(encoded, &messages); err != nil {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if !strings.EqualFold(messages[i].Role, role) {
			continue
		}
		if summary := lensMessageSummary(messages[i]); summary != "" {
			return summary
		}
	}
	return ""
}

func lensMessageSummary(message tlsLensMessage) string {
	parts := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch part.Type {
		case "text", "reasoning":
			if content, ok := part.Content.(string); ok && content != "" {
				parts = append(parts, content)
			}
		case "image_url", "audio_url", "video_url", "file_url":
			parts = append(parts, mediaPartSummary(part))
		case "tool_call":
			if part.Name != "" {
				parts = append(parts, "[tool call: "+part.Name+"]")
			}
		case "tool_call_response":
			if content, ok := part.Result.(string); ok && content != "" {
				parts = append(parts, content)
			} else {
				parts = append(parts, "[tool result]")
			}
		}
	}
	return strings.Join(parts, " ")
}

func mediaPartSummary(part tlsLensMessagePart) string {
	label := strings.TrimSuffix(part.Type, "_url")
	media := part.ImageURL
	if media == nil {
		media = part.AudioURL
	}
	if media == nil {
		media = part.VideoURL
	}
	if media == nil {
		media = part.FileURL
	}
	encoded, err := json.Marshal(media)
	if err != nil {
		return "[" + label + "]"
	}
	var payload struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if json.Unmarshal(encoded, &payload) != nil {
		return "[" + label + "]"
	}
	if payload.Name != "" {
		return "[" + label + ": " + payload.Name + "]"
	}
	if payload.URL != "" {
		return "[" + label + ": " + payload.URL + "]"
	}
	return "[" + label + "]"
}

func normalizeTLSLensMessages(tags map[string]any) {
	if len(tags) == 0 {
		return
	}
	for _, entry := range []struct {
		messagesKey string
		valueKey    string
	}{
		{messagesKey: sem_ai.GEN_AI_INPUT_MESSAGES, valueKey: sem_ai.GEN_AI_INPUT},
		{messagesKey: sem_ai.GEN_AI_OUTPUT_MESSAGES, valueKey: sem_ai.GEN_AI_OUTPUT},
	} {
		key := entry.messagesKey
		raw, ok := tags[key]
		if !ok {
			continue
		}
		// Parser implementations may use their own message slice type. Round
		// through the attribute's JSON contract instead of requiring an exact Go
		// type, then emit the one TLS Lens understands.
		var encoded []byte
		if serialized, ok := raw.(string); ok {
			encoded = []byte(serialized)
		} else {
			var err error
			encoded, err = json.Marshal(raw)
			if err != nil {
				continue
			}
		}
		var messages []*sem_ai.ModelMessage
		if err := json.Unmarshal(encoded, &messages); err != nil {
			continue
		}
		lensMessages := toTLSLensMessages(messages)
		tags[key] = lensMessages
		// input.value/output.value drives the trace detail's Input/Output tabs.
		// Use the same Lens-compatible transcript as gen_ai.*.messages so both
		// streaming and non-streaming spans have one readable representation.
		tags[entry.valueKey] = tlsLensMessagesPayload{Messages: lensMessages}
	}
}

func toTLSLensMessages(messages []*sem_ai.ModelMessage) []tlsLensMessage {
	converted := make([]tlsLensMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		result := tlsLensMessage{
			Role:       message.Role,
			Name:       message.Name,
			ToolCallID: message.ToolCallID,
		}
		if message.Role == "tool" {
			result.Parts = append(result.Parts, tlsLensMessagePart{
				Type:   "tool_call_response",
				ID:     message.ToolCallID,
				Result: message.Content,
			})
		} else if message.Content != "" && !hasTLSLensPartType(message.Parts, sem_ai.ModelMessagePartTypeText) {
			result.Parts = append(result.Parts, tlsLensMessagePart{Type: "text", Content: message.Content})
		}
		if message.ReasoningContent != "" && !hasTLSLensPartType(message.Parts, sem_ai.ModelMessagePartTypeReasoning) {
			result.Parts = append(result.Parts, tlsLensMessagePart{Type: "reasoning", Content: message.ReasoningContent})
		}
		for _, part := range message.Parts {
			if lensPart := toTLSLensMessagePart(part); lensPart != nil {
				result.Parts = appendTLSLensMessagePart(result.Parts, *lensPart)
			}
		}
		for _, toolCall := range message.ToolCalls {
			if toolCall == nil || toolCall.Function == nil {
				continue
			}
			result.Parts = append(result.Parts, tlsLensMessagePart{
				Type:      "tool_call",
				ID:        toolCall.ID,
				Name:      toolCall.Function.Name,
				Arguments: decodeJSONValue(toolCall.Function.Arguments),
			})
		}
		converted = append(converted, result)
	}
	return converted
}

func hasTLSLensPartType(parts []*sem_ai.ModelMessagePart, partType sem_ai.ModelMessagePartType) bool {
	for _, part := range parts {
		if part != nil && part.Type == partType {
			return true
		}
	}
	return false
}

// appendTLSLensMessagePart folds adjacent streamed text/reasoning chunks into
// one part. This keeps the trace detail compact: a token stream is represented
// as one content value rather than an array entry per token.
func appendTLSLensMessagePart(parts []tlsLensMessagePart, next tlsLensMessagePart) []tlsLensMessagePart {
	if len(parts) == 0 || (next.Type != "text" && next.Type != "reasoning") {
		return append(parts, next)
	}
	last := &parts[len(parts)-1]
	if last.Type != next.Type {
		return append(parts, next)
	}
	lastContent, lastOK := last.Content.(string)
	nextContent, nextOK := next.Content.(string)
	if !lastOK || !nextOK {
		return append(parts, next)
	}
	last.Content = lastContent + nextContent
	return parts
}

func toTLSLensMessagePart(part *sem_ai.ModelMessagePart) *tlsLensMessagePart {
	if part == nil {
		return nil
	}
	result := &tlsLensMessagePart{Type: string(part.Type)}
	switch part.Type {
	case sem_ai.ModelMessagePartTypeText, sem_ai.ModelMessagePartTypeReasoning:
		result.Content = part.Text
	case sem_ai.ModelMessagePartTypeImage:
		result.ImageURL = part.ImageURL
	case sem_ai.ModelMessagePartTypeAudio:
		result.AudioURL = part.AudioURL
	case sem_ai.ModelMessagePartTypeVideo:
		result.VideoURL = part.VideoURL
	case sem_ai.ModelMessagePartTypeFile:
		result.FileURL = part.FileURL
	default:
		return nil
	}
	return result
}

func decodeJSONValue(raw string) any {
	if raw == "" {
		return ""
	}
	var value any
	if json.Unmarshal([]byte(raw), &value) == nil {
		return value
	}
	return raw
}

func (r *tlsRootState) rememberModelSpan(span trace.Span, toolCallIDs ...string) {
	if r == nil || span == nil {
		return
	}
	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return
	}
	r.mu.Lock()
	r.lastModelSpan = spanContext
	if len(toolCallIDs) > 0 {
		if r.modelSpanByToolCall == nil {
			r.modelSpanByToolCall = make(map[string]trace.SpanContext, len(toolCallIDs))
		}
		for _, toolCallID := range toolCallIDs {
			if toolCallID != "" {
				r.modelSpanByToolCall[toolCallID] = spanContext
			}
		}
	}
	r.mu.Unlock()
}

func (r *tlsRootState) modelSpanForToolCall(toolCallID string) trace.SpanContext {
	if r == nil {
		return trace.SpanContext{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if toolCallID != "" {
		if spanContext, ok := r.modelSpanByToolCall[toolCallID]; ok && spanContext.IsValid() {
			return spanContext
		}
	}
	return r.lastModelSpan
}

func modelToolCallIDs(message *schema.Message) []string {
	if message == nil || len(message.ToolCalls) == 0 {
		return nil
	}
	ids := make([]string, 0, len(message.ToolCalls))
	for _, toolCall := range message.ToolCalls {
		if toolCall.ID != "" {
			ids = append(ids, toolCall.ID)
		}
	}
	return ids
}

func modelToolCallIDsFromTags(tags map[string]any) []string {
	raw, ok := tags[sem_ai.GEN_AI_OUTPUT_MESSAGES]
	if !ok || raw == nil {
		return nil
	}
	var encoded []byte
	if serialized, ok := raw.(string); ok {
		encoded = []byte(serialized)
	} else {
		var err error
		encoded, err = json.Marshal(raw)
		if err != nil {
			return nil
		}
	}

	var messages []struct {
		ToolCalls []struct {
			ID string `json:"id"`
		} `json:"tool_calls"`
		Parts []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"parts"`
	}
	if err := json.Unmarshal(encoded, &messages); err != nil {
		return nil
	}
	ids := make([]string, 0)
	seen := make(map[string]struct{})
	for _, message := range messages {
		for _, toolCall := range message.ToolCalls {
			if toolCall.ID != "" {
				seen[toolCall.ID] = struct{}{}
			}
		}
		for _, part := range message.Parts {
			if part.Type == "tool_call" && part.ID != "" {
				seen[part.ID] = struct{}{}
			}
		}
	}
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func intTag(tags map[string]any, key string) (int, bool) {
	value, ok := tags[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func stringTag(tags map[string]any, key string) string {
	if tags == nil {
		return ""
	}
	value, _ := tags[key].(string)
	return value
}

// addMissingTLSModelAttributes completes the dashboard contract when an Eino
// model callback does not repeat its Config in CallbackOutput. The input
// callback already contains the selected model, while request duration is
// known by the handler for both streaming and non-streaming calls.
func addMissingTLSModelAttributes(tags map[string]any, state *TLSState) {
	if tags == nil || state == nil {
		return
	}
	model := state.RequestInfo.model()
	if stringTag(tags, sem_ai.GEN_AI_REQUEST_MODEL) == "" && model != "" {
		tags[sem_ai.GEN_AI_REQUEST_MODEL] = model
	}
	if stringTag(tags, sem_ai.GEN_AI_RESPONSE_MODEL) == "" {
		if model != "" {
			tags[sem_ai.GEN_AI_RESPONSE_MODEL] = model
		}
	}
	if _, exists := tags[sem_ai.GEN_AI_REQUEST_DURATION_MS]; !exists && !state.StartTime.IsZero() {
		tags[sem_ai.GEN_AI_REQUEST_DURATION_MS] = time.Since(state.StartTime).Milliseconds()
	}
}

type TLSStreamInputAsyncKey struct{}

type TLSStreamInputAsyncVal chan struct{}

func buildTLSCallbackHandler(cfg *TLSConfig, opts *options) (callbacks.Handler, func(ctx context.Context) error, error) {
	if cfg.usesTLSLogTransport() {
		p, err := newTLSLogProvider(cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("init TLS log provider failed: %w", err)
		}
		parser := newDefaultDataParserWithConcatFuncs(opts.concatFuncs, opts.enableAggrOutput)
		if opts.parser != nil {
			parser = opts.parser
		}
		return &TLSCallbackHandler{
			OtelProvider:       p,
			AppName:            cfg.AppName,
			Release:            cfg.Release,
			Tracer:             p.TracerProvider.Tracer(scopeName),
			dataParser:         parser,
			TLSExporterEnabled: true,
		}, p.Shutdown, nil
	}

	otelOpts := []opentelemetry.Option{
		opentelemetry.WithServiceName(cfg.AppName),
		opentelemetry.WithExportEndpoint(cfg.TLSEndpoint),
		opentelemetry.WithHeaders(cfg.TLSOTLPHeader),
		opentelemetry.WithResourceAttribute(attribute.String("tls.business_type", "gen_ai")),
		opentelemetry.WithEnableTracing(true),
		opentelemetry.WithEnableMetrics(false),
	}
	if cfg.Release != "" {
		otelOpts = append(otelOpts, opentelemetry.WithResourceAttribute(otelsemconv.ServiceVersionKey.String(cfg.Release)))
	}

	p, err := opentelemetry.NewOpenTelemetryProvider(otelOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("init opentelemetry provider failed: %w", err)
	}
	if p == nil {
		return nil, nil, errors.New("init opentelemetry provider failed")
	}
	if p.TracerProvider == nil {
		return nil, p.Shutdown, errors.New("tracer provider is nil")
	}

	parser := newDefaultDataParserWithConcatFuncs(opts.concatFuncs, opts.enableAggrOutput)
	if opts.parser != nil {
		parser = opts.parser
	}

	return &TLSCallbackHandler{
		OtelProvider:       p,
		AppName:            cfg.AppName,
		Release:            cfg.Release,
		Tracer:             p.TracerProvider.Tracer(scopeName),
		dataParser:         parser,
		TLSExporterEnabled: cfg.TLSExporterEnabled,
	}, p.Shutdown, nil
}

func resolveTLSConfig(config ...*TLSConfig) (*TLSConfig, error) {
	if len(config) > 0 && config[0] != nil {
		if err := ValidateTLSConfig(config[0]); err != nil {
			return nil, fmt.Errorf("failed to validate TLS config: %w", err)
		}
		return config[0], nil
	}

	cfg, err := LoadTLSConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS config from environment: %w", err)
	}

	return cfg, nil
}

func setCommonSpanAttributes(ctx context.Context, span trace.Span, info *callbacks.RunInfo, tlsExporterEnabled bool) bool {
	isRootNode := ctx.Value(tlsRootStateKey{}) == nil
	spanKind := _eino_run_component_to_span_kind(info.Component)
	if isRootNode && spanKind == sem_ai.GEN_AI_SPAN_KIND_TASK {
		span.SetAttributes(attribute.String(sem_ai.GEN_AI_SPAN_KIND, sem_ai.GEN_AI_SPAN_KIND_CHAIN))
	} else {
		span.SetAttributes(attribute.String(sem_ai.GEN_AI_SPAN_KIND, spanKind))
	}

	span.SetAttributes(
		attribute.String(sem_ai.GEN_AI_SYSTEM, "eino"),
		attribute.String(sem_ai.GEN_AI_FRAMEWORK, "eino"),
		attribute.String(sem_ai.TASK_NAME, info.Name),
		attribute.String("runinfo.name", info.Name),
		attribute.String("runinfo.type", info.Type),
		attribute.String("runinfo.component", string(info.Component)),
	)
	if tlsExporterEnabled {
		span.SetAttributes(
			attribute.String(sem_ai.TLS_APP_TYPE, "eino"),
			attribute.String(sem_ai.GEN_AI_PROVIDER_NAME, "eino"),
			attribute.String(sem_ai.GEN_AI_OPERATION_NAME, tlsOperationName(info, isRootNode)),
		)
	}
	setSessionAttributes(ctx, span)

	return isRootNode
}

func setSessionAttributes(ctx context.Context, span trace.Span) {
	session, ok := ctx.Value(tlsSessionOptionKey{}).(*sessionOptions)
	if !ok || session == nil {
		return
	}

	if session.SessionID != "" {
		span.SetAttributes(attribute.String(sem_ai.GEN_AI_SESSION_ID, session.SessionID))
		span.SetAttributes(attribute.String(sem_ai.SESSION_ID, session.SessionID))
		span.SetAttributes(attribute.String("gen_ai.conversation.id", session.SessionID))
		span.SetAttributes(attribute.String("session.name", session.SessionID))
	}
	if session.UserID != "" {
		span.SetAttributes(attribute.String(sem_ai.GEN_AI_USER_ID, session.UserID))
		span.SetAttributes(attribute.String(sem_ai.USER_ID, session.UserID))
	}
}

func (h *TLSCallbackHandler) setTLSApplicationAttributes(span trace.Span, info *callbacks.RunInfo) {
	if h == nil || !h.TLSExporterEnabled || span == nil {
		return
	}
	attrs := make([]attribute.KeyValue, 0, 3)
	if h.AppName != "" {
		attrs = append(attrs, attribute.String("app.name", h.AppName))
	}
	if info != nil && info.Name != "" {
		attrs = append(attrs, attribute.String("agent.name", info.Name))
	}
	if h.Release != "" {
		attrs = append(attrs, attribute.String("tls.plugin.version", h.Release))
	}
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
}

const (
	streamInputWaitTimeout  = 5 * time.Second
	streamOutputWaitTimeout = 5 * time.Second
)

func waitStreamInput(ctx context.Context, timeout time.Duration) {
	if ctx == nil {
		ctx = context.Background()
	}
	if stopCh, ok := ctx.Value(TLSStreamInputAsyncKey{}).(TLSStreamInputAsyncVal); ok {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-stopCh:
		case <-ctx.Done():
			log.Printf("stream input did not finish before callback context was canceled")
		case <-timer.C:
			log.Printf("stream input did not finish within %s; ending span without its remaining input attributes", timeout)
		}
	}
}

func endSpan(ctx context.Context, span trace.Span, status codes.Code, description string) {
	waitStreamInput(ctx, streamInputWaitTimeout)
	span.SetStatus(status, description)
	span.End(trace.WithTimestamp(time.Now()))
}

func endTLSState(ctx context.Context, state *TLSState, status codes.Code, description string) {
	if state == nil || state.Span == nil {
		return
	}
	if state.IsRootNode {
		state.root.waitForStreamOutputs(ctx, streamOutputWaitTimeout)
	}
	endSpan(ctx, state.Span, status, description)
	endSyntheticRoot(state.syntheticRoot, status, description)
}

func (h *TLSCallbackHandler) startSyntheticRootIfNeeded(ctx context.Context, info *callbacks.RunInfo, startTime time.Time) (context.Context, *tlsRootState, trace.Span) {
	if h == nil || !h.TLSExporterEnabled || info == nil || ctx.Value(tlsRootStateKey{}) != nil || ctx.Value(TLSStateKey{}) != nil {
		return ctx, nil, nil
	}
	if info.Component != components.ComponentOfChatModel && info.Component != components.ComponentOfTool {
		return ctx, nil, nil
	}

	rootCtx, rootSpan := h.Tracer.Start(ctx, tlsRootSpanName,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithTimestamp(startTime),
	)
	setCommonSpanAttributes(rootCtx, rootSpan, info, true)
	rootSpan.SetAttributes(
		attribute.String(sem_ai.GEN_AI_SPAN_KIND, sem_ai.GEN_AI_SPAN_KIND_CHAIN),
		attribute.String(sem_ai.GEN_AI_OPERATION_NAME, tlsOperationName(info, true)),
	)
	h.setTLSApplicationAttributes(rootSpan, info)
	root := &tlsRootState{span: rootSpan}
	return context.WithValue(rootCtx, tlsRootStateKey{}, root), root, rootSpan
}

func (h *TLSCallbackHandler) contextWithToolParent(ctx context.Context, info *callbacks.RunInfo, root *tlsRootState) context.Context {
	return h.contextWithToolCallParent(ctx, info, root, compose.GetToolCallID(ctx))
}

func (h *TLSCallbackHandler) contextWithToolCallParent(ctx context.Context, info *callbacks.RunInfo, root *tlsRootState, toolCallID string) context.Context {
	if h == nil || !h.TLSExporterEnabled || info == nil || info.Component != components.ComponentOfTool || root == nil || root.span == nil {
		return ctx
	}

	modelSpan := root.modelSpanForToolCall(toolCallID)
	if !modelSpan.IsValid() {
		return ctx
	}
	currentParent := trace.SpanContextFromContext(ctx)
	rootSpan := root.span.SpanContext()
	if !currentParent.IsValid() || currentParent.SpanID() == rootSpan.SpanID() {
		return trace.ContextWithSpanContext(ctx, modelSpan)
	}
	return ctx
}

func endSyntheticRoot(root trace.Span, status codes.Code, description string) {
	if root == nil {
		return
	}
	root.SetStatus(status, description)
	root.End(trace.WithTimestamp(time.Now()))
}

func (h *TLSCallbackHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if h == nil || info == nil {
		return ctx
	}

	info = completeRunInfo(info)
	ctx = injectToolIDNameMapToCtx(ctx, info, input)

	startTime := time.Now()
	ctx, syntheticRootState, syntheticRoot := h.startSyntheticRootIfNeeded(ctx, info, startTime)
	if syntheticRootState != nil {
		ctx = context.WithValue(ctx, tlsRootStateKey{}, syntheticRootState)
	}
	root, _ := ctx.Value(tlsRootStateKey{}).(*tlsRootState)
	ctx = h.contextWithToolParent(ctx, info, root)
	ctx, span := h.Tracer.Start(ctx, h.spanName(ctx, info), trace.WithSpanKind(h.spanKind(ctx, info)), trace.WithTimestamp(startTime))
	isRootNode := setCommonSpanAttributes(ctx, span, info, h.TLSExporterEnabled)
	h.setTLSApplicationAttributes(span, info)
	if root == nil {
		root = &tlsRootState{span: span}
		ctx = context.WithValue(ctx, tlsRootStateKey{}, root)
	}
	if h.TLSExporterEnabled && info.Component == components.ComponentOfTool {
		toolCallID := compose.GetToolCallID(ctx)
		if toolCallID == "" {
			toolCallID = span.SpanContext().SpanID().String()
		}
		span.SetAttributes(
			attribute.String(sem_ai.GEN_AI_TOOL_NAME, getName(info)),
			attribute.String(sem_ai.GEN_AI_TOOL_TYPE, info.Type),
			attribute.String(sem_ai.GEN_AI_TOOL_CALL_ID, toolCallID),
			attribute.Bool(sem_ai.TOOL_ERROR, false),
			attribute.Bool(sem_ai.TOOL_SUCCESS, true),
		)
	}

	requestInfo := &RequestInfo{}
	if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
		if cbInput := model.ConvCallbackInput(input); cbInput != nil && cbInput.Config != nil {
			requestInfo.setModel(cbInput.Config.Model)
		}
		if modelName := requestInfo.model(); modelName != "" {
			span.SetAttributes(attribute.String(sem_ai.GEN_AI_REQUEST_MODEL, modelName))
		}
	}
	if h.dataParser != nil {
		tags, err := h.dataParser.ParseInput(ctx, info, input)
		if err != nil {
			log.Printf("ParseInput failed, info: %+v, err: %+v", info, err)
		} else {
			if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
				normalizeTLSLensMessages(tags)
			}
			setSpanAttributesFromTags(span, tags)
			if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
				requestInfo.setModel(stringTag(tags, sem_ai.GEN_AI_REQUEST_MODEL))
				root.copyModelPresentation(tags)
			}
		}
	}

	return context.WithValue(ctx, TLSStateKey{}, &TLSState{
		StartTime:     startTime,
		Span:          span,
		RequestInfo:   requestInfo,
		IsRootNode:    isRootNode,
		root:          root,
		syntheticRoot: syntheticRoot,
	})
}

func (h *TLSCallbackHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if h == nil || info == nil {
		return ctx
	}

	state, ok := ctx.Value(TLSStateKey{}).(*TLSState)
	if !ok || state == nil || state.Span == nil {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}
	if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
		if cbOutput := model.ConvCallbackOutput(output); cbOutput != nil {
			state.root.rememberModelSpan(state.Span, modelToolCallIDs(cbOutput.Message)...)
		}
	}

	defer func() {
		endTLSState(ctx, state, codes.Ok, "")
	}()

	if h.dataParser != nil {
		tags, err := h.dataParser.ParseOutput(ctx, completeRunInfo(info), output)
		if err != nil {
			log.Printf("ParseOutput failed, info: %+v, err: %+v", info, err)
		} else {
			if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
				addMissingTLSModelAttributes(tags, state)
			}
			if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
				normalizeTLSLensMessages(tags)
			}
			setSpanAttributesFromTags(state.Span, tags)
			if h.TLSExporterEnabled && info.Component == components.ComponentOfTool {
				state.Span.SetAttributes(
					attribute.Bool(sem_ai.TOOL_ERROR, false),
					attribute.Bool(sem_ai.TOOL_SUCCESS, true),
				)
			}
			if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
				state.root.copyModelPresentation(tags)
				state.root.addModelUsage(tags)
			}
		}
	}

	return ctx
}

func (h *TLSCallbackHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if h == nil || info == nil {
		return ctx
	}

	state, ok := ctx.Value(TLSStateKey{}).(*TLSState)
	if !ok || state == nil || state.Span == nil {
		log.Printf("no state in context, runinfo: %+v", info)
		return ctx
	}

	if err != nil {
		state.Span.RecordError(err)
		state.Span.SetAttributes(
			attribute.String("error.type", fmt.Sprintf("%T", err)),
			attribute.String("error.message", err.Error()),
		)
		if h.TLSExporterEnabled && info.Component == components.ComponentOfTool {
			state.Span.SetAttributes(
				attribute.Bool(sem_ai.TOOL_ERROR, true),
				attribute.Bool(sem_ai.TOOL_SUCCESS, false),
			)
		}
		defer func() {
			endTLSState(ctx, state, codes.Error, err.Error())
		}()
	} else {
		defer func() {
			endTLSState(ctx, state, codes.Error, "")
		}()
	}

	return ctx
}

func (h *TLSCallbackHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if h == nil {
		input.Close()
		return ctx
	}
	if info == nil {
		input.Close()
		return ctx
	}

	info = completeRunInfo(info)
	startTime := time.Now()
	ctx, syntheticRootState, syntheticRoot := h.startSyntheticRootIfNeeded(ctx, info, startTime)
	if syntheticRootState != nil {
		ctx = context.WithValue(ctx, tlsRootStateKey{}, syntheticRootState)
	}
	root, _ := ctx.Value(tlsRootStateKey{}).(*tlsRootState)
	ctx = h.contextWithToolParent(ctx, info, root)
	ctx, span := h.Tracer.Start(ctx, h.spanName(ctx, info), trace.WithSpanKind(h.spanKind(ctx, info)), trace.WithTimestamp(startTime))
	isRootNode := setCommonSpanAttributes(ctx, span, info, h.TLSExporterEnabled)
	h.setTLSApplicationAttributes(span, info)
	if root == nil {
		root = &tlsRootState{span: span}
		ctx = context.WithValue(ctx, tlsRootStateKey{}, root)
	}
	if h.TLSExporterEnabled && info.Component == components.ComponentOfTool {
		toolCallID := compose.GetToolCallID(ctx)
		if toolCallID == "" {
			toolCallID = span.SpanContext().SpanID().String()
		}
		span.SetAttributes(
			attribute.String(sem_ai.GEN_AI_TOOL_NAME, getName(info)),
			attribute.String(sem_ai.GEN_AI_TOOL_TYPE, info.Type),
			attribute.String(sem_ai.GEN_AI_TOOL_CALL_ID, toolCallID),
			attribute.Bool(sem_ai.TOOL_ERROR, false),
			attribute.Bool(sem_ai.TOOL_SUCCESS, true),
		)
	}

	requestInfo := &RequestInfo{}
	stopCh := make(TLSStreamInputAsyncVal)
	ctx = context.WithValue(ctx, TLSStreamInputAsyncKey{}, stopCh)

	if h.dataParser != nil {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					log.Printf("recover update span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
				}
				close(stopCh)
			}()

			tags, err := h.dataParser.ParseStreamInput(ctx, info, input)
			if err != nil {
				log.Printf("ParseStreamInput failed, info: %+v, err: %+v", info, err)
				return
			}
			if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
				normalizeTLSLensMessages(tags)
			}
			setSpanAttributesFromTags(span, tags)
			if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
				requestInfo.setModel(stringTag(tags, sem_ai.GEN_AI_REQUEST_MODEL))
				root.copyModelPresentation(tags)
			}
		}()
	} else {
		input.Close()
		close(stopCh)
	}

	return context.WithValue(ctx, TLSStateKey{}, &TLSState{
		StartTime:     startTime,
		Span:          span,
		RequestInfo:   requestInfo,
		IsRootNode:    isRootNode,
		root:          root,
		syntheticRoot: syntheticRoot,
	})
}

func (h *TLSCallbackHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if h == nil {
		output.Close()
		return ctx
	}
	if info == nil {
		output.Close()
		return ctx
	}

	state, ok := ctx.Value(TLSStateKey{}).(*TLSState)
	if !ok || state == nil || state.Span == nil {
		log.Printf("no state in context, runinfo: %+v", info)
		output.Close()
		return ctx
	}

	info = completeRunInfo(info)
	if h.dataParser != nil {
		completeStreamOutput := state.root.beginStreamOutput()
		go func() {
			defer func() {
				if e := recover(); e != nil {
					log.Printf("recover update span panic: %v, runinfo: %+v, stack: %s", e, info, string(debug.Stack()))
				}
				if state.IsRootNode {
					// This parser belongs to the root itself. Mark it complete
					// before waiting so the root waits only for child streams.
					completeStreamOutput()
					endTLSState(ctx, state, codes.Ok, "")
					return
				}
				// A child parser may still be waiting for its stream input.
				// Complete the child only after endSpan has waited for that
				// input, so its root attributes are ready before the root ends.
				endSpan(ctx, state.Span, codes.Ok, "")
				completeStreamOutput()
				endSyntheticRoot(state.syntheticRoot, codes.Ok, "")
			}()

			tags, err := h.dataParser.ParseStreamOutput(ctx, info, output)
			if err != nil {
				log.Printf("ParseStreamOutput failed, info: %+v, err: %+v", info, err)
				return
			}
			if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
				addMissingTLSModelAttributes(tags, state)
				normalizeTLSLensMessages(tags)
			}
			setSpanAttributesFromTags(state.Span, tags)
			if h.TLSExporterEnabled && info.Component == components.ComponentOfTool {
				state.Span.SetAttributes(
					attribute.Bool(sem_ai.TOOL_ERROR, false),
					attribute.Bool(sem_ai.TOOL_SUCCESS, true),
				)
			}
			if h.TLSExporterEnabled && info.Component == components.ComponentOfChatModel {
				state.root.copyModelPresentation(tags)
				state.root.addModelUsage(tags)
				state.root.rememberModelSpan(state.Span, modelToolCallIDsFromTags(tags)...)
			}
		}()
	} else {
		output.Close()
		endTLSState(ctx, state, codes.Ok, "")
	}

	return ctx
}

func (h *TLSHandler) StartSpan(ctx context.Context, spanName string, tracingTags map[string]any) context.Context {
	return h.StartSpanWithKind(ctx, spanName, trace.SpanKindClient, tracingTags)
}

// StartSpanWithKind starts a manual span with the specified OpenTelemetry span kind.
func (h *TLSHandler) StartSpanWithKind(ctx context.Context, spanName string, spanKind trace.SpanKind, tracingTags map[string]any) context.Context {
	if h == nil {
		return ctx
	}

	startTime := time.Now()
	if spanName == "" {
		spanName = DefaultSpanName
	}

	ctx, span := h.Tracer.Start(ctx, spanName, trace.WithSpanKind(spanKind), trace.WithTimestamp(startTime))
	setSessionAttributes(ctx, span)
	setSpanAttributesFromTags(span, tracingTags)

	return context.WithValue(ctx, TLSStateKey{}, &TLSState{
		StartTime:   startTime,
		Span:        span,
		RequestInfo: nil,
		IsRootNode:  true,
	})
}

func (h *TLSHandler) FinishSpan(ctx context.Context, tracingTags map[string]any) context.Context {
	return h.FinishSpanWithError(ctx, tracingTags, nil)
}

// FinishSpanWithError finishes a manual span and records err when it is non-nil.
func (h *TLSHandler) FinishSpanWithError(ctx context.Context, tracingTags map[string]any, err error) context.Context {
	state, ok := ctx.Value(TLSStateKey{}).(*TLSState)
	if !ok || state == nil || state.Span == nil {
		log.Printf("no state in context")
		return ctx
	}

	setSpanAttributesFromTags(state.Span, tracingTags)
	if err != nil {
		state.Span.RecordError(err)
		state.Span.SetStatus(codes.Error, err.Error())
	} else {
		state.Span.SetStatus(codes.Ok, "")
	}
	state.Span.End(trace.WithTimestamp(time.Now()))
	return ctx
}

// SetSession stores session attributes in ctx.
func (h *TLSHandler) SetSession(ctx context.Context, sessionID, userID string) context.Context {
	return SetSession(ctx, WithSessionID(sessionID), WithUserID(userID))
}

func ValidateTLSConfig(cfg *TLSConfig) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	if cfg.AppName == "" {
		return errors.New("AppName is required")
	}
	if cfg.usesTLSLogTransport() {
		if cfg.TLSLogEndpoint == "" || cfg.TLSLogRegion == "" || cfg.TLSLogTopicID == "" {
			return errors.New("TLSLogEndpoint, TLSLogRegion and TLSLogTopicID are required for the TLS log exporter")
		}
		if cfg.TLSLogAPIKey == "" && (cfg.TLSLogAccessKeyID == "" || cfg.TLSLogAccessKeySecret == "") {
			return errors.New("TLSLogAPIKey or TLSLogAccessKeyID/TLSLogAccessKeySecret is required for the TLS log exporter")
		}
		return nil
	}
	if cfg.TLSEndpoint == "" {
		return errors.New("TLSEndpoint is required")
	}

	if cfg.TLSOTLPHeadersStr != "" {
		cfg.TLSOTLPHeader = parseOTLPHeaders(cfg.TLSOTLPHeadersStr)
	}
	if len(cfg.TLSOTLPHeader) == 0 {
		return errors.New("TLSOTLPHeader is required, either directly set or via TLSOTLPHeadersStr")
	}

	return nil
}

func LoadTLSConfigFromEnv() (*TLSConfig, error) {
	cfg := &TLSConfig{
		TLSEndpoint:           os.Getenv("TLS_ENDPOINT"),
		AppName:               os.Getenv("TLS_APP_NAME"),
		Release:               getEnvOrDefault("TLS_AGENT_VERSION", DefaultVersion),
		TLSOTLPHeadersStr:     os.Getenv("TLS_EXPORTER_OTLP_HEADERS"),
		TLSExporterEnabled:    parseBoolEnv("TLS_EXPORTER_ENABLED"),
		TLSLogEndpoint:        os.Getenv("TLS_LOG_ENDPOINT"),
		TLSLogRegion:          os.Getenv("TLS_LOG_REGION"),
		TLSLogTopicID:         os.Getenv("TLS_LOG_TRACE_TOPIC_ID"),
		TLSLogAPIKey:          os.Getenv("TLS_LOG_API_KEY"),
		TLSLogAccessKeyID:     os.Getenv("TLS_LOG_AK"),
		TLSLogAccessKeySecret: os.Getenv("TLS_LOG_SK"),
	}

	return cfg, ValidateTLSConfig(cfg)
}

func parseBoolEnv(key string) bool {
	value, err := strconv.ParseBool(os.Getenv(key))
	return err == nil && value
}

func (h *TLSCallbackHandler) spanName(ctx context.Context, info *callbacks.RunInfo) string {
	if h != nil && h.TLSExporterEnabled {
		return tlsSpanName(ctx, info)
	}
	return getSpanName(info)
}

func (h *TLSCallbackHandler) spanKind(ctx context.Context, info *callbacks.RunInfo) trace.SpanKind {
	if h != nil && h.TLSExporterEnabled && tlsSpanName(ctx, info) == tlsRootSpanName {
		return trace.SpanKindServer
	}
	return trace.SpanKindClient
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getSpanName(info *callbacks.RunInfo) string {
	spanName := getName(info)
	if spanName == "" {
		return DefaultSpanName
	}

	return spanName
}

func parseOTLPHeaders(headersStr string) map[string]string {
	headers := make(map[string]string)
	if headersStr == "" {
		return headers
	}

	pairs := strings.Split(headersStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	return headers
}
