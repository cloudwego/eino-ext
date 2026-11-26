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

package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestStreamThoughtSignaturePartRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name     string
		chunks   [][]*genai.Part
		expected []*genai.Part
	}{
		{"unsigned text", [][]*genai.Part{
			{{Text: "hello "}}, {{Text: "world"}}, {{}},
		}, []*genai.Part{{Text: "hello world"}}},
		{"signature only", [][]*genai.Part{
			{{ThoughtSignature: []byte("tail")}},
		}, []*genai.Part{{ThoughtSignature: []byte("tail")}}},
		{"trailing signature", [][]*genai.Part{
			{{Text: "hello "}}, {{Text: "world"}}, {{ThoughtSignature: []byte("tail")}},
		}, []*genai.Part{{Text: "hello world"}, {ThoughtSignature: []byte("tail")}}},
		{"consecutive signatures", [][]*genai.Part{
			{{Text: "hello"}}, {{ThoughtSignature: []byte("first")}}, {{ThoughtSignature: []byte("second")}},
		}, []*genai.Part{{Text: "hello"}, {ThoughtSignature: []byte("first")}, {ThoughtSignature: []byte("second")}}},
		{"signed text boundaries", [][]*genai.Part{
			{{Text: "before"}}, {{Text: "signed", ThoughtSignature: []byte("signature")}}, {{Text: "after"}},
		}, []*genai.Part{{Text: "before"}, {Text: "signed", ThoughtSignature: []byte("signature")}, {Text: "after"}}},
		{"same chunk tail", [][]*genai.Part{
			{{Text: "answer"}, {ThoughtSignature: []byte("tail")}},
		}, []*genai.Part{{Text: "answer"}, {ThoughtSignature: []byte("tail")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan []*genai.Content, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Contents []*genai.Content `json:"contents"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				requests <- body.Contents
				w.Header().Set("Content-Type", "text/event-stream")
				for i, parts := range tc.chunks {
					candidate := &genai.Candidate{Content: &genai.Content{Role: "model", Parts: parts}}
					if i == len(tc.chunks)-1 {
						candidate.FinishReason = genai.FinishReasonStop
					}
					b, err := json.Marshal(&genai.GenerateContentResponse{Candidates: []*genai.Candidate{candidate}})
					if err != nil {
						t.Error(err)
						return
					}
					fmt.Fprintf(w, "data: %s\n\n", b)
				}
			}))
			defer server.Close()
			ctx := context.Background()
			client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: "local-test", Backend: genai.BackendVertexAI, HTTPClient: server.Client(), HTTPOptions: genai.HTTPOptions{BaseURL: server.URL}})
			require.NoError(t, err)
			model, err := NewChatModel(ctx, &Config{Client: client, Model: "gemini-test"})
			require.NoError(t, err)
			stream, err := model.Stream(ctx, []*schema.Message{schema.UserMessage("hello")})
			require.NoError(t, err)
			var chunks []*schema.Message
			for {
				msg, e := stream.Recv()
				if e == io.EOF {
					break
				}
				require.NoError(t, e)
				chunks = append(chunks, msg)
			}
			stream.Close()
			<-requests
			merged, err := schema.ConcatMessages(chunks)
			require.NoError(t, err)
			encoded, err := json.Marshal(merged)
			require.NoError(t, err)
			var restored schema.Message
			require.NoError(t, json.Unmarshal(encoded, &restored))
			stream, err = model.Stream(ctx, []*schema.Message{schema.UserMessage("hello"), &restored, schema.UserMessage("continue")})
			require.NoError(t, err)
			for {
				_, e := stream.Recv()
				if e == io.EOF {
					break
				}
				require.NoError(t, e)
			}
			stream.Close()
			sent := <-requests
			require.Len(t, sent, 3)
			require.Equal(t, tc.expected, sent[1].Parts)
		})
	}
}

func TestCandidateSignatureOnlyPart(t *testing.T) {
	candidate := &genai.Candidate{Content: &genai.Content{Role: "model", Parts: []*genai.Part{{Text: "answer"}, {ThoughtSignature: []byte("tail")}}}}
	message, err := convCandidate(candidate)
	require.NoError(t, err)
	content, err := convSchemaMessage(message)
	require.NoError(t, err)
	require.Equal(t, candidate.Content.Parts, content.Parts)
}
