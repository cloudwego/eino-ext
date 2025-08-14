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

package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/schema"
)

func TestConcatMessages(t *testing.T) {
	msgs := []*schema.Message{
		{
			Extra: map[string]any{
				"key_of_string": "hi!",
				"key_of_int":    int(10),
				keyOfThinking:   "how ",
			},
		},
		{
			Extra: map[string]any{
				"key_of_string": "hello!",
				"key_of_int":    int(50),
				keyOfThinking:   "are you",
			},
		},
	}

	msg, err := schema.ConcatMessages(msgs)
	assert.NoError(t, err)
	assert.Equal(t, "hi!hello!", msg.Extra["key_of_string"])
	assert.Equal(t, int(50), msg.Extra["key_of_int"])

	reasoningContent, ok := GetThinking(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, "how are you", reasoningContent)
}

func TestSetMessageBreakpointOfClaude(t *testing.T) {
	msg := &schema.Message{
		Role:    schema.System,
		Content: "test",
		Extra: map[string]any{
			"test": "test",
		},
	}

	msg_ := SetMessageBreakpoint(msg)
	assert.Len(t, msg.Extra, 1)
	assert.Len(t, msg_.Extra, 2)
}

func TestSetToolInfoBreakpointOfClaude(t *testing.T) {
	toolInfo := &schema.ToolInfo{
		Name: "test",
		Desc: "test",
		Extra: map[string]any{
			"test": "test",
		},
	}

	toolInfo_ := SetToolInfoBreakpoint(toolInfo)
	assert.Len(t, toolInfo.Extra, 1)
	assert.Len(t, toolInfo_.Extra, 2)
}
