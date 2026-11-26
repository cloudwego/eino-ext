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

package eino

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/a2a/models"
)

func TestConvInputMessages_default(t *testing.T) {
	got, err := convInputMessages(context.Background(),
		[]adk.Message{{Role: schema.User, Content: "ping"}, {Role: schema.User, Content: "pong"}},
		nil,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Role != models.RoleUser {
		t.Errorf("role: got %q, want %q", got.Role, models.RoleUser)
	}
	if len(got.Parts) != 2 || *got.Parts[0].Text != "ping" || *got.Parts[1].Text != "pong" {
		t.Errorf("parts: %+v", got.Parts)
	}
}

func TestConvInputMessages_customConvertor(t *testing.T) {
	conv := func(_ context.Context, ms []*schema.Message) ([]models.Part, error) {
		s := "stub"
		return []models.Part{{Kind: models.PartKindText, Text: &s}}, nil
	}
	got, err := convInputMessages(context.Background(),
		[]adk.Message{{Content: "ignored"}},
		conv,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got.Parts) != 1 || *got.Parts[0].Text != "stub" {
		t.Errorf("custom convertor used: got %+v", got.Parts)
	}
}

func TestConvInputMessages_convertorError(t *testing.T) {
	conv := func(_ context.Context, _ []*schema.Message) ([]models.Part, error) {
		return nil, errors.New("boom")
	}
	_, err := convInputMessages(context.Background(), nil, conv)
	if err == nil || err.Error() != "boom" {
		t.Errorf("err: got %v, want boom", err)
	}
}

func TestHandleNewMessage_singleFinal(t *testing.T) {
	idMap := map[string]*schema.StreamWriter[*schema.Message]{}
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		handleNewMessage("id-1", idMap, &schema.Message{Content: "complete"}, true, schema.Assistant, &AgentEventSender{gen: gen})
	}()

	ev, ok := iter.Next()
	if !ok {
		t.Fatal("expected event")
	}
	if ev.Output == nil || ev.Output.MessageOutput == nil {
		t.Fatalf("MessageOutput: got %+v", ev)
	}
	if ev.Output.MessageOutput.IsStreaming {
		t.Errorf("single-final should not stream")
	}
	if ev.Output.MessageOutput.Message == nil || ev.Output.MessageOutput.Message.Content != "complete" {
		t.Errorf("message: got %+v", ev.Output.MessageOutput.Message)
	}
	if _, ok := idMap["id-1"]; ok {
		t.Errorf("idMap should not retain final-only entries")
	}
	if _, more := iter.Next(); more {
		t.Errorf("expected stream end")
	}
}

func TestHandleNewMessage_multiChunk(t *testing.T) {
	idMap := map[string]*schema.StreamWriter[*schema.Message]{}
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		sender := &AgentEventSender{gen: gen}
		handleNewMessage("id-1", idMap, &schema.Message{Content: "a "}, false, schema.Assistant, sender)
		handleNewMessage("id-1", idMap, &schema.Message{Content: "b "}, false, schema.Assistant, sender)
		handleNewMessage("id-1", idMap, &schema.Message{Content: "c"}, true, schema.Assistant, sender)
	}()

	ev, ok := iter.Next()
	if !ok {
		t.Fatal("expected first event")
	}
	if !ev.Output.MessageOutput.IsStreaming {
		t.Fatal("expected IsStreaming=true on first event")
	}
	sr := ev.Output.MessageOutput.MessageStream
	concat := ""
	for {
		m, err := sr.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		concat += m.Content
	}
	if concat != "a b c" {
		t.Errorf("stream: got %q, want %q", concat, "a b c")
	}
	if _, more := iter.Next(); more {
		t.Errorf("expected stream end")
	}
	if _, ok := idMap["id-1"]; ok {
		t.Errorf("idMap should be cleared after final chunk")
	}
}

type sliceReceiver struct {
	idx    int
	events []*models.SendMessageStreamingResponseUnion
}

func (s *sliceReceiver) Recv() (*models.SendMessageStreamingResponseUnion, error) {
	if s.idx >= len(s.events) {
		return nil, io.EOF
	}
	e := s.events[s.idx]
	s.idx++
	return e, nil
}

func (s *sliceReceiver) Close() error { return nil }

func TestDefaultOutputConvertor_messageStream(t *testing.T) {
	mkChunk := func(text string, final bool) *models.SendMessageStreamingResponseUnion {
		t := text
		md := map[string]any{}
		setStreamChunkFinal(md, final)
		return &models.SendMessageStreamingResponseUnion{
			Message: &models.Message{
				Role:      models.RoleAgent,
				MessageID: "msg-1",
				Parts:     []models.Part{{Kind: models.PartKindText, Text: &t}},
				Metadata:  md,
			},
		}
	}
	rec := &sliceReceiver{events: []*models.SendMessageStreamingResponseUnion{
		mkChunk("hello ", false),
		mkChunk("streaming ", false),
		mkChunk("world", true),
	}}

	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		defaultOutputConvertor(context.Background(), &ResponseUnionReceiver{rec}, &AgentEventSender{gen: gen})
	}()

	ev, ok := iter.Next()
	if !ok {
		t.Fatal("expected first event")
	}
	if !ev.Output.MessageOutput.IsStreaming {
		t.Fatalf("IsStreaming=true expected; ev=%+v", ev)
	}
	got := ""
	for {
		m, err := ev.Output.MessageOutput.MessageStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		got += m.Content
	}
	if got != "hello streaming world" {
		t.Errorf("stream: got %q", got)
	}
}

func TestDefaultOutputConvertor_completeMessage(t *testing.T) {
	text := "done"
	rec := &sliceReceiver{events: []*models.SendMessageStreamingResponseUnion{
		{Message: &models.Message{
			Role:      models.RoleAgent,
			MessageID: "m-1",
			Parts:     []models.Part{{Kind: models.PartKindText, Text: &text}},
		}},
	}}

	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		defaultOutputConvertor(context.Background(), &ResponseUnionReceiver{rec}, &AgentEventSender{gen: gen})
	}()

	ev, ok := iter.Next()
	if !ok {
		t.Fatal("expected event")
	}
	if ev.Output.MessageOutput.IsStreaming {
		t.Errorf("non-streaming Message should not produce IsStreaming=true")
	}
	if ev.Output.MessageOutput.Message == nil || ev.Output.MessageOutput.Message.Content != "done" {
		t.Errorf("content: got %+v", ev.Output.MessageOutput.Message)
	}
}

func TestDefaultOutputConvertor_taskInterrupt(t *testing.T) {
	text := "need input"
	rec := &sliceReceiver{events: []*models.SendMessageStreamingResponseUnion{
		{Task: &models.Task{
			ID: "t-1",
			Status: models.TaskStatus{
				State: models.TaskStateInputRequired,
				Message: &models.Message{
					Role:  models.RoleAgent,
					Parts: []models.Part{{Kind: models.PartKindText, Text: &text}},
				},
			},
		}},
	}}

	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		defaultOutputConvertor(context.Background(), &ResponseUnionReceiver{rec}, &AgentEventSender{gen: gen})
	}()

	ev, ok := iter.Next()
	if !ok {
		t.Fatal("expected event")
	}
	if ev.Action == nil || ev.Action.Interrupted == nil {
		t.Fatalf("interrupt: got %+v", ev)
	}
	info, ok := ev.Action.Interrupted.Data.(*InterruptInfo)
	if !ok {
		t.Fatalf("interrupt data type: got %T", ev.Action.Interrupted.Data)
	}
	if info.TaskID != "t-1" {
		t.Errorf("interrupt taskID: got %q", info.TaskID)
	}
}

func TestLocalResponseUnionReceiver(t *testing.T) {
	u := &models.SendMessageStreamingResponseUnion{Task: &models.Task{ID: "t"}}
	r := &localResponseUnionReceiver{union: u}

	got, err := r.Recv()
	if err != nil {
		t.Fatalf("first recv err: %v", err)
	}
	if got != u {
		t.Errorf("first recv: got %+v, want passthrough", got)
	}

	_, err = r.Recv()
	if err != io.EOF {
		t.Errorf("second recv err: got %v, want io.EOF", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("close err: %v", err)
	}
}
