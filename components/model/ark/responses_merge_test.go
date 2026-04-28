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

package ark

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"

	"github.com/cloudwego/eino/components/model"
)

func TestWithExtraFieldsOption(t *testing.T) {
	fields := map[string]any{"custom_param": "x", "nested": map[string]any{"k": 1}}
	opt := model.GetImplSpecificOptions(&arkOptions{}, WithExtraFields(fields))
	assert.Equal(t, fields, opt.extraFields)
}

func TestMergeResponsesRequestExtraJSON(t *testing.T) {
	req := &responses.ResponsesRequest{
		Model: "ep-test",
	}
	extra := map[string]any{"thinking": map[string]any{"type": "disabled"}}

	raw, err := mergeResponsesRequestExtraJSON(req, extra)
	assert.NoError(t, err)

	var out map[string]any
	assert.NoError(t, sonic.Unmarshal(raw, &out))
	assert.Equal(t, "ep-test", out["model"])
	th, ok := out["thinking"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "disabled", th["type"])
}
