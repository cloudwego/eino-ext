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

package cozeloop

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go"
	"github.com/coze-dev/cozeloop-go/entity"
	"github.com/vincent-petithory/dataurl"
)

type Config struct {
	Key            string
	Version        string
	CozeLoopClient cozeloop.Client
}

func NewPromptHub(_ context.Context, conf *Config) (prompt.ChatTemplate, error) {
	if conf == nil {
		return nil, fmt.Errorf("new prompt hub fail because conf is empty")
	}
	if conf.CozeLoopClient == nil {
		return nil, fmt.Errorf("new prompt hub fail because cozeloop client in conf is empty")
	}
	if conf.Key == "" {
		return nil, fmt.Errorf("new prompt hub fail because key in conf is empty")
	}
	return &promptHub{
		cli:     conf.CozeLoopClient,
		key:     conf.Key,
		version: conf.Version,
	}, nil
}

type promptHub struct {
	cli     cozeloop.Client
	key     string
	version string
}

func (p *promptHub) Format(ctx context.Context, vs map[string]any, opts ...prompt.Option) (result []*schema.Message, err error) {
	var loopPrompt *entity.Prompt
	var formattedMessages []*entity.Message

	extMap := map[string]any{
		"prompt_key":      p.key,
		"prompt_provider": "cozeloop",
	}
	if p.version != "" {
		extMap["prompt_version"] = p.version
	}

	ctx = callbacks.OnStart(ctx, &prompt.CallbackInput{
		Variables: vs,
		Extra:     extMap,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		} else {
			callbacks.OnEnd(ctx, &prompt.CallbackOutput{
				Result: result,
				Extra:  extMap,
			})
		}
	}()

	// Get prompt from CozeLoop
	param := cozeloop.GetPromptParam{
		PromptKey: p.key,
	}
	if p.version != "" {
		param.Version = p.version
	}
	loopPrompt, err = p.cli.GetPrompt(ctx, param)
	if err != nil {
		return nil, fmt.Errorf("get prompt from prompt service fail: %w", err)
	}

	if loopPrompt == nil {
		return nil, fmt.Errorf("prompt is empty")
	}

	// Format prompt with variables using cozeloop's PromptFormat
	formattedMessages, err = p.cli.PromptFormat(ctx, loopPrompt, vs)
	if err != nil {
		return nil, fmt.Errorf("format prompt fail: %w", err)
	}

	// Convert CozeLoop messages to eino schema messages
	result = make([]*schema.Message, 0, len(formattedMessages))
	for _, msg := range formattedMessages {
		if msg == nil {
			continue
		}
		m, err := messageConv(msg)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}

	return result, nil
}

func (p *promptHub) GetType() string {
	return "PromptHub"
}

func (p *promptHub) IsCallbacksEnabled() bool {
	return true
}

func messageConv(orig *entity.Message) (*schema.Message, error) {

	var err error
	ret := &schema.Message{}

	ret.Role, err = roleConv(orig.Role)
	if err != nil {
		return nil, err
	}

	if orig.Content != nil {
		ret.Content = *orig.Content
	}

	// Convert multi-content parts based on message role
	if len(orig.Parts) > 0 {
		switch ret.Role {
		case schema.User:
			ret.UserInputMultiContent, err = inputPartsConv(orig.Parts)
			if err != nil {
				return nil, err
			}
		case schema.Assistant:
			ret.AssistantGenMultiContent, err = outputPartsConv(orig.Parts)
			if err != nil {
				return nil, err
			}
		}
	}

	return ret, nil
}

func roleConv(r entity.Role) (schema.RoleType, error) {
	switch r {
	case entity.RoleSystem:
		return schema.System, nil
	case entity.RoleUser:
		return schema.User, nil
	case entity.RoleAssistant:
		return schema.Assistant, nil
	case entity.RoleTool:
		return schema.Tool, nil
	default:
		return "", fmt.Errorf("unknown role type from cozeloop: %v", r)
	}
}

func inputPartsConv(parts []*entity.ContentPart) ([]schema.MessageInputPart, error) {
	ret := make([]schema.MessageInputPart, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}

		partType, err := contentTypeConv(part.Type)
		if err != nil {
			return nil, err
		}

		inputPart := schema.MessageInputPart{
			Type: partType,
		}

		switch part.Type {
		case entity.ContentTypeText:
			if part.Text != nil {
				inputPart.Text = *part.Text
			}
		case entity.ContentTypeImageURL:
			if part.ImageURL != nil {
				inputPart.Image = &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						URL: part.ImageURL,
					},
				}
			}
		case entity.ContentTypeBase64Data:
			if part.Base64Data != nil {
				mimeType, base64Data, err := parseBase64DataURL(*part.Base64Data)
				if err != nil {
					return nil, fmt.Errorf("failed to parse base64 data URL: %w", err)
				}
				inputPart.Image = &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						Base64Data: &base64Data,
						MIMEType:   mimeType,
					},
				}
			}
		}

		ret = append(ret, inputPart)
	}
	return ret, nil
}

func outputPartsConv(parts []*entity.ContentPart) ([]schema.MessageOutputPart, error) {
	ret := make([]schema.MessageOutputPart, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}

		partType, err := contentTypeConv(part.Type)
		if err != nil {
			return nil, err
		}

		outputPart := schema.MessageOutputPart{
			Type: partType,
		}

		switch part.Type {
		case entity.ContentTypeText:
			if part.Text != nil {
				outputPart.Text = *part.Text
			}
		case entity.ContentTypeImageURL:
			if part.ImageURL != nil {
				outputPart.Image = &schema.MessageOutputImage{
					MessagePartCommon: schema.MessagePartCommon{
						URL: part.ImageURL,
					},
				}
			}
		case entity.ContentTypeBase64Data:
			if part.Base64Data != nil {
				mimeType, base64Data, err := parseBase64DataURL(*part.Base64Data)
				if err != nil {
					return nil, fmt.Errorf("failed to parse base64 data URL: %w", err)
				}
				outputPart.Image = &schema.MessageOutputImage{
					MessagePartCommon: schema.MessagePartCommon{
						Base64Data: &base64Data,
						MIMEType:   mimeType,
					},
				}
			}
		}

		ret = append(ret, outputPart)
	}
	return ret, nil
}

func contentTypeConv(t entity.ContentType) (schema.ChatMessagePartType, error) {
	switch t {
	case entity.ContentTypeText:
		return schema.ChatMessagePartTypeText, nil
	case entity.ContentTypeImageURL, entity.ContentTypeBase64Data:
		return schema.ChatMessagePartTypeImageURL, nil
	default:
		return "", fmt.Errorf("unknown chat message part type from cozeloop: %v", t)
	}
}

// parseBase64DataURL parses a data URL (e.g., "data:image/png;base64,iVBORw0KGgo...")
// and returns the MIME type and the base64 data separately
func parseBase64DataURL(dataURLString string) (mimeType string, base64Data string, err error) {
	dataURL, err := dataurl.DecodeString(dataURLString)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode data URL: %w", err)
	}

	// Extract base64 data from original string (after comma)
	parts := strings.SplitN(dataURLString, ",", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid data URL format")
	}

	return dataURL.ContentType(), parts[1], nil
}
