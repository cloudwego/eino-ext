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

package agenticopenai

import (
	"testing"

	"github.com/bytedance/mockey"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/stretchr/testify/assert"
)

func TestNewOpenaiOpt(t *testing.T) {
	mockey.PatchConvey("newOpenaiOpt", t, func() {
		mockey.PatchConvey("non-nil value", func() {
			val := 123
			opt := newOpenaiOpt(&val)
			assert.Equal(t, param.NewOpt(val), opt)
		})

		mockey.PatchConvey("nil value", func() {
			var val *int
			opt := newOpenaiOpt(val)
			assert.Equal(t, param.Opt[int]{}, opt)
		})
	})
}

func TestNewOpenaiStrOpt(t *testing.T) {
	mockey.PatchConvey("newOpenaiStrOpt", t, func() {
		mockey.PatchConvey("non-empty string", func() {
			val := "hello"
			opt := newOpenaiStrOpt(val)
			assert.Equal(t, param.NewOpt(val), opt)
		})

		mockey.PatchConvey("empty string", func() {
			opt := newOpenaiStrOpt("")
			assert.Equal(t, param.Opt[string]{}, opt)
		})
	})
}

func TestCoalesce(t *testing.T) {
	mockey.PatchConvey("coalesce", t, func() {
		mockey.PatchConvey("x is non-zero", func() {
			x := "a"
			y := "b"
			assert.Equal(t, x, coalesce(x, y))
		})

		mockey.PatchConvey("x is zero", func() {
			x := ""
			y := "b"
			assert.Equal(t, y, coalesce(x, y))
		})
	})
}

func TestPtrOf(t *testing.T) {
	mockey.PatchConvey("ptrOf", t, func() {
		mockey.PatchConvey("int", func() {
			val := 123
			res := ptrOf(val)
			assert.Equal(t, val, *res)
		})

		mockey.PatchConvey("string", func() {
			val := "hello"
			res := ptrOf(val)
			assert.Equal(t, val, *res)
		})
	})
}

func TestInt64ToStr(t *testing.T) {
	mockey.PatchConvey("int64ToStr", t, func() {
		mockey.PatchConvey("positive", func() {
			assert.Equal(t, "123", int64ToStr(123))
		})

		mockey.PatchConvey("negative", func() {
			assert.Equal(t, "-123", int64ToStr(-123))
		})

		mockey.PatchConvey("zero", func() {
			assert.Equal(t, "0", int64ToStr(0))
		})
	})
}

func TestNewPanicErr(t *testing.T) {
	mockey.PatchConvey("newPanicErr", t, func() {
		mockey.PatchConvey("error interface and formatting", func() {
			info := "something went wrong"
			stack := []byte("stack trace")
			err := newPanicErr(info, stack)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), info)
			assert.Contains(t, err.Error(), string(stack))
		})
	})
}
