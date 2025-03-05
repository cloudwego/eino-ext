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
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestConvMessage(t *testing.T) {
	input := []mcp.PromptMessage{
		{
			Role: mcp.RoleUser,
			Content: mcp.TextContent{
				Type: "text",
				Text: "hello world",
			},
		},
		{
			Role: mcp.RoleUser,
			Content: mcp.ImageContent{
				Type:     "image",
				Data:     "test data",
				MIMEType: "image/jpeg",
			},
		},
		{
			Role: mcp.RoleUser,
			Content: mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      "test uri",
					MIMEType: "text/plain",
					Text:     "test text",
				},
			},
		},
		{
			Role: mcp.RoleUser,
			Content: mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      "test uri",
					MIMEType: "image/jpeg",
					Text:     "test text",
				},
			},
		},
		{
			Role: mcp.RoleUser,
			Content: mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      "test uri",
					MIMEType: "audio/mpeg",
					Text:     "test text",
				},
			},
		},
		{
			Role: mcp.RoleUser,
			Content: mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      "test uri",
					MIMEType: "video/mpeg",
					Text:     "test text",
				},
			},
		},
	}
	expected := []*schema.Message{
		schema.UserMessage("hello world"),
		{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					ImageURL: &schema.ChatMessageImageURL{
						URL:      "test data",
						MIMEType: "image/jpeg",
					},
				},
			},
		},
		{
			Role:    schema.User,
			Content: "test uri",
		},
		{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					ImageURL: &schema.ChatMessageImageURL{
						URL:      "test uri",
						MIMEType: "image/jpeg",
					},
				},
			},
		},
		{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeAudioURL,
					AudioURL: &schema.ChatMessageAudioURL{
						URL:      "test uri",
						MIMEType: "audio/mpeg",
					},
				},
			},
		},
		{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeVideoURL,
					VideoURL: &schema.ChatMessageVideoURL{
						URL:      "test uri",
						MIMEType: "video/mpeg",
					},
				},
			},
		},
	}

	var output []*schema.Message
	for _, msg := range input {
		m, err := convMessage(msg)
		assert.NoError(t, err)
		output = append(output, m)
	}
	assert.Equal(t, expected, output)
}

func TestFormat(t *testing.T) {
	ctx := context.Background()
	tpl, err := NewPromptTemplate(ctx, &Config{
		Cli: &mockMCPClient{},
	})
	assert.NoError(t, err)
	result, err := tpl.Format(ctx, map[string]interface{}{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, "hello world", result[0].Content)
}

type mockMCPClient struct{}

func (m *mockMCPClient) Initialize(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) Ping(ctx context.Context) error {
	panic("implement me")
}

func (m *mockMCPClient) ListResources(ctx context.Context, request mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) ListResourceTemplates(ctx context.Context, request mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) ReadResource(ctx context.Context, request mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) Subscribe(ctx context.Context, request mcp.SubscribeRequest) error {
	panic("implement me")
}

func (m *mockMCPClient) Unsubscribe(ctx context.Context, request mcp.UnsubscribeRequest) error {
	panic("implement me")
}

func (m *mockMCPClient) ListPrompts(ctx context.Context, request mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) GetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: "hello world",
				},
			},
		},
	}, nil
}

func (m *mockMCPClient) ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) SetLevel(ctx context.Context, request mcp.SetLevelRequest) error {
	panic("implement me")
}

func (m *mockMCPClient) Complete(ctx context.Context, request mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	panic("implement me")
}

func (m *mockMCPClient) Close() error {
	panic("implement me")
}

func (m *mockMCPClient) OnNotification(handler func(notification mcp.JSONRPCNotification)) {
	panic("implement me")
}
