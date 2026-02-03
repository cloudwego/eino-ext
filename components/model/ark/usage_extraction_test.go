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

package ark

import (
	"io"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"
)

// TestResponsesAPIChatModelReceivedStreamResponse_EventResponseWithUsage tests that Usage information
// is correctly extracted from Event_Response events in the stream.
// This verifies the fix for extracting token usage from intermediate stream events.
//
// Background: ARK Responses API may return token usage information in two places:
// 1. Event_Response: Intermediate stream events that may contain Usage data
// 2. Event_ResponseCompleted: Final event with Status and possibly FinishReason
//
// This test demonstrates:
// - Real ARK API returns Usage in Event_Response (intermediate stream events)
// - Event_ResponseCompleted may not have Usage data
// - The fix ensures Usage from Event_Response is extracted and not lost
func TestResponsesAPIChatModelReceivedStreamResponse_EventResponseWithUsage(t *testing.T) {
	PatchConvey("Extract Usage from Event_Response", t, func() {
		cm := &ResponsesAPIChatModel{}

		// Simulate ARK API returning Usage in Event_Response
		// This matches real-world ARK API behavior where token counts appear in stream events
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(
			// First event: Event_Response with Usage (intermediate stream event)
			&responses.Event{
				Event: &responses.Event_Response{
					Response: &responses.ResponseEvent{
						Response: &responses.ResponseObject{
							Usage: &responses.Usage{
								InputTokens:  49,
								OutputTokens: 144,
								TotalTokens:  193,
								InputTokensDetails: &responses.InputTokensDetails{
									CachedTokens: 10,
								},
								OutputTokensDetails: &responses.OutputTokensDetails{
									ReasoningTokens: 20,
								},
							},
						},
					},
				},
			}, nil).Then(
			// Second event: Event_ResponseCompleted without Usage
			&responses.Event{
				Event: &responses.Event_ResponseCompleted{
					ResponseCompleted: &responses.ResponseCompletedEvent{
						Response: &responses.ResponseObject{
							Status: responses.ResponseStatus_completed,
							Usage:  nil,
						},
					},
				},
			}, nil).Then(nil, io.EOF)).Build()

		streamReader := &utils.ResponsesStreamReader{}

		// Track calls to verify behavior
		callCount := 0
		var firstCallUsage *schema.TokenUsage

		Mock((*ResponsesAPIChatModel).sendCallbackOutput).To(func(
			sw *schema.StreamWriter[*model.CallbackOutput],
			reqConf *model.Config,
			modelName string,
			msg *schema.Message,
		) {
			callCount++
			if callCount == 1 {
				// Capture Usage from first call (Event_Response)
				if msg.ResponseMeta != nil {
					firstCallUsage = msg.ResponseMeta.Usage
				}
			}
		}).Build()

		// Execute the method being tested
		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil)

		// Verify results
		assert.Equal(t, 2, callCount, "sendCallbackOutput called for both Event_Response and Event_ResponseCompleted")
		assert.NotNil(t, firstCallUsage, "First call (Event_Response) should have Usage extracted")
		assert.Equal(t, 49, firstCallUsage.PromptTokens, "PromptTokens extracted from Event_Response")
		assert.Equal(t, 144, firstCallUsage.CompletionTokens, "CompletionTokens extracted from Event_Response")
		assert.Equal(t, 193, firstCallUsage.TotalTokens, "TotalTokens extracted from Event_Response")
		assert.Equal(t, 10, firstCallUsage.PromptTokenDetails.CachedTokens, "CachedTokens details preserved")
		assert.Equal(t, 20, firstCallUsage.CompletionTokensDetails.ReasoningTokens, "ReasoningTokens details preserved")
	})
}
