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

package openai

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestDefaultOpenAIImplSpecificOptions(t *testing.T) {
	cm := &Client{config: &Config{Model: "test model"}}
	msg := schema.Message{
		Role:    schema.System,
		Content: "test",
	}
	msgs := []*schema.Message{&msg}
	req, _, err := cm.genRequest(msgs)
	assert.NoError(t, err)
	assert.Equal(t, req.ReasoningEffort, "")
}

func TestHighReasoningEffortOpenAIImplSpecificOptions(t *testing.T) {
	cm := &Client{config: &Config{Model: "test model"}}
	msg := schema.Message{
		Role:    schema.System,
		Content: "test",
	}
	msgs := []*schema.Message{&msg}
	req, _, err := cm.genRequest(msgs,
		WithReasoningEffort(ReasoningEffortLevelHigh))
	assert.NoError(t, err)
	assert.Equal(t, req.ReasoningEffort, string(ReasoningEffortLevelHigh))
}
