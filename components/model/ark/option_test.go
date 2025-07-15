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

	"github.com/stretchr/testify/assert"
	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino/components/model"
)

func TestOptions(t *testing.T) {
	opt := model.GetImplSpecificOptions(&arkOptions{
		customHeaders: nil,
	}, WithCustomHeader(map[string]string{"k1": "v1"}),
		WithCaching(CachingEnabled, 86400),
		WithSessionCache("123"),
		WithThinking(&arkModel.Thinking{
			Type: arkModel.ThinkingTypeEnabled,
		}))

	assert.Equal(t, map[string]string{"k1": "v1"}, opt.customHeaders)
	assert.Equal(t, CachingEnabled, *opt.caching)
	assert.Equal(t, 86400, *opt.cachingTTL)
	assert.Equal(t, "123", *opt.sessionContextID)
	assert.Equal(t, arkModel.ThinkingTypeEnabled, opt.thinking.Type)
}
