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

package apmplus

import (
	"context"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
)

// Test_SetSession tests the SetSession function
func Test_SetSession(t *testing.T) {
	mockey.PatchConvey("Test SetSession function", t, func() {
		mockey.PatchConvey("No SessionOption parameters", func() {
			// Initialize a context
			ctx := context.Background()
			// Call the function under test
			newCtx := SetSession(ctx)
			// Get sessionOptions from context
			options, ok := newCtx.Value(apmplusSessionOptionKey{}).(*sessionOptions)
			// Assert retrieval success
			convey.So(ok, convey.ShouldBeTrue)
			// Assert UserID is empty
			convey.So(options.UserID, convey.ShouldEqual, "")
			// Assert SessionID is empty
			convey.So(options.SessionID, convey.ShouldEqual, "")
		})

		mockey.PatchConvey("With SessionOption parameters", func() {
			// Initialize a context
			ctx := context.Background()
			// Call the function with SessionOption parameters
			newCtx := SetSession(ctx, WithUserID("testUser"), WithSessionID("testSession"))
			// Get sessionOptions from context
			options, ok := newCtx.Value(apmplusSessionOptionKey{}).(*sessionOptions)
			// Assert retrieval success
			convey.So(ok, convey.ShouldBeTrue)
			// Assert UserID, SessionID matches
			convey.So(options.UserID, convey.ShouldEqual, "testUser")
			convey.So(options.SessionID, convey.ShouldEqual, "testSession")
		})
	})
}
