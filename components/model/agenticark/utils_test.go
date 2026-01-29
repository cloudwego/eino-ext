/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticark

import (
	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"

	"testing"
)

func TestCoalesce(t *testing.T) {
	mockey.PatchConvey("TestCoalesce", t, func() {
		mockey.PatchConvey("x is not zero", func() {
			x := "1"
			y := ""
			got := coalesce(x, y)
			assert.Equal(t, x, got)
		})

		mockey.PatchConvey("x and y is pointer", func() {
			x := ptrOf("1")
			y := ptrOf("")
			got := coalesce(x, y)
			assert.Equal(t, x, got)
		})

		mockey.PatchConvey("x and y is nil pointer", func() {
			var x, y *string
			got := coalesce(x, y)
			assert.Equal(t, x, got)
		})
	})
}
