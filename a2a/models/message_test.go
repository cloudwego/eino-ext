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

package models

import "testing"

func TestSendMessageStreamingResponseUnion_GetTaskID(t *testing.T) {
	tid := "t-1"
	cases := []struct {
		name string
		u    *SendMessageStreamingResponseUnion
		want string
	}{
		{
			name: "from message TaskID",
			u:    &SendMessageStreamingResponseUnion{Message: &Message{TaskID: &tid}},
			want: "t-1",
		},
		{
			name: "from message without TaskID falls through",
			u:    &SendMessageStreamingResponseUnion{Message: &Message{}},
			want: "",
		},
		{
			name: "from task",
			u:    &SendMessageStreamingResponseUnion{Task: &Task{ID: "t-2"}},
			want: "t-2",
		},
		{
			name: "from status update",
			u:    &SendMessageStreamingResponseUnion{TaskStatusUpdateEvent: &TaskStatusUpdateEvent{TaskID: "t-3"}},
			want: "t-3",
		},
		{
			name: "from artifact update",
			u:    &SendMessageStreamingResponseUnion{TaskArtifactUpdateEvent: &TaskArtifactUpdateEvent{TaskID: "t-4"}},
			want: "t-4",
		},
		{
			name: "empty union",
			u:    &SendMessageStreamingResponseUnion{},
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.u.GetTaskID(); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestMessage_EnsureRequiredFields(t *testing.T) {
	t.Run("nil receiver no-op", func(t *testing.T) {
		var m *Message
		m.EnsureRequiredFields() // must not panic
	})
	t.Run("initializes nil parts", func(t *testing.T) {
		m := &Message{}
		m.EnsureRequiredFields()
		if m.Parts == nil {
			t.Errorf("Parts should be non-nil after EnsureRequiredFields")
		}
		if len(m.Parts) != 0 {
			t.Errorf("Parts should be empty: got %+v", m.Parts)
		}
	})
	t.Run("initializes nil Data on data parts", func(t *testing.T) {
		m := &Message{Parts: []Part{
			{Kind: PartKindData, Data: nil},
			{Kind: PartKindText, Text: nil},
		}}
		m.EnsureRequiredFields()
		if m.Parts[0].Data == nil {
			t.Errorf("data part: nil after EnsureRequiredFields")
		}
		// non-data parts should not have Data assigned
	})
}

func TestArtifact_EnsureRequiredFields(t *testing.T) {
	a := &Artifact{Parts: []Part{{Kind: PartKindData, Data: nil}}}
	a.EnsureRequiredFields()
	if a.Parts[0].Data == nil {
		t.Errorf("data part: nil after EnsureRequiredFields")
	}
}
