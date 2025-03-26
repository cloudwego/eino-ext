/*
 * Copyright 2024 CloudWeGo Authors
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

package types

import (
	"encoding/json"

	"github.com/cloudwego/eino-ext/devops/internal/model"
	devmodel "github.com/cloudwego/eino-ext/devops/model"
)

type GetCanvasInfoResponse struct {
	CanvasInfo devmodel.CanvasInfo `json:"canvas_info,omitempty"`
}

type CreateDebugThreadResponse struct {
	ThreadID string `json:"thread_id,omitempty"`
}

type DebugRunRequest struct {
	FromNode string `json:"from_node"`
	Input    string `json:"input"` // mock input data after json marshal
	LogID    string `json:"log_id"`
}

type DebugRunEventType string

const (
	debugRunEventOfData   DebugRunEventType = "data"
	debugRunEventOfFinish DebugRunEventType = "finish"
	debugRunEventOfError  DebugRunEventType = "error"
)

type DebugRunEventMsg struct {
	Type    DebugRunEventType `json:"type"`
	DebugID string            `json:"debug_id"`
	Error   string            `json:"error,omitempty"`
	Content *NodeDebugState   `json:"content,omitempty"`
}

type NodeDebugState struct {
	NodeKey string `json:"node_key,omitempty"`

	Input     string `json:"input,omitempty"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"error_type,omitempty"`

	Metrics NodeDebugMetrics `json:"metrics,omitempty"`
}

type NodeDebugMetrics struct {
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`

	InvokeTimeMS     int64 `json:"invoke_time_ms,omitempty"`
	CompletionTimeMS int64 `json:"completion_time_ms,omitempty"`
}

func DebugRunDataEVT(debugID string, state *model.NodeDebugState) (s DebugRunEventMsg) {
	return DebugRunEventMsg{
		Type:    debugRunEventOfData,
		DebugID: debugID,
		Content: &NodeDebugState{
			NodeKey:   state.NodeKey,
			Input:     state.Input,
			Output:    state.Output,
			Error:     state.Error,
			ErrorType: string(state.ErrorType),
			Metrics: NodeDebugMetrics{
				PromptTokens:     state.Metrics.PromptTokens,
				CompletionTokens: state.Metrics.CompletionTokens,
				InvokeTimeMS:     state.Metrics.InvokeTimeMS,
				CompletionTimeMS: state.Metrics.CompletionTimeMS,
			},
		},
	}
}

func DebugRunErrEVT(debugID string, errStr string) (s DebugRunEventMsg) {
	return DebugRunEventMsg{
		Type:    debugRunEventOfError,
		DebugID: debugID,
		Error:   errStr,
	}
}

func DebugRunFinishEVT(debugID string) (s DebugRunEventMsg) {
	return DebugRunEventMsg{
		Type:    debugRunEventOfFinish,
		DebugID: debugID,
	}
}

func (d DebugRunEventMsg) JsonBytes() []byte {
	bytes, _ := json.Marshal(d)
	return bytes
}

type ListInputTypesResponse struct {
	Types []*devmodel.JsonSchema `json:"types,omitempty"`
}
