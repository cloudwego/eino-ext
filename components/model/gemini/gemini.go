package gemini

// A generic Vertex-AI chat-model adapter that supports Gemini (publisher "google")
// and partner models such as Claude via Anthropic. It implements the same
// github.com/cloudwego/eino/components/model.ToolCallingChatModel interface that
// the project already expects.
//
// IMPORTANT: Vertex AI has a limitation where multiple tools are only supported
// when they are all search tools. For function calling tools, only ONE tool
// with function declarations is allowed. This adapter handles this by merging
// all function declarations into a single tool.

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/firebase/genkit/go/core/logger"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	genai "google.golang.org/genai"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

// Constants for the transparent response formatter tool
// This tool is automatically injected when response schema is specified,
// allowing structured output while maintaining API transparency
const (
	RESPONSE_FORMATTER_TOOL_NAME   = "provide_structured_response"
	RESPONSE_FORMATTER_DESCRIPTION = "Use this tool to provide your final structured response. Call this tool when you have completed your task and are ready to give your final answer."
)

// NewChatModel creates a new Gemini chat model instance
//
// Parameters:
//   - ctx: The context for the operation
//   - cfg: Configuration for the Gemini model
//
// Returns:
//   - model.ChatModel: A chat model interface implementation
//   - error: Any error that occurred during creation
//
// Example:
//
//	model, err := gemini.NewChatModel(ctx, &gemini.Config{
//	    Client: client,
//	    Model: "gemini-pro",
//	})
func NewChatModel(_ context.Context, cfg *Config) (*ChatModel, error) {
	if cfg == nil || cfg.Client == nil {
		return nil, fmt.Errorf("gemini: client must be provided")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("gemini: model name must be set")
	}

	pub := cfg.Publisher
	if pub == "" {
		pub = "google"
	}

	// Expand shorthand model name to full Vertex resource when project/location
	// are provided and the string doesn't already look fully qualified.
	if cfg.Project != "" && cfg.Location != "" && !strings.ContainsRune(cfg.Model, '/') {
		cfg.Model = fmt.Sprintf("projects/%s/locations/%s/publishers/%s/models/%s", cfg.Project, cfg.Location, pub, cfg.Model)
	}

	return &ChatModel{
		cli:                   cfg.Client,
		model:                 cfg.Model,
		responseSchema:        cfg.ResponseSchema,
		generateContentConfig: cfg.GenerateContentConfig,
	}, nil
}

type ChatModel struct {
	cli *genai.Client

	model                 string
	responseSchema        *openapi3.Schema
	tools                 []*genai.Tool                // Converted Gemini tools (merged into single tool)
	origTools             []*schema.ToolInfo           // Original tool definitions for callbacks
	toolChoice            *schema.ToolChoice           // Tool usage preference
	generateContentConfig *genai.GenerateContentConfig // Direct access to Gemini config
}

// Generate processes a conversation and returns a single response message
// This method handles the complete flow: message processing, tool aggregation,
// chat session initialization, and response conversion
func (cm *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (message *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	// Extract system instructions and build conversation history
	// This handles tool call/response aggregation and message ordering
	systemInstruction, history, currentMessage, err := cm.processMessages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("process messages fail: %w", err)
	}

	// Initialize chat session with proper configuration and tool setup
	chat, conf, err := cm.initChatSession(ctx, history, systemInstruction, opts...)
	if err != nil {
		return nil, err
	}
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Tools:    model.GetCommonOptions(&model.Options{Tools: cm.origTools}, opts...).Tools,
		Config:   conf,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if len(input) == 0 {
		return nil, fmt.Errorf("gemini input is empty")
	}

	// Send only the current (last) turn to the model
	// Vertex AI manages conversation state through the chat session
	lastParts, err := cm.convSchemaMessageToParts(currentMessage)
	if err != nil {
		return nil, err
	}

	result, err := chat.SendMessage(ctx, lastParts...)
	if err != nil {
		return nil, fmt.Errorf("send message fail: %w", err)
	}

	// Convert Gemini response back to schema format and handle transparency
	message, err = cm.convResponse(result)
	if err != nil {
		return nil, fmt.Errorf("convert response fail: %w", err)
	}

	callbacks.OnEnd(ctx, cm.convCallbackOutput(message, conf))
	return message, nil
}

// Stream provides streaming responses with real-time message processing
// Uses goroutines to handle streaming while maintaining callback integration
func (cm *ChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (result *schema.StreamReader[*schema.Message], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	// Extract system instructions and build conversation history
	systemInstruction, history, currentMessage, err := cm.processMessages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("process messages fail: %w", err)
	}

	chat, conf, err := cm.initChatSession(ctx, history, systemInstruction, opts...)
	if err != nil {
		return nil, err
	}
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Tools:    model.GetCommonOptions(&model.Options{Tools: cm.origTools}, opts...).Tools,
		Config:   conf,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if len(input) == 0 {
		return nil, fmt.Errorf("gemini input is empty")
	}

	// Send only the current (last) turn to the model
	lastParts, err := cm.convSchemaMessageToParts(currentMessage)
	if err != nil {
		return nil, err
	}

	// Set up streaming pipeline with panic recovery
	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				_ = sw.Send(nil, newPanicErr(r, debug.Stack()))
			}
			sw.Close()
		}()

		// Process streaming responses until iterator is done
		for resp, errIter := range chat.SendMessageStream(ctx, lastParts...) {
			if errors.Is(errIter, iterator.Done) {
				return
			}
			if errIter != nil {
				sw.Send(nil, errIter)
				return
			}

			// Convert each streaming chunk and send through pipeline
			message, errConv := cm.convResponse(resp)
			if errConv != nil {
				sw.Send(nil, errConv)
				return
			}
			if closed := sw.Send(cm.convCallbackOutput(message, conf), nil); closed {
				return
			}
		}
	}()

	// Set up dual streams: one for callbacks, one for user
	srList := sr.Copy(2)
	callbacks.OnEndWithStreamOutput(ctx, srList[0])
	return schema.StreamReaderWithConvert(srList[1], func(t *model.CallbackOutput) (*schema.Message, error) {
		return t.Message, nil
	}), nil
}

// WithTools creates a new model instance with bound tools
// Returns a new instance to maintain immutability
func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	// Convert to Gemini format and handle Vertex AI's single-tool limitation
	gTools, err := cm.toGeminiTools(tools)
	if err != nil {
		return nil, fmt.Errorf("convert to gemini tools fail: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	ncm := *cm
	ncm.toolChoice = &tc
	ncm.tools = gTools
	ncm.origTools = tools
	return &ncm, nil
}

// BindTools modifies the current instance to use the provided tools
func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	gTools, err := cm.toGeminiTools(tools)
	if err != nil {
		return err
	}

	cm.tools = gTools
	cm.origTools = tools
	tc := schema.ToolChoiceAllowed
	cm.toolChoice = &tc
	return nil
}

// BindForcedTools binds tools with forced usage (model must use tools)
func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	gTools, err := cm.toGeminiTools(tools)
	if err != nil {
		return err
	}

	cm.tools = gTools
	cm.origTools = tools
	tc := schema.ToolChoiceForced
	cm.toolChoice = &tc
	return nil
}

// initChatSession creates and configures a Gemini chat session
// Handles complex configuration merging, tool setup, and response schema integration
func (cm *ChatModel) initChatSession(ctx context.Context, history []*genai.Content, systemInstruction string, opts ...model.Option) (*genai.Chat, *model.Config, error) {
	// Merge configuration from model defaults and runtime options
	commonOptions := model.GetCommonOptions(&model.Options{
		Tools:      nil,
		ToolChoice: cm.toolChoice,
	}, opts...)
	geminiOptions := model.GetImplSpecificOptions(&options{
		ResponseSchema: cm.responseSchema,
	}, opts...)
	conf := &model.Config{}

	// Build chat configuration
	chatConfig, conf, err := cm.buildChatConfig(commonOptions, geminiOptions, systemInstruction, conf)
	if err != nil {
		return nil, nil, err
	}

	// Handle tools configuration
	tools, err := cm.configureTools(commonOptions, geminiOptions)
	if err != nil {
		return nil, nil, err
	}

	// Apply tools to chat config
	cm.applyToolsToConfig(chatConfig, tools)

	// Configure tool choice behavior
	if err := cm.configureToolChoice(chatConfig, commonOptions, len(tools)); err != nil {
		return nil, nil, err
	}

	// Create chat session with provided history (Vertex limitation compliance)
	modelName := cm.getModelName(commonOptions, conf)
	chat, err := cm.cli.Chats.Create(ctx, modelName, chatConfig, history)
	if err != nil {
		return nil, nil, fmt.Errorf("create chat session fail: %w", err)
	}

	return chat, conf, nil
}

// buildChatConfig creates the base chat configuration and copies values to callback config
func (cm *ChatModel) buildChatConfig(commonOptions *model.Options, geminiOptions *options, systemInstruction string, conf *model.Config) (*genai.GenerateContentConfig, *model.Config, error) {
	// Create chat configuration - use direct Gemini config if available
	var chatConfig *genai.GenerateContentConfig
	if cm.generateContentConfig != nil {
		// Use the provided config directly
		chatConfig = cm.generateContentConfig
	} else {
		// Create empty config if none provided
		chatConfig = &genai.GenerateContentConfig{}
	}

	// Apply Gemini-specific options first
	if geminiOptions.TopK != nil {
		topKFloat := float32(*geminiOptions.TopK)
		chatConfig.TopK = &topKFloat
	}

	// Apply common options to chat config (these take precedence over Gemini-specific options)
	if commonOptions.MaxTokens != nil {
		chatConfig.MaxOutputTokens = int32(*commonOptions.MaxTokens)
	}
	if commonOptions.Temperature != nil {
		chatConfig.Temperature = commonOptions.Temperature
	}
	if commonOptions.TopP != nil {
		chatConfig.TopP = commonOptions.TopP
	}

	// Handle system instructions properly
	if systemInstruction != "" {
		chatConfig.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(systemInstruction)},
		}
	}

	// Copy final config values for callback output
	if chatConfig.MaxOutputTokens != 0 {
		conf.MaxTokens = int(chatConfig.MaxOutputTokens)
	}
	if chatConfig.TopP != nil {
		conf.TopP = *chatConfig.TopP
	}
	if chatConfig.Temperature != nil {
		conf.Temperature = *chatConfig.Temperature
	}

	return chatConfig, conf, nil
}

// configureTools handles tool setup including response formatter integration
func (cm *ChatModel) configureTools(commonOptions *model.Options, geminiOptions *options) ([]*genai.Tool, error) {
	// Handle tools - either from options or model defaults
	tools := cm.tools
	if commonOptions.Tools != nil {
		var err error
		tools, err = cm.toGeminiTools(commonOptions.Tools)
		if err != nil {
			return nil, err
		}
	}

	// Add response formatter function to existing tool when response schema is needed
	// This creates a transparent way to get structured responses
	if geminiOptions.ResponseSchema != nil {
		if len(tools) == 0 {
			// Create new tool with response formatter
			responseFormatterTool, err := cm.createResponseFormatterTool(geminiOptions.ResponseSchema)
			if err != nil {
				return nil, fmt.Errorf("create response formatter tool fail: %w", err)
			}
			tools = []*genai.Tool{responseFormatterTool}
		} else {
			// Add response formatter function to existing tool
			responseFormatterFunc := &genai.FunctionDeclaration{
				Name:        RESPONSE_FORMATTER_TOOL_NAME,
				Description: RESPONSE_FORMATTER_DESCRIPTION,
			}
			parameters, err := cm.convOpenSchema(geminiOptions.ResponseSchema)
			if err != nil {
				return nil, fmt.Errorf("convert response schema fail: %w", err)
			}
			responseFormatterFunc.Parameters = parameters
			tools[0].FunctionDeclarations = append(tools[0].FunctionDeclarations, responseFormatterFunc)
		}
	}

	return tools, nil
}

// applyToolsToConfig applies tools to the chat configuration
func (cm *ChatModel) applyToolsToConfig(chatConfig *genai.GenerateContentConfig, tools []*genai.Tool) {
	if len(tools) > 0 {
		chatConfig.Tools = tools
	}
}

// configureToolChoice sets up the tool choice behavior based on available tools and options
func (cm *ChatModel) configureToolChoice(chatConfig *genai.GenerateContentConfig, commonOptions *model.Options, toolCount int) error {
	// Handle tool choice - configure function calling behavior
	if toolCount > 0 {
		chatConfig.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAny, // Force tool usage
			},
		}
	} else if commonOptions.ToolChoice != nil {
		switch *commonOptions.ToolChoice {
		case schema.ToolChoiceForbidden:
			chatConfig.ToolConfig = &genai.ToolConfig{
				FunctionCallingConfig: &genai.FunctionCallingConfig{
					Mode: genai.FunctionCallingConfigModeUnspecified, // No function calling
				},
			}
		case schema.ToolChoiceAllowed:
			chatConfig.ToolConfig = &genai.ToolConfig{
				FunctionCallingConfig: &genai.FunctionCallingConfig{
					Mode: genai.FunctionCallingConfigModeAuto,
				},
			}
		case schema.ToolChoiceForced:
			return fmt.Errorf("tool choice is forced but no tools are provided")
		default:
			return fmt.Errorf("tool choice=%s not support", *commonOptions.ToolChoice)
		}
	}

	return nil
}

// getModelName determines the model name to use, updating the config if needed
func (cm *ChatModel) getModelName(commonOptions *model.Options, conf *model.Config) string {
	modelName := cm.model
	if commonOptions.Model != nil {
		modelName = *commonOptions.Model
		conf.Model = *commonOptions.Model
	} else {
		conf.Model = cm.model
	}
	return modelName
}

// toGeminiTools converts schema tools to Gemini format
// IMPORTANT: Merges all function declarations into a single tool to comply
// with Vertex AI's limitation of only supporting one function calling tool
func (cm *ChatModel) toGeminiTools(tools []*schema.ToolInfo) ([]*genai.Tool, error) {
	// Collect all function declarations
	var funcDecls []*genai.FunctionDeclaration

	for _, tool := range tools {
		funcDecl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Desc,
		}

		// Convert OpenAPI schema to Gemini schema format
		openSchema, err := tool.ToOpenAPIV3()
		if err != nil {
			return nil, fmt.Errorf("get open schema fail: %w", err)
		}
		funcDecl.Parameters, err = cm.convOpenSchema(openSchema)
		if err != nil {
			return nil, fmt.Errorf("convert open schema fail: %w", err)
		}

		funcDecls = append(funcDecls, funcDecl)
	}

	// Create single tool with all function declarations
	// This is required due to Vertex AI's single function calling tool limitation
	gTool := &genai.Tool{
		FunctionDeclarations: funcDecls,
	}

	return []*genai.Tool{gTool}, nil
}

// convOpenSchema recursively converts OpenAPI v3 schema to Gemini schema format
// Handles complex nested structures, arrays, enums, and all supported types
func (cm *ChatModel) convOpenSchema(schema *openapi3.Schema) (*genai.Schema, error) {
	if schema == nil {
		return nil, nil
	}
	var err error

	// copy nullable into dedicated variable to avoid taking address of struct field
	nullable := schema.Nullable
	result := &genai.Schema{
		Format:      schema.Format,
		Description: schema.Description,
		Nullable:    &nullable,
	}

	switch {
	case schema.Type != nil && schema.Type.Is(openapi3.TypeObject):
		result.Type = genai.TypeObject
		// Convert object properties recursively
		if schema.Properties != nil {
			properties := make(map[string]*genai.Schema)
			for name, prop := range schema.Properties {
				if prop == nil || prop.Value == nil {
					continue
				}
				properties[name], err = cm.convOpenSchema(prop.Value)
				if err != nil {
					return nil, err
				}
			}
			result.Properties = properties
		}
		if schema.Required != nil {
			result.Required = schema.Required
		}

	case schema.Type != nil && schema.Type.Is(openapi3.TypeArray):
		result.Type = genai.TypeArray
		// Convert array item schema recursively
		if schema.Items != nil && schema.Items.Value != nil {
			result.Items, err = cm.convOpenSchema(schema.Items.Value)
			if err != nil {
				return nil, err
			}
		}

	case schema.Type != nil && schema.Type.Is(openapi3.TypeString):
		result.Type = genai.TypeString
		// Handle string enums
		if schema.Enum != nil {
			enums := make([]string, 0, len(schema.Enum))
			for _, e := range schema.Enum {
				if str, ok := e.(string); ok {
					enums = append(enums, str)
				} else {
					return nil, fmt.Errorf("enum value must be a string, schema: %+v", schema)
				}
			}
			result.Enum = enums
		}

	case schema.Type != nil && schema.Type.Is(openapi3.TypeNumber):
		result.Type = genai.TypeNumber
	case schema.Type != nil && schema.Type.Is(openapi3.TypeInteger):
		result.Type = genai.TypeInteger
	case schema.Type != nil && schema.Type.Is(openapi3.TypeBoolean):
		result.Type = genai.TypeBoolean
	default:
		result.Type = genai.TypeUnspecified
		fmt.Printf("unsupported schema type: %v", schema.Type)
	}

	return result, nil
}

// convSchemaMessageToParts converts a schema message to Gemini parts
// Handles different message types: tool calls, tool responses, and regular content
func (cm *ChatModel) convSchemaMessageToParts(message *schema.Message) ([]genai.Part, error) {
	if message == nil {
		return nil, nil
	}

	var parts []genai.Part

	// Only user/assistant messages may contain outgoing tool calls.
	if message.Role != schema.Tool && message.ToolCalls != nil {
		for _, call := range message.ToolCalls {
			// Parse tool call arguments from JSON string
			args := make(map[string]any)
			err := sonic.UnmarshalString(call.Function.Arguments, &args)
			if err != nil {
				return nil, fmt.Errorf("unmarshal schema tool call arguments to map[string]any fail: %w", err)
			}
			functionCall := &genai.FunctionCall{
				Name: call.Function.Name,
				Args: args,
			}
			parts = append(parts, genai.Part{FunctionCall: functionCall})
		}
	}

	// Handle tool response messages
	if message.Role == schema.Tool {
		// Handle different tool response types - convert to appropriate format for Vertex AI
		var response any
		err := sonic.UnmarshalString(message.Content, &response)
		if err != nil {
			return nil, fmt.Errorf("unmarshal schema tool call response fail: %w", err)
		}

		// Ensure response is in the format expected by Vertex AI
		var responseMap map[string]any
		if respMap, ok := response.(map[string]any); ok {
			// Already a map - use as is
			responseMap = respMap
		} else {
			// For primitive values, wrap in a result object
			responseMap = map[string]any{"result": response}
		}
		functionResponse := &genai.FunctionResponse{
			// ID:       message.ToolCallID,  // Vertex AI doesn't require ID matching
			Name:     message.ToolName,
			Response: responseMap,
		}
		parts = append(parts, genai.Part{FunctionResponse: functionResponse})
	} else {
		// Handle regular text content
		if message.Content != "" {
			parts = append(parts, *genai.NewPartFromText(message.Content))
		}
		// Handle multimedia content (images, audio, video, files)
		mediaParts := cm.convMedia(message.MultiContent)
		parts = append(parts, mediaParts...)
	}
	return parts, nil
}

// convMedia converts multi-content parts to Gemini parts
// Supports text, images, audio, video, and file URIs
func (cm *ChatModel) convMedia(contents []schema.ChatMessagePart) []genai.Part {
	result := make([]genai.Part, 0, len(contents))
	for _, content := range contents {
		switch content.Type {
		case schema.ChatMessagePartTypeText:
			result = append(result, *genai.NewPartFromText(content.Text))
		case schema.ChatMessagePartTypeImageURL:
			if content.ImageURL != nil {
				result = append(result, *genai.NewPartFromURI(content.ImageURL.URI, content.ImageURL.MIMEType))
			}
		case schema.ChatMessagePartTypeAudioURL:
			if content.AudioURL != nil {
				result = append(result, *genai.NewPartFromURI(content.AudioURL.URI, content.AudioURL.MIMEType))
			}
		case schema.ChatMessagePartTypeVideoURL:
			if content.VideoURL != nil {
				result = append(result, *genai.NewPartFromURI(content.VideoURL.URI, content.VideoURL.MIMEType))
			}
		case schema.ChatMessagePartTypeFileURL:
			if content.FileURL != nil {
				result = append(result, *genai.NewPartFromURI(content.FileURL.URI, content.FileURL.MIMEType))
			}
		}
	}
	return result
}

// convResponse converts Gemini response to schema format
// Handles token usage, multiple candidates, and response transparency
func (cm *ChatModel) convResponse(resp *genai.GenerateContentResponse) (*schema.Message, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini result is empty")
	}

	message, err := cm.convCandidate(resp.Candidates[0])
	if err != nil {
		return nil, fmt.Errorf("convert candidate fail: %w", err)
	}

	// Make response formatter tool transparent to the user
	message = cm.makeResponseFormatterTransparent(message)

	// Add token usage metadata if available
	if resp.UsageMetadata != nil {
		if message.ResponseMeta == nil {
			message.ResponseMeta = &schema.ResponseMeta{}
		}
		message.ResponseMeta.Usage = &schema.TokenUsage{
			PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
		}
	}
	return message, nil
}

// convCandidate converts a single Gemini candidate to schema message
// Processes different part types: text, function calls, code execution results
func (cm *ChatModel) convCandidate(candidate *genai.Candidate) (*schema.Message, error) {
	result := &schema.Message{}
	result.ResponseMeta = &schema.ResponseMeta{
		FinishReason: string(candidate.FinishReason),
	}
	if candidate.Content != nil {
		// Convert Gemini role to schema role
		if candidate.Content.Role == roleModel {
			result.Role = schema.Assistant
		} else {
			result.Role = schema.User
		}

		var texts []string
		// Process each part of the candidate content
		for _, part := range candidate.Content.Parts {
			switch {
			case part.Text != "":
				texts = append(texts, part.Text)
			case part.FunctionCall != nil:
				// Convert function call to schema format
				fc, err := convFC(part.FunctionCall)
				if err != nil {
					return nil, err
				}
				result.ToolCalls = append(result.ToolCalls, *fc)
			case part.CodeExecutionResult != nil:
				texts = append(texts, part.CodeExecutionResult.Output)
			case part.ExecutableCode != nil:
				texts = append(texts, part.ExecutableCode.Code)
			default:
				return nil, fmt.Errorf("unsupported part type: %+v", part)
			}
		}

		// Consolidate text content appropriately
		if len(texts) == 1 {
			result.Content = texts[0]
		} else if len(texts) > 1 {
			// Multiple text parts become multi-content
			for _, text := range texts {
				result.MultiContent = append(result.MultiContent, schema.ChatMessagePart{
					Type: schema.ChatMessagePartTypeText,
					Text: text,
				})
			}
		}
	}
	return result, nil
}

// generateToolCallID creates a unique tool call ID in the format name_uuid
func generateToolCallID(functionName string) string {
	return fmt.Sprintf("%s_%s", functionName, uuid.New().String())
}

// convFC converts Gemini function call to schema format
// Handles argument marshaling and ID generation
func convFC(tp *genai.FunctionCall) (*schema.ToolCall, error) {
	args, err := sonic.MarshalString(tp.Args)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini tool call arguments fail: %w", err)
	}

	// Use existing ID if available, otherwise generate a new one
	var id string
	if tp.ID != "" {
		id = tp.ID
	} else {
		id = generateToolCallID(tp.Name)
	}

	return &schema.ToolCall{
		ID: id,
		Function: schema.FunctionCall{
			Name:      tp.Name,
			Arguments: args,
		},
	}, nil
}

func (cm *ChatModel) convCallbackOutput(message *schema.Message, conf *model.Config) *model.CallbackOutput {
	callbackOutput := &model.CallbackOutput{
		Message: message,
		Config:  conf,
	}
	if message.ResponseMeta != nil && message.ResponseMeta.Usage != nil {
		callbackOutput.TokenUsage = &model.TokenUsage{
			PromptTokens:     message.ResponseMeta.Usage.PromptTokens,
			CompletionTokens: message.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      message.ResponseMeta.Usage.TotalTokens,
		}
	}
	return callbackOutput
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

const (
	roleModel = "model"
	roleUser  = "user"
)

func toGeminiRole(role schema.RoleType) string {
	if role == schema.Assistant {
		return roleModel
	}
	return roleUser
}

const typ = "Gemini"

func (cm *ChatModel) GetType() string {
	return typ
}

// aggregateToolResponses reorganizes messages to properly pair tool calls with their responses
// This is crucial for Vertex AI which expects proper call/response ordering in conversation history
//
// The process:
// 1. Collect all tool call IDs and their corresponding responses
// 2. Split messages with multiple tool calls into individual call messages
// 3. Interleave each call with its matching response
// 4. Filter out unmatched responses to avoid API errors
func (cm *ChatModel) aggregateToolResponses(ctx context.Context, messages []*schema.Message) []*schema.Message {
	// First pass: collect all tool call IDs and responses
	allToolCallIDs := make(map[string]bool)
	responseMap := make(map[string]*schema.Message)

	// Collect all tool call IDs from all messages
	for _, msg := range messages {
		if msg.ToolCalls != nil {
			for _, call := range msg.ToolCalls {
				allToolCallIDs[call.ID] = true
			}
		}
	}

	// Collect all tool responses
	for _, msg := range messages {
		if msg.Role == schema.Tool && msg.ToolCallID != "" {
			responseMap[msg.ToolCallID] = msg
		}
	}

	// Second pass: process messages and interleave calls with responses
	var result []*schema.Message

	for _, msg := range messages {
		// Handle non-tool-call messages
		if len(msg.ToolCalls) == 0 {
			// Only add non-tool-call messages that are not tool responses
			if msg.Role != schema.Tool {
				result = append(result, msg)
			}
			continue
		}

		// Split multi-call message into individual call messages
		// This ensures each tool call gets its own conversation turn
		callMessages := cm.splitToolCallMessage(msg)

		// Interleave calls with their matching responses
		for _, callMsg := range callMessages {
			// Add matching response if found
			if resp, found := responseMap[callMsg.ToolCalls[0].ID]; found {
				result = append(result, callMsg)
				result = append(result, resp)
				delete(responseMap, callMsg.ToolCalls[0].ID)
			}
		}
	}

	// Log any truly unmatched responses (those that don't have corresponding tool calls)
	log := logger.FromContext(ctx)
	for respID, resp := range responseMap {
		if !allToolCallIDs[respID] {
			log.Warn("Tool response has no corresponding tool call", "response_id", respID, "tool_name", resp.ToolName)
		}
	}

	return result
}

// splitToolCallMessage splits a message with multiple tool calls into individual messages
// This is necessary because Vertex AI processes tool calls better when each has its own turn
func (cm *ChatModel) splitToolCallMessage(msg *schema.Message) []*schema.Message {
	if len(msg.ToolCalls) <= 1 {
		return []*schema.Message{msg}
	}

	var result []*schema.Message
	for _, call := range msg.ToolCalls {
		// Create proper deep copy for all calls to ensure complete isolation
		callMsg := &schema.Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: call.ID,
			ToolName:   call.Function.Name,
			ToolCalls:  []schema.ToolCall{call}, // Single tool call per message
		}

		// Deep copy MultiContent slice if present
		if msg.MultiContent != nil {
			callMsg.MultiContent = make([]schema.ChatMessagePart, len(msg.MultiContent))
			copy(callMsg.MultiContent, msg.MultiContent)
		}

		// Deep copy ResponseMeta if present
		if msg.ResponseMeta != nil {
			callMsg.ResponseMeta = &schema.ResponseMeta{
				FinishReason: msg.ResponseMeta.FinishReason,
			}
			if msg.ResponseMeta.Usage != nil {
				callMsg.ResponseMeta.Usage = &schema.TokenUsage{
					PromptTokens:     msg.ResponseMeta.Usage.PromptTokens,
					CompletionTokens: msg.ResponseMeta.Usage.CompletionTokens,
					TotalTokens:      msg.ResponseMeta.Usage.TotalTokens,
				}
			}
		}

		result = append(result, callMsg)
	}

	return result
}

// processMessages orchestrates the complete message processing pipeline
// This includes tool call aggregation, system instruction extraction, and history building
func (cm *ChatModel) processMessages(ctx context.Context, input []*schema.Message) (systemInstruction string, history []*genai.Content, currentMessage *schema.Message, err error) {
	if len(input) == 0 {
		return "", nil, nil, fmt.Errorf("no input messages provided")
	}

	// Aggregate tool responses by ID before processing
	// This ensures proper call/response pairing for Vertex AI
	aggregatedMessages := cm.aggregateToolResponses(ctx, input)

	// Extract system instructions and build conversation history
	systemInstruction, history, currentMessage = cm.extractSystemAndHistory(aggregatedMessages)

	// Add response formatter instruction to system prompt if needed
	// This ensures the model knows to use the transparent response tool
	systemInstruction = cm.addResponseFormatterInstruction(systemInstruction)

	return systemInstruction, history, currentMessage, nil
}

// extractSystemAndHistory separates system instructions from conversation history
// System messages are collected into systemInstruction, others become conversation history
func (cm *ChatModel) extractSystemAndHistory(input []*schema.Message) (systemInstruction string, history []*genai.Content, currentMessage *schema.Message) {
	// Process historical messages (all except the last)
	if len(input) > 1 {
		for _, m := range input[:len(input)-1] {
			if m.Role == schema.System {
				// Collect system messages as system instructions
				if systemInstruction != "" {
					systemInstruction += "\n"
				}
				systemInstruction += m.Content
				continue
			}

			// Add non-system messages to conversation history
			parts, err := cm.convSchemaMessageToParts(m)
			if err != nil {
				// Log error but continue processing
				continue
			}
			// Convert parts to pointers for Gemini API
			ptrParts := make([]*genai.Part, len(parts))
			for i := range parts {
				ptrParts[i] = &parts[i]
			}
			history = append(history, &genai.Content{Role: toGeminiRole(m.Role), Parts: ptrParts})
		}
	}

	// Handle the current (last) message
	lastMsg := input[len(input)-1]
	if lastMsg.Role == schema.System {
		// If the current message is a system message, add it to system instructions
		// and create a minimal user message to continue the conversation
		if systemInstruction != "" {
			systemInstruction += "\n"
		}
		systemInstruction += lastMsg.Content

		// Create a simple acknowledgment message to keep the conversation flowing
		currentMessage = &schema.Message{
			Role:    schema.User,
			Content: "Please respond based on the system instructions provided.",
		}
	} else {
		currentMessage = lastMsg
	}

	return systemInstruction, history, currentMessage
}

// addResponseFormatterInstruction injects the response formatter instruction
// This ensures the model uses the transparent response tool when response schema is specified
func (cm *ChatModel) addResponseFormatterInstruction(systemInstruction string) string {
	if cm.responseSchema == nil {
		return systemInstruction
	}

	responseFormatterInstruction := "\n\nIMPORTANT: After completing any necessary tool operations, you MUST provide your final response using the 'provide_structured_response' tool. This is required for all responses."

	// Try to add to the system instruction
	if systemInstruction != "" {
		systemInstruction += "\n"
	}
	systemInstruction += responseFormatterInstruction

	return systemInstruction
}

// createResponseFormatterTool creates the transparent response formatter tool
// This tool allows structured output while maintaining API transparency
func (cm *ChatModel) createResponseFormatterTool(schema *openapi3.Schema) (*genai.Tool, error) {
	tool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        RESPONSE_FORMATTER_TOOL_NAME,
				Description: RESPONSE_FORMATTER_DESCRIPTION,
			},
		},
	}

	parameters, err := cm.convOpenSchema(schema)
	if err != nil {
		return nil, fmt.Errorf("convert response schema to parameters fail: %w", err)
	}
	tool.FunctionDeclarations[0].Parameters = parameters
	return tool, nil
}

// makeResponseFormatterTransparent extracts structured response from response formatter tool
// and makes it appear as if the model returned it directly
// This provides transparent structured output without exposing the internal tool mechanism
func (cm *ChatModel) makeResponseFormatterTransparent(message *schema.Message) *schema.Message {
	if message.ToolCalls == nil {
		return message
	}

	var filteredToolCalls []schema.ToolCall
	for _, toolCall := range message.ToolCalls {
		if toolCall.Function.Name == RESPONSE_FORMATTER_TOOL_NAME {
			// Extract structured response and put in Content
			// This makes the response appear as normal text output
			message.Content = toolCall.Function.Arguments
		} else {
			// Keep other tool calls
			filteredToolCalls = append(filteredToolCalls, toolCall)
		}
	}

	// After filtering, clean up empty tool calls
	if len(filteredToolCalls) == 0 {
		message.ToolCalls = nil // Complete transparency
	} else {
		message.ToolCalls = filteredToolCalls
	}
	return message
}
