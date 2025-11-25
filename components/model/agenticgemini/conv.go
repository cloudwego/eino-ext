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

package agenticgemini

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
	gemini_schema "github.com/cloudwego/eino/schema/gemini"
)

const (
	roleModel = "model"
	roleUser  = "user"
)

// convAgenticMessages converts a slice of AgenticMessage to genai.Content
func convAgenticMessages(messages []*schema.AgenticMessage) ([]*genai.Content, error) {
	result := make([]*genai.Content, len(messages))
	for i, message := range messages {
		content, err := convAgenticMessage(message)
		if err != nil {
			return nil, fmt.Errorf("convert agentic message fail: %w", err)
		}
		result[i] = content
	}
	return result, nil
}

// convAgenticMessage converts a single AgenticMessage to genai.Content
func convAgenticMessage(message *schema.AgenticMessage) (*genai.Content, error) {
	if message == nil {
		return nil, nil
	}

	var err error
	content := &genai.Content{
		Role: toGeminiRole(message.Role),
	}

	for _, block := range message.ContentBlocks {
		if block == nil {
			continue
		}
		var part *genai.Part
		switch block.Type {
		case schema.ContentBlockTypeReasoning:
			if block.Reasoning != nil {
				part = genai.NewPartFromText(block.Reasoning.Text)
				part.Thought = true
			}

		case schema.ContentBlockTypeUserInputText:
			if block.UserInputText != nil {
				part = genai.NewPartFromText(block.UserInputText.Text)
			}

		case schema.ContentBlockTypeUserInputImage:
			if block.UserInputImage != nil {
				part, err = convUserInputImage(block.UserInputImage)
				if err != nil {
					return nil, err
				}
			}

		case schema.ContentBlockTypeUserInputAudio:
			if block.UserInputAudio != nil {
				part, err = convUserInputAudio(block.UserInputAudio)
				if err != nil {
					return nil, err
				}
			}

		case schema.ContentBlockTypeUserInputVideo:
			if block.UserInputVideo != nil {
				part, err = convUserInputVideo(block.UserInputVideo)
				if err != nil {
					return nil, err
				}
			}

		case schema.ContentBlockTypeUserInputFile:
			if block.UserInputFile != nil {
				part, err = convUserInputFile(block.UserInputFile)
				if err != nil {
					return nil, err
				}
			}

		case schema.ContentBlockTypeAssistantGenText:
			if block.AssistantGenText != nil {
				part = genai.NewPartFromText(block.AssistantGenText.Text)
			}

		case schema.ContentBlockTypeAssistantGenImage:
			if block.AssistantGenImage != nil {
				part, err = convAssistantGenImage(block.AssistantGenImage)
				if err != nil {
					return nil, err
				}
			}
		case schema.ContentBlockTypeAssistantGenAudio:
			if block.AssistantGenAudio != nil {
				part, err = convAssistantGenAudio(block.AssistantGenAudio)
				if err != nil {
					return nil, err
				}
			}

		case schema.ContentBlockTypeAssistantGenVideo:
			if block.AssistantGenVideo != nil {
				part, err = convAssistantGenVideo(block.AssistantGenVideo)
				if err != nil {
					return nil, err
				}
			}

		case schema.ContentBlockTypeFunctionToolCall:
			if block.FunctionToolCall != nil {
				part, err = convFunctionToolCall(block.FunctionToolCall)
				if err != nil {
					return nil, err
				}
			}

		case schema.ContentBlockTypeFunctionToolResult:
			if block.FunctionToolResult != nil {
				part, err = convFunctionToolResult(block.FunctionToolResult)
				if err != nil {
					return nil, err
				}
			}

		case schema.ContentBlockTypeServerToolCall:
			if block.ServerToolCall != nil {
				switch block.ServerToolCall.Name {
				case ServerToolNameCodeExecution:
					result, ok := block.ServerToolCall.Arguments.(*ServerToolCallArguments)
					if !ok {
						return nil, fmt.Errorf("failed to convert to genai content: CodeExecution tool call argument isn't *ServerToolCallArguments")
					}
					if result.ExecutableCode == nil {
						return nil, fmt.Errorf("failed to convert to genai content: CodeExecution tool call argument is empty")
					}
					part = genai.NewPartFromExecutableCode(result.ExecutableCode.Code, genai.Language(result.ExecutableCode.Language))
				default:
					return nil, fmt.Errorf("invalid server tool call name: %s", block.ServerToolCall.Name)
				}
			}
		case schema.ContentBlockTypeServerToolResult:
			if block.ServerToolResult != nil {
				switch block.ServerToolResult.Name {
				case ServerToolNameCodeExecution:
					result, ok := block.ServerToolResult.Result.(*ServerToolCallResult)
					if !ok {
						return nil, fmt.Errorf("failed to convert to genai content: CodeExecution tool result isn't *ServerToolCallResult")
					}
					if result.CodeExecutionResult == nil {
						return nil, fmt.Errorf("failed to convert to genai content: CodeExecution tool result is empty")
					}
					part = genai.NewPartFromCodeExecutionResult(genai.Outcome(result.CodeExecutionResult.Outcome), result.CodeExecutionResult.Output)
				}
			}
		default:
			return nil, fmt.Errorf("unsupported content block type: %s", block.Type)
		}

		if part != nil {
			if ts := getThoughtSignature(block); len(ts) > 0 {
				part.ThoughtSignature = ts
			}
			content.Parts = append(content.Parts, part)
		}
	}

	return content, nil
}

func convUserInputImage(img *schema.UserInputImage) (*genai.Part, error) {
	if img.Base64Data != "" {
		b, err := decodeBase64DataURL(img.Base64Data)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to genai content: base64 decode failed: %v", err)
		}
		return genai.NewPartFromBytes(b, img.MIMEType), nil
	} else if img.URL != "" {
		return genai.NewPartFromURI(img.URL, img.MIMEType), nil
	}
	return nil, fmt.Errorf("image must have either URL or Base64Data")
}

func convAssistantGenImage(img *schema.AssistantGenImage) (*genai.Part, error) {
	if img.Base64Data != "" {
		b, err := decodeBase64DataURL(img.Base64Data)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to genai content: base64 decode failed: %v", err)
		}
		return genai.NewPartFromBytes(b, img.MIMEType), nil
	} else if img.URL != "" {
		return genai.NewPartFromURI(img.URL, img.MIMEType), nil
	}
	return nil, fmt.Errorf("image must have either Base64Data or URL")
}

func convUserInputAudio(audio *schema.UserInputAudio) (*genai.Part, error) {
	if audio.Base64Data != "" {
		b, err := decodeBase64DataURL(audio.Base64Data)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to genai content: base64 decode failed: %v", err)
		}
		return genai.NewPartFromBytes(b, audio.MIMEType), nil
	} else if audio.URL != "" {
		return genai.NewPartFromURI(audio.URL, audio.MIMEType), nil
	}
	return nil, fmt.Errorf("audio must have either URL or Base64Data")
}

func convAssistantGenAudio(audio *schema.AssistantGenAudio) (*genai.Part, error) {
	if audio.Base64Data != "" {
		b, err := decodeBase64DataURL(audio.Base64Data)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to genai content: base64 decode failed: %v", err)
		}
		return genai.NewPartFromBytes(b, audio.MIMEType), nil
	} else if audio.URL != "" {
		return genai.NewPartFromURI(audio.URL, audio.MIMEType), nil
	}
	return nil, fmt.Errorf("audio must have either Base64Data or URL")
}

func convUserInputVideo(video *schema.UserInputVideo) (*genai.Part, error) {
	if video.Base64Data != "" {
		b, err := decodeBase64DataURL(video.Base64Data)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to genai content: base64 decode failed: %v", err)
		}
		return genai.NewPartFromBytes(b, video.MIMEType), nil
	} else if video.URL != "" {
		return genai.NewPartFromURI(video.URL, video.MIMEType), nil
	}
	return nil, fmt.Errorf("video must have either URL or Base64Data")
}

func convAssistantGenVideo(video *schema.AssistantGenVideo) (*genai.Part, error) {
	if video.Base64Data != "" {
		b, err := decodeBase64DataURL(video.Base64Data)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to genai content: base64 decode failed: %v", err)
		}
		return genai.NewPartFromBytes(b, video.MIMEType), nil
	} else if video.URL != "" {
		return genai.NewPartFromURI(video.URL, video.MIMEType), nil
	}
	return nil, fmt.Errorf("video must have either Base64Data or URL")
}

func convUserInputFile(file *schema.UserInputFile) (*genai.Part, error) {
	if file.Base64Data != "" {
		b, err := decodeBase64DataURL(file.Base64Data)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to genai content: base64 decode failed: %v", err)
		}
		return genai.NewPartFromBytes(b, file.MIMEType), nil
	} else if file.URL != "" {
		return genai.NewPartFromURI(file.URL, file.MIMEType), nil
	}
	return nil, fmt.Errorf("file must have either URL or Base64Data")
}

func convFunctionToolCall(call *schema.FunctionToolCall) (*genai.Part, error) {
	args := make(map[string]any)
	err := sonic.UnmarshalString(call.Arguments, &args)
	if err != nil {
		return nil, fmt.Errorf("unmarshal function tool call arguments to map[string]any fail: %w", err)
	}

	return genai.NewPartFromFunctionCall(call.Name, args), nil
}

func convFunctionToolResult(result *schema.FunctionToolResult) (*genai.Part, error) {
	response := make(map[string]any)
	err := sonic.UnmarshalString(result.Result, &response)
	if err != nil {
		response["output"] = result.Result
	}
	return genai.NewPartFromFunctionResponse(result.Name, response), nil
}

func convAgenticResponse(resp *genai.GenerateContentResponse, lastType schema.ContentBlockType) (*schema.AgenticMessage, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini result is empty")
	}

	message, err := convAgenticCandidate(resp.Candidates[0], lastType)
	if err != nil {
		return nil, fmt.Errorf("convert candidate fail: %w", err)
	}

	if resp.UsageMetadata != nil {
		if message.ResponseMeta == nil {
			message.ResponseMeta = &schema.AgenticResponseMeta{}
		}
		message.ResponseMeta.TokenUsage = &schema.TokenUsage{
			PromptTokens: int(resp.UsageMetadata.PromptTokenCount),
			PromptTokenDetails: schema.PromptTokenDetails{
				CachedTokens: int(resp.UsageMetadata.CachedContentTokenCount),
			},
			CompletionTokensDetails: schema.CompletionTokensDetails{
				ReasoningTokens: int(resp.UsageMetadata.ThoughtsTokenCount),
			},
			CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
		}
	}
	return message, nil
}

func convAgenticCandidate(candidate *genai.Candidate, lastType schema.ContentBlockType) (*schema.AgenticMessage, error) {
	var err error
	result := &schema.AgenticMessage{
		ResponseMeta: &schema.AgenticResponseMeta{
			GeminiExtension: &gemini_schema.ResponseMetaExtension{
				FinishReason:  string(candidate.FinishReason),
				GroundingMeta: convGroundingMetadata(candidate.GroundingMetadata),
			},
		},
		ContentBlocks: make([]*schema.ContentBlock, 0),
	}

	if candidate.Content == nil {
		return result, nil
	}

	if candidate.Content.Role == roleModel {
		result.Role = schema.AgenticRoleTypeAssistant
	} else {
		result.Role = schema.AgenticRoleTypeUser
	}

	for _, part := range candidate.Content.Parts {
		cb := &schema.ContentBlock{}
		if part.CodeExecutionResult != nil {
			cb.Type = schema.ContentBlockTypeServerToolResult
			cb.ServerToolResult = &schema.ServerToolResult{
				Name: ServerToolNameCodeExecution,
				Result: &ServerToolCallResult{
					CodeExecutionResult: &CodeExecutionResult{
						Outcome: Outcome(part.CodeExecutionResult.Outcome),
						Output:  part.CodeExecutionResult.Output,
					},
				},
			}
		} else if part.ExecutableCode != nil {
			cb.Type = schema.ContentBlockTypeServerToolCall
			cb.ServerToolCall = &schema.ServerToolCall{
				Name: ServerToolNameCodeExecution,
				Arguments: &ServerToolCallArguments{ExecutableCode: &ExecutableCode{
					Code:     part.ExecutableCode.Code,
					Language: Language(part.ExecutableCode.Language),
				}},
			}
		} else if part.FileData != nil {
			cb, err = convAgenticFileData(part.FileData)
			if err != nil {
				return nil, fmt.Errorf("convert file data fail: %w", err)
			}
		} else if part.FunctionCall != nil {
			cb, err = convAgenticFC(part.FunctionCall)
			if err != nil {
				return nil, fmt.Errorf("convert function call fail: %w", err)
			}
		} else if part.FunctionResponse != nil {
			// unreachable
		} else if part.InlineData != nil {
			cb, err = convAgenticInlineData(part.InlineData)
			if err != nil {
				return nil, fmt.Errorf("convert inline data fail: %w", err)
			}
		} else if len(part.Text) > 0 {
			if part.Thought {
				cb.Type = schema.ContentBlockTypeReasoning
				cb.Reasoning = &schema.Reasoning{
					Text: part.Text,
				}
			} else {
				cb.Type = schema.ContentBlockTypeAssistantGenText
				cb.AssistantGenText = &schema.AssistantGenText{
					Text: part.Text,
				}
			}
		} else {
			// thought signature will be a single chunk in streaming, set it to the last content type block
			cb = createContentBlockFromType(lastType)
		}

		if len(cb.Type) > 0 {
			if len(part.ThoughtSignature) > 0 {
				setThoughtSignature(cb, part.ThoughtSignature)
			}
			result.ContentBlocks = append(result.ContentBlocks, cb)
		}
	}

	return result, nil
}

func convGroundingMetadata(gm *genai.GroundingMetadata) *gemini_schema.GroundingMetadata {
	if gm == nil {
		return nil
	}
	ret := &gemini_schema.GroundingMetadata{}
	for _, chunk := range gm.GroundingChunks {
		if chunk.Web == nil {
			continue
		}
		ret.GroundingChunks = append(ret.GroundingChunks, &gemini_schema.GroundingChunk{
			Web: &gemini_schema.GroundingChunkWeb{
				Domain: chunk.Web.Domain,
				Title:  chunk.Web.Title,
				URI:    chunk.Web.URI,
			},
		})
	}
	for _, support := range gm.GroundingSupports {
		if support == nil {
			continue
		}
		nSupport := &gemini_schema.GroundingSupport{}
		if support.Segment != nil {
			nSupport.Segment = &gemini_schema.Segment{
				EndIndex:   int(support.Segment.EndIndex),
				PartIndex:  int(support.Segment.PartIndex),
				StartIndex: int(support.Segment.StartIndex),
				Text:       support.Segment.Text,
			}
		}
		for _, index := range support.GroundingChunkIndices {
			nSupport.GroundingChunkIndices = append(nSupport.GroundingChunkIndices, int(index))
		}
		nSupport.ConfidenceScores = support.ConfidenceScores
	}
	if gm.SearchEntryPoint != nil {
		ret.SearchEntryPoint = &gemini_schema.SearchEntryPoint{
			RenderedContent: gm.SearchEntryPoint.RenderedContent,
			SDKBlob:         gm.SearchEntryPoint.SDKBlob,
		}
	}
	if gm.WebSearchQueries != nil {
		ret.WebSearchQueries = gm.WebSearchQueries
	}
	return ret
}

func convAgenticFC(fc *genai.FunctionCall) (*schema.ContentBlock, error) {
	if fc == nil {
		return nil, nil
	}

	args, err := sonic.MarshalString(fc.Args)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini tool call arguments fail: %w", err)
	}

	return &schema.ContentBlock{
		Type: schema.ContentBlockTypeFunctionToolCall,
		FunctionToolCall: &schema.FunctionToolCall{
			Name:      fc.Name,
			Arguments: args,
		},
	}, nil
}

func convAgenticInlineData(data *genai.Blob) (*schema.ContentBlock, error) {
	if data == nil {
		return nil, nil
	}
	mimeType := data.MIMEType
	multiMediaData := base64.StdEncoding.EncodeToString(data.Data)

	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return &schema.ContentBlock{
			Type: schema.ContentBlockTypeAssistantGenImage,
			AssistantGenImage: &schema.AssistantGenImage{
				Base64Data: multiMediaData,
				MIMEType:   mimeType,
			},
		}, nil
	case strings.HasPrefix(mimeType, "audio/"):
		return &schema.ContentBlock{
			Type: schema.ContentBlockTypeAssistantGenAudio,
			AssistantGenAudio: &schema.AssistantGenAudio{
				Base64Data: multiMediaData,
				MIMEType:   mimeType,
			},
		}, nil
	case strings.HasPrefix(mimeType, "video/"):
		return &schema.ContentBlock{
			Type: schema.ContentBlockTypeAssistantGenVideo,
			AssistantGenVideo: &schema.AssistantGenVideo{
				Base64Data: multiMediaData,
				MIMEType:   mimeType,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown media type from Gemini model response: MIMEType=%s", mimeType)
	}
}

func convAgenticFileData(data *genai.FileData) (*schema.ContentBlock, error) {
	if data == nil {
		return nil, nil
	}
	mimeType := data.MIMEType
	multiMediaData := data.FileURI

	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return &schema.ContentBlock{
			Type: schema.ContentBlockTypeAssistantGenImage,
			AssistantGenImage: &schema.AssistantGenImage{
				URL:      multiMediaData,
				MIMEType: mimeType,
			},
		}, nil
	case strings.HasPrefix(mimeType, "audio/"):
		return &schema.ContentBlock{
			Type: schema.ContentBlockTypeAssistantGenAudio,
			AssistantGenAudio: &schema.AssistantGenAudio{
				URL:      multiMediaData,
				MIMEType: mimeType,
			},
		}, nil
	case strings.HasPrefix(mimeType, "video/"):
		return &schema.ContentBlock{
			Type: schema.ContentBlockTypeAssistantGenVideo,
			AssistantGenVideo: &schema.AssistantGenVideo{
				URL:      multiMediaData,
				MIMEType: mimeType,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown media type from Gemini model response: MIMEType=%s", mimeType)
	}
}

func toGeminiRole(role schema.AgenticRoleType) string {
	if role == schema.AgenticRoleTypeAssistant {
		return roleModel
	}
	return roleUser
}

func populateStreamingMeta(curBlocks []*schema.ContentBlock, curIndex int, lastType schema.ContentBlockType) (int, schema.ContentBlockType) {
	if len(curBlocks) == 0 {
		return curIndex, lastType
	}
	if len(lastType) > 0 && curBlocks[0].Type != lastType {
		// a new part, index++
		curIndex++
	}

	i := 0
	for ; i < len(curBlocks)-1; i++ {
		block := curBlocks[i]
		block.StreamingMeta = &schema.StreamingMeta{
			Index: curIndex,
		}
		curIndex++
	}
	curBlocks[i].StreamingMeta = &schema.StreamingMeta{Index: curIndex}

	return curIndex, curBlocks[len(curBlocks)-1].Type
}

func toGeminiTools(tools []*schema.ToolInfo) ([]*genai.FunctionDeclaration, error) {
	gTools := make([]*genai.FunctionDeclaration, len(tools))
	for i, tool := range tools {
		funcDecl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Desc,
		}

		var err error
		funcDecl.ParametersJsonSchema, err = tool.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("convert to json schema fail: %w", err)
		}

		gTools[i] = funcDecl
	}

	return gTools, nil
}

func decodeBase64DataURL(dataURL string) ([]byte, error) {
	// Check if a web URL is passed by mistake.
	if strings.HasPrefix(dataURL, "http") {
		return nil, fmt.Errorf("invalid input: expected base64 data or data URL, but got a web URL starting with 'http'. Please fetch the content from the URL first")
	}
	// Find the comma that separates the prefix from the data
	commaIndex := strings.Index(dataURL, ",")
	if commaIndex == -1 {
		// If no comma, assume it's a raw base64 string and try to decode it directly.
		decoded, err := base64.StdEncoding.DecodeString(dataURL)
		if err != nil {
			return nil, fmt.Errorf("failed to decode raw base64 data: %w", err)
		}
		return decoded, nil
	}

	// Extract the base64 part of the data URL
	base64Data := dataURL[commaIndex+1:]

	// Decode the base64 string
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data from data URL: %w", err)
	}

	return decoded, nil
}

func createContentBlockFromType(t schema.ContentBlockType) *schema.ContentBlock {
	switch t {
	case schema.ContentBlockTypeReasoning:
		return &schema.ContentBlock{Type: t, Reasoning: &schema.Reasoning{}}
	case schema.ContentBlockTypeUserInputText:
		return &schema.ContentBlock{Type: t, UserInputText: &schema.UserInputText{}}
	case schema.ContentBlockTypeUserInputImage:
		return &schema.ContentBlock{Type: t, UserInputImage: &schema.UserInputImage{}}
	case schema.ContentBlockTypeUserInputAudio:
		return &schema.ContentBlock{Type: t, UserInputAudio: &schema.UserInputAudio{}}
	case schema.ContentBlockTypeUserInputVideo:
		return &schema.ContentBlock{Type: t, UserInputVideo: &schema.UserInputVideo{}}
	case schema.ContentBlockTypeUserInputFile:
		return &schema.ContentBlock{Type: t, UserInputFile: &schema.UserInputFile{}}
	case schema.ContentBlockTypeAssistantGenText:
		return &schema.ContentBlock{Type: t, AssistantGenText: &schema.AssistantGenText{}}
	case schema.ContentBlockTypeAssistantGenImage:
		return &schema.ContentBlock{Type: t, AssistantGenImage: &schema.AssistantGenImage{}}
	case schema.ContentBlockTypeAssistantGenAudio:
		return &schema.ContentBlock{Type: t, AssistantGenAudio: &schema.AssistantGenAudio{}}
	case schema.ContentBlockTypeAssistantGenVideo:
		return &schema.ContentBlock{Type: t, AssistantGenVideo: &schema.AssistantGenVideo{}}
	case schema.ContentBlockTypeFunctionToolCall:
		return &schema.ContentBlock{Type: t, FunctionToolCall: &schema.FunctionToolCall{}}
	case schema.ContentBlockTypeFunctionToolResult:
		return &schema.ContentBlock{Type: t, FunctionToolResult: &schema.FunctionToolResult{}}
	case schema.ContentBlockTypeServerToolCall:
		return &schema.ContentBlock{Type: t, ServerToolCall: &schema.ServerToolCall{}}
	case schema.ContentBlockTypeServerToolResult:
		return &schema.ContentBlock{Type: t, ServerToolResult: &schema.ServerToolResult{}}
	case schema.ContentBlockTypeMCPToolCall:
		return &schema.ContentBlock{Type: t, MCPToolCall: &schema.MCPToolCall{}}
	case schema.ContentBlockTypeMCPToolResult:
		return &schema.ContentBlock{Type: t, MCPToolResult: &schema.MCPToolResult{}}
	case schema.ContentBlockTypeMCPListToolsResult:
		return &schema.ContentBlock{Type: t, MCPListToolsResult: &schema.MCPListToolsResult{}}
	case schema.ContentBlockTypeMCPToolApprovalRequest:
		return &schema.ContentBlock{Type: t, MCPToolApprovalRequest: &schema.MCPToolApprovalRequest{}}
	case schema.ContentBlockTypeMCPToolApprovalResponse:
		return &schema.ContentBlock{Type: t, MCPToolApprovalResponse: &schema.MCPToolApprovalResponse{}}
	default:
		return &schema.ContentBlock{Type: t}
	}
}

// genInputAndConf generates input messages and configuration for Gemini API
func (g *Gemini) genInputAndConf(input []*schema.AgenticMessage, opts ...model.Option) (string, []*schema.AgenticMessage, *genai.GenerateContentConfig, *model.AgenticConfig, error) {
	commonOptions := model.GetCommonOptions(&model.Options{
		Temperature:       g.temperature,
		TopP:              g.topP,
		Tools:             nil,
		AgenticToolChoice: g.toolChoice,
		MaxTokens:         g.maxTokens,
	}, opts...)
	geminiOptions := model.GetImplSpecificOptions(&options{
		TopK:               g.topK,
		ResponseJSONSchema: g.responseJSONSchema,
		ResponseModalities: g.responseModalities,
	}, opts...)
	conf := &model.AgenticConfig{}

	m := &genai.GenerateContentConfig{
		SafetySettings: g.safetySettings,
	}
	if commonOptions.Model != nil {
		conf.Model = *commonOptions.Model
	} else {
		conf.Model = g.model
	}

	tools := g.tools
	if commonOptions.Tools != nil {
		var err error
		tools, err = toGeminiTools(commonOptions.Tools)
		if err != nil {
			return "", nil, nil, nil, err
		}
	}

	if len(tools) > 0 {
		t := &genai.Tool{
			FunctionDeclarations: make([]*genai.FunctionDeclaration, len(tools)),
		}
		copy(t.FunctionDeclarations, tools)
		m.Tools = append(m.Tools, t)
	}
	if g.enableCodeExecution != nil {
		m.Tools = append(m.Tools, &genai.Tool{
			CodeExecution: g.enableCodeExecution,
		})
	}
	if g.enableGoogleSearch != nil {
		m.Tools = append(m.Tools, &genai.Tool{
			GoogleSearch: g.enableGoogleSearch,
		})
	}
	if g.enableGoogleSearchRetrieval != nil {
		m.Tools = append(m.Tools, &genai.Tool{
			GoogleSearchRetrieval: g.enableGoogleSearchRetrieval,
		})
	}
	if g.enableComputerUse != nil {
		m.Tools = append(m.Tools, &genai.Tool{
			ComputerUse: g.enableComputerUse,
		})
	}
	if g.enableURLContext != nil {
		m.Tools = append(m.Tools, &genai.Tool{
			URLContext: g.enableURLContext,
		})
	}
	if g.enableFileSearch != nil {
		m.Tools = append(m.Tools, &genai.Tool{
			FileSearch: g.enableFileSearch,
		})
	}
	if g.enableGoogleMaps != nil {
		m.Tools = append(m.Tools, &genai.Tool{
			GoogleMaps: g.enableGoogleMaps,
		})
	}

	m.MediaResolution = g.mediaResolution

	if commonOptions.MaxTokens != nil {
		m.MaxOutputTokens = int32(*commonOptions.MaxTokens)
	}
	if commonOptions.TopP != nil {
		topP := *commonOptions.TopP
		conf.TopP = topP
		m.TopP = &topP
	}
	if commonOptions.Temperature != nil {
		temp := float32(*commonOptions.Temperature)
		conf.Temperature = *commonOptions.Temperature
		m.Temperature = &temp
	}
	if commonOptions.ToolChoice != nil {
		return "", nil, nil, nil, fmt.Errorf("agentic model unsupported tool choice")
	}
	if commonOptions.AgenticToolChoice != nil {
		switch commonOptions.AgenticToolChoice.Type {
		case schema.ToolChoiceForbidden:
			m.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeNone,
			}}
		case schema.ToolChoiceAllowed:
			var allowedTools []string
			if commonOptions.AgenticToolChoice.Allowed != nil {
				for _, at := range commonOptions.AgenticToolChoice.Allowed.Tools {
					if at.FunctionName != "" {
						allowedTools = append(allowedTools, at.FunctionName)
					}
					if at.ServerTool != nil {
						return "", nil, nil, nil, fmt.Errorf("unsupported allowed tools type: server tool")
					}
					if at.MCPTool != nil {
						return "", nil, nil, nil, fmt.Errorf("unsupported allowed tools type: mcp tool")
					}
				}
			}
			m.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode:                 genai.FunctionCallingConfigModeAuto,
				AllowedFunctionNames: allowedTools,
			}}
		case schema.ToolChoiceForced:
			// The predicted function call will be any one of the provided "functionDeclarations".
			if len(m.Tools) == 0 {
				return "", nil, nil, nil, fmt.Errorf("tool choice is forced but tool is not provided")
			}
			var allowedTools []string
			if commonOptions.AgenticToolChoice.Forced != nil {
				for _, at := range commonOptions.AgenticToolChoice.Forced.Tools {
					if at.FunctionName != "" {
						allowedTools = append(allowedTools, at.FunctionName)
					}
					if at.ServerTool != nil {
						return "", nil, nil, nil, fmt.Errorf("unsupported allowed tools type: server tool")
					}
					if at.MCPTool != nil {
						return "", nil, nil, nil, fmt.Errorf("unsupported allowed tools type: mcp tool")
					}
				}
			}
			m.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode:                 genai.FunctionCallingConfigModeAny,
				AllowedFunctionNames: allowedTools,
			}}

		default:
			return "", nil, nil, nil, fmt.Errorf("tool choice=%s not support", *commonOptions.ToolChoice)
		}
	}
	if geminiOptions.TopK != nil {
		topK := float32(*geminiOptions.TopK)
		m.TopK = &topK
	}

	if geminiOptions.ResponseJSONSchema != nil {
		m.ResponseMIMEType = "application/json"
		m.ResponseJsonSchema = geminiOptions.ResponseJSONSchema
	}

	if len(geminiOptions.ResponseModalities) > 0 {
		m.ResponseModalities = make([]string, len(geminiOptions.ResponseModalities))
		for i, v := range geminiOptions.ResponseModalities {
			m.ResponseModalities[i] = string(v)
		}
	}

	nInput := make([]*schema.AgenticMessage, len(input))
	copy(nInput, input)
	if len(input) > 1 && input[0].Role == schema.AgenticRoleTypeSystem {
		var err error
		m.SystemInstruction, err = convAgenticMessage(input[0])
		if err != nil {
			return "", nil, nil, nil, fmt.Errorf("failed to convert system instruction: %w", err)
		}
		nInput = input[1:]
	}

	m.ThinkingConfig = g.thinkingConfig
	if geminiOptions.ThinkingConfig != nil {
		m.ThinkingConfig = geminiOptions.ThinkingConfig
	}

	if len(geminiOptions.CachedContentName) > 0 {
		m.CachedContent = geminiOptions.CachedContentName
		// remove system instruction and tools when using cached content
		m.SystemInstruction = nil
		m.Tools = nil
		m.ToolConfig = nil
	}
	return conf.Model, nInput, m, conf, nil
}

func convCallbackOutput(message *schema.AgenticMessage, conf *model.AgenticConfig) *model.AgenticCallbackOutput {
	callbackOutput := &model.AgenticCallbackOutput{
		Message: message,
		Config:  conf,
	}
	return callbackOutput
}
