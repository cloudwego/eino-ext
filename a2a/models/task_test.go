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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsureRequiredFields(t *testing.T) {
	r := ResponseEvent{
		Message: &Message{
			Parts: nil,
		},
		TaskContent: &TaskContent{
			Status: TaskStatus{Message: &Message{Parts: []Part{
				{Kind: PartKindData, Data: nil},
			}}},
			Artifacts: []*Artifact{{Parts: []Part{
				{Kind: PartKindData, Data: nil},
			}}},
			History: []*Message{{
				Parts: []Part{{Kind: PartKindData, Data: nil}},
			}},
		},
		TaskStatusUpdateEventContent: &TaskStatusUpdateEventContent{
			Status: TaskStatus{Message: &Message{Parts: nil}},
		},
		TaskArtifactUpdateEventContent: &TaskArtifactUpdateEventContent{
			Artifact: Artifact{Parts: []Part{{Kind: PartKindData, Data: nil}}},
		},
	}
	expected := ResponseEvent{
		Message: &Message{
			Parts: []Part{},
		},
		TaskContent: &TaskContent{
			Status: TaskStatus{Message: &Message{Parts: []Part{
				{Kind: PartKindData, Data: make(map[string]any)},
			}}},
			Artifacts: []*Artifact{{Parts: []Part{
				{Kind: PartKindData, Data: make(map[string]any)},
			}}},
			History: []*Message{{
				Parts: []Part{{Kind: PartKindData, Data: make(map[string]any)}},
			}},
		},
		TaskStatusUpdateEventContent: &TaskStatusUpdateEventContent{
			Status: TaskStatus{Message: &Message{Parts: make([]Part, 0)}},
		},
		TaskArtifactUpdateEventContent: &TaskArtifactUpdateEventContent{
			Artifact: Artifact{Parts: []Part{{Kind: PartKindData, Data: make(map[string]any)}}},
		},
	}
	(&r).EnsureRequiredFields()
	assert.Equal(t, expected, r)
}
