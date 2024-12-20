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

package model

type DebugGraph struct {
	DT []*DebugThread
}

func (d DebugGraph) GetDebugThread(threadID string) (thread DebugThread, exist bool) {
	for _, dt := range d.DT {
		if dt.ID == threadID {
			return *dt, true
		}
	}
	return thread, false
}

type DebugThread struct {
	// ID: unique id of each debug thread, from IDE Client.
	ID string
}

type NodeDebugState struct {
	// NodeKey: from graph compile callback.
	NodeKey string

	// Input: the input of the node, json marshal string.
	Input string
	// Output: the output of the node, json marshal string.
	Output string
	// Error: the error of the node, plain text.
	Error string
	// ErrorType: the type of error.
	ErrorType ErrorType

	Metrics NodeDebugMetrics
}

type NodeDebugMetrics struct {
	PromptTokens     int64
	CompletionTokens int64

	InvokeTimeMS     int64
	CompletionTimeMS int64
}

type DebugRunMeta struct {
	GraphID  string
	ThreadID string
	FromNode string
}

type ErrorType string

const (
	NodeError   ErrorType = "NodeError"
	SystemError ErrorType = "SystemError"
)
