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

package langfuse

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestLangfuse(t *testing.T) {
	mockey.PatchConvey("", t, func() {
		mockey.Mock((*http.Client).Do).To(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == getUploadURLPath {
				respBody, _ := sonic.Marshal(&getUploadURLResponse{
					MediaID:   "mediaID",
					UploadURL: "url",
				})
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBuffer(respBody))}, nil
			} else if req.URL.Path == ingestionPath {
				respBody, _ := sonic.Marshal(&batchIngestionResponse{})
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBuffer(respBody))}, nil
			}
			return &http.Response{StatusCode: http.StatusOK}, nil
		}).Build()
		lf := NewLangfuse("https://host", "pk", "sk")
		traceID, err := lf.CreateTrace(&TraceEventBody{
			BaseEventBody: BaseEventBody{
				Name: "test",
			},
			TimeStamp: time.Now(),
		})
		assert.Nil(t, err)
		spanID, err := lf.CreateSpan(&SpanEventBody{
			BaseObservationEventBody: BaseObservationEventBody{
				BaseEventBody: BaseEventBody{
					Name: "test span",
				},
				TraceID:   traceID,
				StartTime: time.Now(),
			},
		})
		assert.Nil(t, err)
		_, err = lf.CreateEvent(&EventEventBody{BaseObservationEventBody{
			BaseEventBody: BaseEventBody{
				Name: "test event",
			},
			ParentObservationID: spanID,
			TraceID:             traceID,
			StartTime:           time.Now(),
		}})
		assert.Nil(t, err)

		genID, err := lf.CreateGeneration(&GenerationEventBody{
			BaseObservationEventBody: BaseObservationEventBody{
				BaseEventBody: BaseEventBody{
					Name:    "wdz test model",
					Version: "1",
				},
				TraceID:             traceID,
				ParentObservationID: spanID,
				StartTime:           time.Now(),
			},
			InMessages: []*schema.Message{
				{
					Role: schema.User,
					MultiContent: []schema.ChatMessagePart{
						{
							Type: schema.ChatMessagePartTypeText,
							Text: "text",
						},
					},
				},
			},
			Model:         "gpt-4o",
			PromptName:    "test prompt",
			PromptVersion: 1,
		})
		assert.Nil(t, err)
		err = lf.EndGeneration(&GenerationEventBody{
			BaseObservationEventBody: BaseObservationEventBody{
				BaseEventBody: BaseEventBody{
					ID: genID,
				},
				TraceID:             traceID,
				ParentObservationID: spanID,
			},
			OutMessage: &schema.Message{
				Role:    schema.Assistant,
				Content: "good",
			},
			CompletionStartTime: time.Now(),
			EndTime:             time.Now(),
			Usage: &UsageDetail{
				PromptTokens:     10,
				CompletionTokens: 100,
				TotalTokens:      110,
			},
		})
		assert.Nil(t, err)
		err = lf.EndSpan(&SpanEventBody{
			BaseObservationEventBody: BaseObservationEventBody{
				BaseEventBody: BaseEventBody{
					ID:   spanID,
					Name: "test event",
				},
				TraceID:   traceID,
				StartTime: time.Now(),
			},
			EndTime: time.Now().Add(time.Second * 2),
		})
		assert.Nil(t, err)
		spanID2, err := lf.CreateSpan(&SpanEventBody{
			BaseObservationEventBody: BaseObservationEventBody{
				BaseEventBody: BaseEventBody{
					Name: "test span2",
				},
				TraceID:   traceID,
				StartTime: time.Now(),
			},
		})
		assert.Nil(t, err)
		_, err = lf.CreateEvent(&EventEventBody{BaseObservationEventBody{
			BaseEventBody: BaseEventBody{
				Name: "test event2",
			},
			ParentObservationID: spanID2,
			TraceID:             traceID,
			StartTime:           time.Now().Add(time.Second),
		}})
		assert.Nil(t, err)
		err = lf.EndSpan(&SpanEventBody{
			BaseObservationEventBody: BaseObservationEventBody{
				BaseEventBody: BaseEventBody{
					ID:   spanID2,
					Name: "test event",
				},
				TraceID:   traceID,
				StartTime: time.Now(),
			},
			EndTime: time.Now().Add(time.Second * 2),
		})
		assert.Nil(t, err)
		lf.Flush()
	})
}
