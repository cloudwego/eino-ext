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
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestMessageExtra_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		set  func(m *schema.Message)
		get  func(m *schema.Message) (any, bool)
		want any
	}{
		{
			"messageID",
			func(m *schema.Message) { SetMessageID(m, "m-1") },
			func(m *schema.Message) (any, bool) { v, ok := GetMessageID(m); return v, ok },
			"m-1",
		},
		{
			"taskID",
			func(m *schema.Message) { SetTaskID(m, "t-1") },
			func(m *schema.Message) (any, bool) { v, ok := GetTaskID(m); return v, ok },
			"t-1",
		},
		{
			"contextID",
			func(m *schema.Message) { SetContextID(m, "c-1") },
			func(m *schema.Message) (any, bool) { v, ok := GetContextID(m); return v, ok },
			"c-1",
		},
		{
			"artifactID",
			func(m *schema.Message) { SetArtifactID(m, "a-1") },
			func(m *schema.Message) (any, bool) { v, ok := GetArtifactID(m); return v, ok },
			"a-1",
		},
		{
			"referenceTaskIDs",
			func(m *schema.Message) { SetReferenceTaskIDs(m, []string{"r-1", "r-2"}) },
			func(m *schema.Message) (any, bool) { v, ok := GetReferenceTaskIDs(m); return v, ok },
			[]string{"r-1", "r-2"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &schema.Message{}
			c.set(m)
			got, ok := c.get(m)
			if !ok {
				t.Fatalf("expected ok=true after set")
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("get: got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestMessageExtra_AbsentKey(t *testing.T) {
	m := &schema.Message{Extra: map[string]any{}}
	if _, ok := GetMessageID(m); ok {
		t.Errorf("expected ok=false for absent messageID")
	}
	if _, ok := GetTaskID(m); ok {
		t.Errorf("expected ok=false for absent taskID")
	}
	if _, ok := GetContextID(m); ok {
		t.Errorf("expected ok=false for absent contextID")
	}
	if _, ok := GetArtifactID(m); ok {
		t.Errorf("expected ok=false for absent artifactID")
	}
	if _, ok := GetReferenceTaskIDs(m); ok {
		t.Errorf("expected ok=false for absent referenceTaskIDs")
	}
}

func TestMessageExtra_NilMessage(t *testing.T) {
	// Setters silently no-op on nil.
	SetMessageID(nil, "m")
	SetTaskID(nil, "t")
	SetContextID(nil, "c")
	SetArtifactID(nil, "a")
	SetReferenceTaskIDs(nil, []string{"r"})

	// Getters return zero-value, ok=false on nil.
	if v, ok := GetMessageID(nil); ok || v != "" {
		t.Errorf("nil GetMessageID: got %q, %v", v, ok)
	}
	if v, ok := GetTaskID(nil); ok || v != "" {
		t.Errorf("nil GetTaskID: got %q, %v", v, ok)
	}
	if v, ok := GetContextID(nil); ok || v != "" {
		t.Errorf("nil GetContextID: got %q, %v", v, ok)
	}
	if v, ok := GetArtifactID(nil); ok || v != "" {
		t.Errorf("nil GetArtifactID: got %q, %v", v, ok)
	}
	if v, ok := GetReferenceTaskIDs(nil); ok || v != nil {
		t.Errorf("nil GetReferenceTaskIDs: got %v, %v", v, ok)
	}
}

func TestMessageExtra_LazyInitExtra(t *testing.T) {
	m := &schema.Message{} // Extra is nil
	SetMessageID(m, "m-1")
	if m.Extra == nil {
		t.Fatal("Extra should be initialized after Set")
	}
	if got := m.Extra[extraKeyOfMessageID]; got != "m-1" {
		t.Errorf("messageID stored: got %v", got)
	}
}
