/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/agenticark"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	am, err := agenticark.New(ctx, &agenticark.Config{
		Model:  os.Getenv("ARK_MODEL_ID"),
		APIKey: os.Getenv("ARK_API_KEY"),
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err=%v", err)
	}

	mcpTools := []*responses.ToolMcp{
		{
			Type:        responses.ToolType_mcp,
			ServerLabel: "test_mcp_server",
			RequireApproval: &responses.McpRequireApproval{
				Union: &responses.McpRequireApproval_Mode{
					Mode: responses.ApprovalMode_never,
				},
			},
			ServerUrl: "server url",
			Headers: map[string]string{
				"X-API-KEY": "x-api-key if needed",
			},
		},
	}

	allowedTools := []*schema.AllowedTool{
		{
			MCPTool: &schema.AllowedMCPTool{
				ServerLabel: "test_mcp_server",
				Name:        "amap/maps_weather",
			},
		},
	}

	opts := []model.Option{
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{
			Type: schema.ToolChoiceForced,
			Forced: &schema.AgenticForcedToolChoice{
				Tools: allowedTools,
			},
		}),
		agenticark.WithMCPTools(mcpTools),
		agenticark.WithThinking(&responses.ResponsesThinking{
			Type: responses.ThinkingType_disabled.Enum(),
		}),
		agenticark.WithCustomHeaders(map[string]string{
			"ark-beta-mcp": "true",
		}),
	}

	input := []*schema.AgenticMessage{
		schema.UserAgenticMessage("what's the weather like in Beijing today"),
	}

	resp, err := am.Stream(ctx, input, opts...)
	if err != nil {
		log.Fatalf("failed to stream, err: %v", err)
	}

	var msgs []*schema.AgenticMessage
	for {
		msg, err := resp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", err)
		}
		msgs = append(msgs, msg)
	}

	concatenated, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	meta := concatenated.ResponseMeta.Extension.(*agenticark.ResponseMetaExtension)

	log.Printf("request_id: %s\n", meta.ID)
	respBody, _ := sonic.MarshalIndent(concatenated, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
