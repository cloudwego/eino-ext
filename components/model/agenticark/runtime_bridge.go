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

package agenticark

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	runtimev2 "github.com/volcengine/ark-runtime-go/arkruntime"
	responsesv2 "github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
	legacyresponses "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

// The legacy SDK only accepts string tool outputs. These private prefixes carry
// the new SDK's array form through the legacy request model without expanding
// agenticark's public API.
const (
	multimodalToolOutputPrefix = "\x00eino_ext_agenticark_multimodal_tool_output:"
	escapedToolOutputPrefix    = "\x00eino_ext_agenticark_escaped_tool_output:"
)

type responseStreamReader interface {
	Recv() (*legacyresponses.Event, error)
	Close() error
}

type runtimeBridge struct {
	client *runtimev2.Client
}

func (c *runtimeBridge) CreateResponses(ctx context.Context, req *legacyresponses.ResponsesRequest,
	headers map[string]string) (*legacyresponses.ResponseObject, error) {
	v2Req, err := toV2Request(req)
	if err != nil {
		return nil, err
	}

	res, err := c.client.CreateResponses(ctx, v2Req, runtimev2.WithCustomHeaders(headers))
	if err != nil {
		return nil, err
	}

	var legacy legacyresponses.ResponseObject
	if err := convertJSON(&res.Response, &legacy); err != nil {
		return nil, fmt.Errorf("convert responses response to legacy model: %w", err)
	}
	return &legacy, nil
}

func (c *runtimeBridge) CreateResponsesStream(ctx context.Context, req *legacyresponses.ResponsesRequest,
	headers map[string]string) (responseStreamReader, error) {
	v2Req, err := toV2Request(req)
	if err != nil {
		return nil, err
	}

	stream, err := c.client.CreateResponsesStream(ctx, v2Req, runtimev2.WithCustomHeaders(headers))
	if err != nil {
		return nil, err
	}
	return &streamBridge{stream: stream}, nil
}

type streamBridge struct {
	stream interface {
		Recv() (*responsesv2.ResponseStreamEvent, error)
		Close() error
	}
}

func (s *streamBridge) Recv() (*legacyresponses.Event, error) {
	event, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}

	var legacy legacyresponses.Event
	if err := convertJSON(event, &legacy); err != nil {
		return nil, fmt.Errorf("convert responses stream event to legacy model: %w", err)
	}
	return &legacy, nil
}

func (s *streamBridge) Close() error {
	return s.stream.Close()
}

func toV2Request(req *legacyresponses.ResponsesRequest) (*responsesv2.ResponsesRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("legacy responses request is nil")
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal legacy responses request: %w", err)
	}
	raw, err = rewriteMultimodalToolOutput(raw)
	if err != nil {
		return nil, fmt.Errorf("rewrite multimodal function tool output: %w", err)
	}

	var v2Req responsesv2.ResponsesRequest
	if err := json.Unmarshal(raw, &v2Req); err != nil {
		return nil, fmt.Errorf("unmarshal request into ark-runtime-go model: %w", err)
	}
	return &v2Req, nil
}

func rewriteMultimodalToolOutput(raw json.RawMessage) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		if output, ok := object["output"]; ok {
			var value string
			if err := json.Unmarshal(output, &value); err == nil {
				switch {
				case strings.HasPrefix(value, escapedToolOutputPrefix):
					unescaped, err := json.Marshal(strings.TrimPrefix(value, escapedToolOutputPrefix))
					if err != nil {
						return nil, err
					}
					object["output"] = unescaped
				case strings.HasPrefix(value, multimodalToolOutputPrefix):
					content := json.RawMessage(strings.TrimPrefix(value, multimodalToolOutputPrefix))
					var blocks []json.RawMessage
					if err := json.Unmarshal(content, &blocks); err != nil {
						return nil, fmt.Errorf("invalid multimodal tool output: %w", err)
					}
					object["output"] = content
				}
			}
		}
		for key, child := range object {
			if key == "output" {
				continue
			}
			rewritten, err := rewriteMultimodalToolOutput(child)
			if err != nil {
				return nil, err
			}
			object[key] = rewritten
		}
		return json.Marshal(object)
	}

	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		for i, child := range list {
			rewritten, err := rewriteMultimodalToolOutput(child)
			if err != nil {
				return nil, err
			}
			list[i] = rewritten
		}
		return json.Marshal(list)
	}
	return raw, nil
}

func convertJSON(src, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
