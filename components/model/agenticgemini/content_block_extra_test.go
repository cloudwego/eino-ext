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

func TestSetAndGetThoughtSignature(t *testing.T) {
	t.Run("nil content block", func(t *testing.T) {
		setThoughtSignature(nil, []byte("test"))
		assert.Nil(t, getThoughtSignature(nil))
	})

	t.Run("set and get", func(t *testing.T) {
		cb := &schema.ContentBlock{}
		sig := []byte("thought-sig-123")

		setThoughtSignature(cb, sig)
		got := getThoughtSignature(cb)

		assert.Equal(t, sig, got)
	})

	t.Run("get from empty extra", func(t *testing.T) {
		cb := &schema.ContentBlock{}
		got := getThoughtSignature(cb)
		assert.Nil(t, got)
	})

	t.Run("get with wrong type in extra", func(t *testing.T) {
		cb := &schema.ContentBlock{
			Extra: map[string]interface{}{
				thoughtSignatureExtraKey: "not-bytes",
			},
		}
		got := getThoughtSignature(cb)
		assert.Nil(t, got)
	})

	t.Run("overwrite existing signature", func(t *testing.T) {
		cb := &schema.ContentBlock{}
		setThoughtSignature(cb, []byte("first"))
		setThoughtSignature(cb, []byte("second"))

		got := getThoughtSignature(cb)
		assert.Equal(t, []byte("second"), got)
	})
}
