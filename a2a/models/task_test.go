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
