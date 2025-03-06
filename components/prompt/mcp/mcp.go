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

func NewPromptTemplate(_ context.Context, conf *Config) (prompt.ChatTemplate, error) {
	return &chatTemplate{
		cli:  conf.Cli,
		name: conf.Name,
	}, nil
}

type chatTemplate struct {
	cli  client.MCPClient
	name string
}

func (c *chatTemplate) Format(ctx context.Context, vs map[string]any, _ ...prompt.Option) ([]*schema.Message, error) {
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
	switch m := message.Content.(type) {
	case mcp.TextContent:
		ret.Content = m.Text
	case mcp.ImageContent:
		ret.MultiContent = append(ret.MultiContent, schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeImageURL,
			ImageURL: &schema.ChatMessageImageURL{
				URL:      m.Data,
				MIMEType: m.MIMEType,
			},
		})
	case mcp.EmbeddedResource:
		var mimeType, uri string
		switch resource := m.Resource.(type) {
		case mcp.BlobResourceContents:
			uri = resource.URI
			mimeType = resource.MIMEType
		case mcp.TextResourceContents:
			uri = resource.URI
			mimeType = resource.MIMEType
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
			return nil, fmt.Errorf("unsupported mcp resource mime type %v", mimeType)
		}
	default:
		return nil, fmt.Errorf("unknown mcp prompt content type: %T", message.Content)
	}

	return ret, nil
}
