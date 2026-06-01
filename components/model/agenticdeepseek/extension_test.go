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

package agenticdeepseek

import (
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino/schema"
)

func TestExtractResponseMetaExtension(t *testing.T) {
	PatchConvey("test extractResponseMetaExtension", t, func() {
		PatchConvey("nil Extra", func() {
			msg := &schema.AgenticMessage{}
			extractResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldBeNil)
		})

		PatchConvey("Extra without extension key", func() {
			msg := &schema.AgenticMessage{
				Extra: map[string]any{"other_key": "value"},
			}
			extractResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldBeNil)
		})

		PatchConvey("Extra with wrong type", func() {
			msg := &schema.AgenticMessage{
				Extra: map[string]any{extraKeyResponseMetaExtension: "wrong_type"},
			}
			extractResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldBeNil)
		})

		PatchConvey("Extra with valid extension and nil ResponseMeta", func() {
			ext := &ResponseMetaExtension{FinishReason: "stop"}
			msg := &schema.AgenticMessage{
				Extra: map[string]any{extraKeyResponseMetaExtension: ext},
			}
			extractResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta, convey.ShouldNotBeNil)
			convey.So(msg.ResponseMeta.Extension, convey.ShouldEqual, ext)
		})

		PatchConvey("Extra with valid extension and existing ResponseMeta", func() {
			ext := &ResponseMetaExtension{FinishReason: "length"}
			msg := &schema.AgenticMessage{
				Extra:        map[string]any{extraKeyResponseMetaExtension: ext},
				ResponseMeta: &schema.AgenticResponseMeta{},
			}
			extractResponseMetaExtension(msg)
			convey.So(msg.ResponseMeta.Extension, convey.ShouldEqual, ext)
		})
	})
}

func TestSetMsgExtra(t *testing.T) {
	PatchConvey("test setMsgExtra", t, func() {
		PatchConvey("nil Extra map", func() {
			msg := &schema.Message{}
			setMsgExtra(msg, "key1", "value1")
			convey.So(msg.Extra, convey.ShouldNotBeNil)
			convey.So(msg.Extra["key1"], convey.ShouldEqual, "value1")
		})

		PatchConvey("existing Extra map", func() {
			msg := &schema.Message{Extra: map[string]any{"existing": "val"}}
			setMsgExtra(msg, "key2", "value2")
			convey.So(msg.Extra["existing"], convey.ShouldEqual, "val")
			convey.So(msg.Extra["key2"], convey.ShouldEqual, "value2")
		})
	})
}

func TestConcatResponseMetaExtensions(t *testing.T) {
	PatchConvey("test concatResponseMetaExtensions", t, func() {
		PatchConvey("empty chunks", func() {
			result, err := concatResponseMetaExtensions(nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldBeNil)
		})

		PatchConvey("single chunk", func() {
			ext := &ResponseMetaExtension{FinishReason: "stop"}
			result, err := concatResponseMetaExtensions([]*ResponseMetaExtension{ext})
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldEqual, ext)
		})

		PatchConvey("multiple chunks", func() {
			logProbs := &schema.LogProbs{Content: []schema.LogProb{{Token: "a"}}}
			chunks := []*ResponseMetaExtension{
				{FinishReason: ""},
				{FinishReason: "stop", LogProbs: logProbs},
			}
			result, err := concatResponseMetaExtensions(chunks)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.FinishReason, convey.ShouldEqual, "stop")
			convey.So(result.LogProbs, convey.ShouldEqual, logProbs)
		})

		PatchConvey("multiple chunks with overwrite", func() {
			chunks := []*ResponseMetaExtension{
				{FinishReason: "length"},
				{FinishReason: "stop"},
			}
			result, err := concatResponseMetaExtensions(chunks)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.FinishReason, convey.ShouldEqual, "stop")
		})

		PatchConvey("multiple chunks append logprobs", func() {
			chunks := []*ResponseMetaExtension{
				{LogProbs: &schema.LogProbs{Content: []schema.LogProb{{Token: "a"}}}},
				{LogProbs: &schema.LogProbs{Content: []schema.LogProb{{Token: "b"}}}},
			}
			result, err := concatResponseMetaExtensions(chunks)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.LogProbs, convey.ShouldNotBeNil)
			convey.So(result.LogProbs.Content, convey.ShouldResemble, []schema.LogProb{
				{Token: "a"},
				{Token: "b"},
			})
		})
	})
}

func TestResponseMetaModifier(t *testing.T) {
	PatchConvey("test responseMetaModifier", t, func() {
		PatchConvey("returns non-nil option", func() {
			opt := responseMetaModifier()
			convey.So(opt, convey.ShouldNotBeNil)
		})
	})
}

func TestResponseMetaChunkModifier(t *testing.T) {
	PatchConvey("test responseMetaChunkModifier", t, func() {
		PatchConvey("returns non-nil option", func() {
			opt := responseMetaChunkModifier()
			convey.So(opt, convey.ShouldNotBeNil)
		})
	})
}
