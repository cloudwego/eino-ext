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

package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type Config struct {
	// Cli is the MCP (Model Control Protocol) client, ref: https://github.com/mark3labs/mcp-go
	// Notice: should Initialize with server before use
	// Required
	Cli client.MCPClient
	// Name specifies the prompt name to use from MCP service
	// Required
	Name string
}

func NewPromptTemplate(ctx context.Context, conf *Config) (prompt.ChatTemplate, error) {
	return &chatTemplate{
		cli:  conf.Cli,
		name: conf.Name,
	}, nil
}

type chatTemplate struct {
	cli  client.MCPClient
	name string
}

func (c *chatTemplate) Format(ctx context.Context, vs map[string]any, opts ...prompt.Option) ([]*schema.Message, error) {
	arg := make(map[string]string, len(vs))
	for k, v := range vs {
		arg[k] = fmt.Sprint(v)
	}

	result, err := c.cli.GetPrompt(ctx, mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name:      c.name,
			Arguments: arg,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get mcp prompt fail: %w", err)
	}

	messages := make([]*schema.Message, 0, len(result.Messages))
	for _, message := range result.Messages {
		m, convErr := convMessage(message)
		if convErr != nil {
			return nil, fmt.Errorf("convert mcp message fail: %w", convErr)
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// GetType returns the type of the chat template (Default).
func (c *chatTemplate) GetType() string {
	return "MCP"
}

func convRole(role mcp.Role) (schema.RoleType, error) {
	switch role {
	case mcp.RoleUser:
		return schema.User, nil
	case mcp.RoleAssistant:
		return schema.Assistant, nil
	default:
		return "", fmt.Errorf("unknown mcp role %v", role)
	}
}

func convMessage(message mcp.PromptMessage) (*schema.Message, error) {
	ret := &schema.Message{}
	var err error
	ret.Role, err = convRole(message.Role)
	if err != nil {
		return nil, err
	}
	// the type of message content is map[string]any, but not TextContent, ImageContent or ResourceContent
	//switch m := message.Content.(type) {
	//case mcp.TextContent:
	//	ret.Content = m.Text
	//case mcp.ImageContent:
	//	ret.MultiContent = append(ret.MultiContent, schema.ChatMessagePart{
	//		Type: schema.ChatMessagePartTypeImageURL,
	//		ImageURL: &schema.ChatMessageImageURL{
	//			URL:      m.Data,
	//			MIMEType: m.MIMEType,
	//		},
	//	})
	//case mcp.EmbeddedResource:
	//	if strings.HasPrefix(m.Resource.MIMEType, "audio") {
	//		ret.MultiContent = append(ret.MultiContent, schema.ChatMessagePart{
	//			Type: schema.ChatMessagePartTypeAudioURL,
	//			AudioURL: &schema.ChatMessageAudioURL{
	//				URL:      m.Resource.URI,
	//				MIMEType: m.Resource.MIMEType,
	//			},
	//		})
	//	} else if strings.HasPrefix(m.Resource.MIMEType, "image") {
	//		ret.MultiContent = append(ret.MultiContent, schema.ChatMessagePart{
	//			Type: schema.ChatMessagePartTypeImageURL,
	//			ImageURL: &schema.ChatMessageImageURL{
	//				URL:      m.Resource.URI,
	//				MIMEType: m.Resource.MIMEType,
	//			},
	//		})
	//	} else if strings.HasPrefix(m.Resource.MIMEType, "video") {
	//		ret.MultiContent = append(ret.MultiContent, schema.ChatMessagePart{
	//			Type: schema.ChatMessagePartTypeVideoURL,
	//			VideoURL: &schema.ChatMessageVideoURL{
	//				URL:      m.Resource.URI,
	//				MIMEType: m.Resource.MIMEType,
	//			},
	//		})
	//	} else if strings.HasPrefix(m.Resource.MIMEType, "text") {
	//		ret.Content = m.Resource.URI
	//	} else {
	//		return nil, fmt.Errorf("support mcp resource mime type %v", m.Resource.MIMEType)
	//	}
	//default:
	//	return nil, fmt.Errorf("unknown mcp prompt content type: %T", message.Content)
	//}
	content := message.Content.(map[string]interface{})
	if t, ok := content["type"].(string); ok {
		if t == "text" {
			if text, okk := content["text"].(string); okk {
				ret.Content = text
			} else {
				return nil, fmt.Errorf("mcp prompt content type is text, but doesn't contain text, content: %v", content)
			}
		} else if t == "image" {
			var okk bool
			var data string
			var mimeType string
			if data, okk = content["data"].(string); !okk {
				return nil, fmt.Errorf("mcp prompt content type is image, but doesn't contain data, content: %v", content)
			}
			if mimeType, okk = content["mimeType"].(string); !okk {
				return nil, fmt.Errorf("mcp prompt content type is image, but doesn't contain mimeType, content: %v", content)
			}
			ret.MultiContent = append(ret.MultiContent, schema.ChatMessagePart{
				Type: schema.ChatMessagePartTypeImageURL,
				ImageURL: &schema.ChatMessageImageURL{
					URL:      data,
					MIMEType: mimeType,
				},
			})
		} else if t == "resource" {
			if resource, okk := content["resource"].(map[string]any); okk {
				var uri string
				var mimeType string
				if uri, okk = resource["uri"].(string); !okk {
					return nil, fmt.Errorf("mcp prompt content type is resource, but doesn't contain uri, content: %v", content)
				}
				if mimeType, okk = resource["mimeType"].(string); !okk {
					return nil, fmt.Errorf("mcp prompt content type is resource, but doesn't contain mimeType, content: %v", content)
				}
				if strings.HasPrefix(mimeType, "audio") {
					ret.MultiContent = append(ret.MultiContent, schema.ChatMessagePart{
						Type: schema.ChatMessagePartTypeAudioURL,
						AudioURL: &schema.ChatMessageAudioURL{
							URL:      uri,
							MIMEType: mimeType,
						},
					})
				} else if strings.HasPrefix(mimeType, "image") {
					ret.MultiContent = append(ret.MultiContent, schema.ChatMessagePart{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL:      uri,
							MIMEType: mimeType,
						},
					})
				} else if strings.HasPrefix(mimeType, "video") {
					ret.MultiContent = append(ret.MultiContent, schema.ChatMessagePart{
						Type: schema.ChatMessagePartTypeVideoURL,
						VideoURL: &schema.ChatMessageVideoURL{
							URL:      uri,
							MIMEType: mimeType,
						},
					})
				} else if strings.HasPrefix(mimeType, "text") {
					ret.Content = uri
				} else {
					return nil, fmt.Errorf("support mcp resource mime type %v", mimeType)
				}
			} else {
				return nil, fmt.Errorf("mcp prompt content type is resource, but doesn't contain resource, content: %v", content)
			}
		} else {
			return nil, fmt.Errorf("unknown mcp content type %s", t)
		}
	} else {
		return nil, fmt.Errorf("mcp content type is missing, content: %v", content)
	}
	return ret, nil
}
