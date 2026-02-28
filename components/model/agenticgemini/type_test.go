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

package agenticgemini

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestCustomConcat(t *testing.T) {
	extras := []map[string]any{
		{"ExecutableCode": &ServerToolCallArguments{ExecutableCode: &ExecutableCode{Code: "1", Language: "1"}}},
		{"ExecutableCode": &ServerToolCallArguments{ExecutableCode: &ExecutableCode{Code: "2", Language: "2"}}},
		{"ExecutableCode": &ServerToolCallArguments{ExecutableCode: &ExecutableCode{Code: "3", Language: ""}}},
		{"CodeExecutionResult": &ServerToolCallResult{CodeExecutionResult: &CodeExecutionResult{Outcome: "1", Output: "1"}}},
		{"CodeExecutionResult": &ServerToolCallResult{CodeExecutionResult: &CodeExecutionResult{Outcome: "2", Output: "2"}}},
		{"CodeExecutionResult": &ServerToolCallResult{CodeExecutionResult: &CodeExecutionResult{Outcome: "", Output: "3"}}},
	}

	var msgs []*schema.Message
	for _, extra := range extras {
		msgs = append(msgs, &schema.Message{
			Role:  schema.Assistant,
			Extra: extra,
		})
	}

	msg, err := schema.ConcatMessages(msgs)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{
		"ExecutableCode":      &ServerToolCallArguments{ExecutableCode: &ExecutableCode{Code: "123", Language: "2"}},
		"CodeExecutionResult": &ServerToolCallResult{CodeExecutionResult: &CodeExecutionResult{Outcome: "2", Output: "123"}},
	}, msg.Extra)
}
