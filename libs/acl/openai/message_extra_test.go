package openai

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestReasoningContent(t *testing.T) {
	msg := &schema.Message{}
	_, ok := GetReasoningContent(msg)
	assert.False(t, ok)
	SetReasoningContent(msg, "reasoning content")
	content, ok := GetReasoningContent(msg)
	assert.True(t, ok)
	assert.Equal(t, "reasoning content", content)
}
