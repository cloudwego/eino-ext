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
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"
)

// TestResponsesAPIChatModelReceivedStreamResponse_EventResponseWithUsage tests that Usage information
// is correctly extracted from Event_Response events in the stream.
// This verifies the fix for extracting token usage from intermediate stream events.
func TestResponsesAPIChatModelReceivedStreamResponse_EventResponseWithUsage(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	PatchConvey("Event_Response with Usage information", t, func() {
		// Simulate ARK API returning Usage in Event_Response
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(
			// Event_Response with Usage information
			&responses.Event{
				Event: &responses.Event_Response{
					Response: &responses.ResponseEvent{
						Response: &responses.ResponseObject{
							Usage: &responses.Usage{
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
			// Event_ResponseCompleted without Usage
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
		mocker := Mock((*ResponsesAPIChatModel).sendCallbackOutput).Return().Build()

		// Call receivedStreamResponse which should extract Usage from Event_Response
		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil)

		// Verify sendCallbackOutput was called twice:
		// 1. For Event_Response with Usage information
		// 2. For Event_ResponseCompleted with FinishReason
		assert.Equal(t, 2, mocker.Times())
	})
}
