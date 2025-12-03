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

package openrouter

import (
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	openrouterTerminatedErrorKey  = "openrouter_terminated_error"
	openrouterReasoningDetailsKey = "openrouter_reasoning_details"
)

func init() {
	compose.RegisterStreamChunkConcatFunc(func(chunks [][]*reasoningDetails) (final []*reasoningDetails, err error) {
		if len(chunks) == 0 {
			return []*reasoningDetails{}, nil
		}
		for _, details := range chunks {
			final = append(final, details...)
		}
		return final, nil
	})
	schema.RegisterName[*reasoningDetails]("_eino_ext_openrouter_reasoning_details")

	compose.RegisterStreamChunkConcatFunc(func(chunks []*StreamTerminatedError) (final *StreamTerminatedError, err error) {
		if len(chunks) == 0 {
			return &StreamTerminatedError{}, nil
		}
		return chunks[len(chunks)-1], nil
	})

	schema.RegisterName[*StreamTerminatedError]("_eino_ext_openrouter_stream_terminated_error")

}

type StreamTerminatedError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func setStreamTerminatedError(message *schema.Message, terminatedError string) (err error) {
	if message.Extra == nil {
		message.Extra = map[string]any{}
	}
	e := &StreamTerminatedError{}
	err = sonic.UnmarshalString(terminatedError, e)
	if err != nil {
		return
	}
	message.Extra[openrouterTerminatedErrorKey] = e
	return nil
}
func GetStreamTerminatedError(message *schema.Message) (*StreamTerminatedError, bool) {
	if message.Extra == nil {
		return nil, false
	}
	e, ok := message.Extra[openrouterTerminatedErrorKey].(*StreamTerminatedError)
	return e, ok
}

func setReasoningDetails(msg *schema.Message, reasoningDetails []*reasoningDetails) {
	if msg.Extra == nil {
		msg.Extra = map[string]any{}
	}
	msg.Extra[openrouterReasoningDetailsKey] = reasoningDetails
}
func getReasoningDetails(msg *schema.Message) (details []*reasoningDetails, b bool) {
	if msg.Extra == nil {
		return nil, false
	}
	details, b = msg.Extra[openrouterReasoningDetailsKey].([]*reasoningDetails)
	return

}
