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
	"context"
	"io"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	"github.com/smartystreets/goconvey/convey"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"

	"github.com/cloudwego/eino-ext/components/model/ark/utils"
)

// TestResponsesAPIChatModel_UsageExtractionFromEventResponse tests that Usage information
// is correctly extracted from Event_Response events, not just Event_ResponseCompleted.
// This is important because ARK Responses API may include token usage in stream events.
func TestResponsesAPIChatModel_UsageExtractionFromEventResponse(t *testing.T) {
	convey.Convey("Usage should be extracted from Event_Response", t, func() {
		defer mockey.Mock((*utils.ResponsesStreamReader).Recv).To(func(streamReader *utils.ResponsesStreamReader) (*responses.Event, error) {
			return nil, nil
		}).Build().UnPatch()

		ctx := context.Background()
		model := genModel(ctx, arkSDKResponseChatConfigForTest, &ARKChatModelConfig{})

		streamReader := &utils.ResponsesStreamReader{}

		mock := mockey.Mock((*utils.ResponsesStreamReader).Recv).To(func(streamReader *utils.ResponsesStreamReader) (*responses.Event, error) {
			return nil, nil
		}).Build()

		// Simulate ARK API behavior where Usage appears in Event_Response
		mock.MockChain(mockey.GetMethod(streamReader, "Recv")).Return(
			// Event_Response with Usage information
			&responses.Event{
				Event: &responses.Event_Response{
					Response: &responses.ResponseEvent{
						Response: &responses.ResponseObject{
							Usage: &responses.Usage{
								PromptTokens:     49,
								CompletionTokens: 130,
								TotalTokens:      179,
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
			// Event_ResponseCompleted with FinishReason but no Usage
			&responses.Event{
				Event: &responses.Event_ResponseCompleted{
					ResponseCompleted: &responses.ResponseCompletedEvent{
						Response: &responses.ResponseObject{
							Status: responses.ResponseObject_completed,
							Usage:  nil, // Usage may be nil in Event_ResponseCompleted
						},
					},
				},
			}, nil).Then(nil, io.EOF).Build()

		streamer, err := model.Stream(ctx, []*schema.Message{
			schema.UserMessage("test"),
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(streamer, convey.ShouldNotBeNil)

		// Collect all chunks
		var chunks []*schema.Message
		for {
			chunk, err := streamer.Recv()
			if err != nil {
				break
			}
			if chunk != nil {
				chunks = append(chunks, chunk)
			}
		}

		// Verify that at least one chunk contains Usage information
		hasUsage := false
		for _, chunk := range chunks {
			if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
				hasUsage = true
				// Verify the usage values
				convey.So(chunk.ResponseMeta.Usage.PromptTokens, convey.ShouldEqual, 49)
				convey.So(chunk.ResponseMeta.Usage.CompletionTokens, convey.ShouldEqual, 130)
				convey.So(chunk.ResponseMeta.Usage.TotalTokens, convey.ShouldEqual, 179)
				convey.So(chunk.ResponseMeta.Usage.PromptTokenDetails.CachedTokens, convey.ShouldEqual, 10)
				convey.So(chunk.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens, convey.ShouldEqual, 20)
				break
			}
		}

		convey.So(hasUsage, convey.ShouldBeTrue, "Usage information should be extracted from Event_Response")
	})
}
