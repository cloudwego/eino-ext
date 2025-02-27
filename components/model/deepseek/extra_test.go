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

package deepseek

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

func TestPrefix(t *testing.T) {
	msg := &schema.Message{}
	assert.False(t, HasPrefix(msg))
	SetPrefix(msg)
	assert.True(t, HasPrefix(msg))
}
