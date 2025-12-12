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
	openrouterCacheControlKey     = "openrouter_cache_control_key"
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

	compose.RegisterStreamChunkConcatFunc(func(chunks []*cacheControl) (final *cacheControl, err error) {
		for _, chunk := range chunks {
			if chunk != nil && chunk.TTL == CacheControlTTL1Hour {
				return chunk, nil
			}
		}
		return chunks[len(chunks)-1], nil
	})

	schema.RegisterName[*cacheControl]("_eino_ext_openrouter_cache_control")
}

// StreamTerminatedError represents an error that occurs when the stream is terminated unexpectedly.
// It contains a code and a message providing details about the error.
// This is particularly useful for handling stream interruptions in a structured way.
type StreamTerminatedError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func setStreamTerminatedError(message *schema.Message, terminatedError string) (err error) {
	if message == nil {
		return nil
	}
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

// GetStreamTerminatedError retrieves the StreamTerminatedError from the message's extra data.
// It returns the error and a boolean indicating whether the error was found.
// This function is useful for checking if a stream was terminated due to an error and handling it gracefully.
func GetStreamTerminatedError(message *schema.Message) (*StreamTerminatedError, bool) {
	if message == nil {
		return nil, false
	}
	if message.Extra == nil {
		return nil, false
	}
	e, ok := message.Extra[openrouterTerminatedErrorKey].(*StreamTerminatedError)
	return e, ok
}

func setReasoningDetails(msg *schema.Message, reasoningDetails []*reasoningDetails) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = map[string]any{}
	}
	msg.Extra[openrouterReasoningDetailsKey] = reasoningDetails
}
func getReasoningDetails(msg *schema.Message) (details []*reasoningDetails, b bool) {
	if msg == nil {
		return nil, false
	}
	if msg.Extra == nil {
		return nil, false
	}
	details, b = msg.Extra[openrouterReasoningDetailsKey].([]*reasoningDetails)
	return

}

type CacheControlTTL string

const (
	cacheControlEphemeralType                 = "ephemeral"
	CacheControlTTL5Minutes   CacheControlTTL = "5m"
	CacheControlTTL1Hour      CacheControlTTL = "1h"
)

func WithCacheControlTTL(ttl CacheControlTTL) CacheControlOption {
	return func(control *cacheControl) {
		control.TTL = ttl
	}
}

type CacheControlOption func(control *cacheControl)

type cacheControl struct {
	Type string          `json:"type,omitempty"`
	TTL  CacheControlTTL `json:"ttl,omitempty"`
}

// EnableMessageInputPartCacheControl enables cache control for a specific part of a user's message input.
// This allows for fine-grained control over how individual parts of a message are cached.
// Note: This currently only applies to text parts (schema.ChatMessagePartTypeText).
// By default, it sets the cache type to ephemeral with a 5-minute TTL.
// Use WithCacheControlTTL to customize the time-to-live for the cache.
func EnableMessageInputPartCacheControl(part *schema.MessageInputPart, opts ...CacheControlOption) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = map[string]any{}
	}
	ctrl := &cacheControl{Type: cacheControlEphemeralType, TTL: CacheControlTTL5Minutes}
	for _, opt := range opts {
		opt(ctrl)
	}
	part.Extra[openrouterCacheControlKey] = ctrl
}

// EnableMessageOutputPartCacheControl enables cache control for a specific part of an assistant's message output.
// This allows for fine-grained control over how individual parts of a message are cached.
// Note: This currently only applies to text parts (schema.ChatMessagePartTypeText).
// By default, it sets the cache type to ephemeral with a 5-minute TTL.
// Use WithCacheControlTTL to customize the time-to-live for the cache.
func EnableMessageOutputPartCacheControl(part *schema.MessageOutputPart, opts ...CacheControlOption) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = map[string]any{}
	}
	ctrl := &cacheControl{Type: cacheControlEphemeralType, TTL: CacheControlTTL5Minutes}
	for _, opt := range opts {
		opt(ctrl)
	}
	part.Extra[openrouterCacheControlKey] = ctrl
}

// EnableMessageContentCacheControl enables cache control for an entire message.
// This is useful for caching the entire content of a message, such as a user's query or an assistant's response.
// By default, it sets the cache type to ephemeral with a 5-minute TTL.
// Use WithCacheControlTTL to customize the time-to-live for the cache.
func EnableMessageContentCacheControl(part *schema.Message, opts ...CacheControlOption) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = map[string]any{}
	}
	ctrl := &cacheControl{Type: cacheControlEphemeralType, TTL: CacheControlTTL5Minutes}
	for _, opt := range opts {
		opt(ctrl)
	}
	part.Extra[openrouterCacheControlKey] = ctrl
}

func getMessageInputPartCacheControl(part *schema.MessageInputPart) (*cacheControl, bool) {
	if part == nil {
		return nil, false
	}
	if part.Extra == nil {
		return nil, false
	}
	ctrl, ok := part.Extra[openrouterCacheControlKey].(*cacheControl)
	return ctrl, ok
}

func getMessageOutputPartCacheControl(part *schema.MessageOutputPart) (*cacheControl, bool) {
	if part == nil {
		return nil, false
	}
	if part.Extra == nil {
		return nil, false
	}
	ctrl, ok := part.Extra[openrouterCacheControlKey].(*cacheControl)
	return ctrl, ok
}

func getMessageContentCacheControl(msg *schema.Message) (*cacheControl, bool) {
	if msg == nil {
		return nil, false
	}
	if msg.Extra == nil {
		msg.Extra = map[string]any{}
	}
	ctrl, ok := msg.Extra[openrouterCacheControlKey].(*cacheControl)
	return ctrl, ok

}
