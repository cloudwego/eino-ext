/*
 * Copyright 2026 CloudWeGo Authors
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

package sequentialthinking

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestToolPreservesThoughtBeyondEstimate(t *testing.T) {
	tool, err := NewTool()
	if err != nil {
		t.Fatal(err)
	}
	result, err := tool.InvokableRun(context.Background(), `{"thought":"Verify the additional hypothesis","thought_number":4,"total_thoughts":3,"next_thought_needed":true}`)
	if err != nil {
		t.Fatal(err)
	}
	var thought ThoughtResult
	if err := json.Unmarshal([]byte(result), &thought); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(thought.Content, "Verify the additional hypothesis") {
		t.Fatalf("thought content lost: %s", thought.Content)
	}
	if !strings.Contains(thought.Content, "💭 Thought 4/4") {
		t.Fatalf("expected regular thought with expanded estimate, got %s", thought.Content)
	}
	if thought.TotalThoughts != 4 || thought.ThoughtHistoryLength != 1 {
		t.Fatalf("unexpected thought metadata: %+v", thought)
	}
}

func TestProcessThoughtPreservesRevisionBeyondEstimate(t *testing.T) {
	server := newThinkingServer()
	request := &ThoughtRequest{Thought: "Revise the hypothesis", ThoughtNumber: 4, TotalThoughts: 3, IsRevision: true, RevisesThought: 2}
	result, err := server.processThought(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "Revise the hypothesis") || request.RevisesThought != 2 {
		t.Fatalf("revision lost: result=%s, request=%+v", result.Content, request)
	}
	if len(server.thoughtHistory) != 1 || server.thoughtHistory[0].Thought != "Revise the hypothesis" {
		t.Fatalf("thought history lost original content: %+v", server.thoughtHistory)
	}
}
