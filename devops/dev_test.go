/*
 * Copyright 2024 CloudWeGo Authors
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

package einodev

import (
	"context"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

func Test_Debug(t *testing.T) {
	ctx := context.Background()
	PatchConvey("Test Success", t, func() {
		PatchConvey("Test success", func() {
			actualErr := Run(ctx)
			assert.Nil(t, actualErr)
		})
	})

	PatchConvey("Test Init Success", t, func() {
		PatchConvey("Test success", func() {
			actualErr := Init(ctx)
			assert.Nil(t, actualErr)
		})
	})
}