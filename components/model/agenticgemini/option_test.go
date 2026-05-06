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

	"github.com/cloudwego/eino/components/model"
	"github.com/eino-contrib/jsonschema"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func TestSpecificOptions(t *testing.T) {
	o := model.GetImplSpecificOptions[options](nil,
		WithTopK(2),
		WithResponseJSONSchema(&jsonschema.Schema{}),
		WithCachedContentName("name"),
		WithThinkingConfig(&genai.ThinkingConfig{}),
		WithResponseModalities([]ResponseModality{ResponseModalityText}),
	)

	assert.Equal(t, int32(2), *o.TopK)
	assert.NotNil(t, o.ResponseJSONSchema)
	assert.Equal(t, "name", o.CachedContentName)
	assert.NotNil(t, o.ThinkingConfig)
	assert.Len(t, o.ResponseModalities, 1)
}
