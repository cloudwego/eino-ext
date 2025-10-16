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
	"context"
	"errors"
	"io"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"

	"github.com/cloudwego/eino/schema"
)

func TestImageGenerationGenerate(t *testing.T) {
	PatchConvey("test ImageGeneration Generate", t, func() {
		ctx := context.Background()
		im, err := NewImageGenerationModel(ctx, &ImageGenerationConfig{
			Model:  "test-image-model",
			APIKey: "test-api-key",
		})
		convey.So(err, convey.ShouldBeNil)
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "a cat",
				MultiContent: []schema.ChatMessagePart{
					{
						Type: schema.ChatMessagePartTypeImageURL,
						ImageURL: &schema.ChatMessageImageURL{
							URL: "https://example.com/cat.png",
						},
					},
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "a cat",
					},
				},
			},
		}

		PatchConvey("test generate images error", func() {
			Mock(GetMethod(im.client, "GenerateImages")).Return(
				nil, errors.New("test generate error")).Build()

			outMsg, err := im.Generate(ctx, msgs)

			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test response with error", func() {
			Mock(GetMethod(im.client, "GenerateImages")).Return(
				model.ImagesResponse{
					Error: &model.GenerateImagesError{Code: "1", Message: "internal error"},
				}, nil).Build()

			outMsg, err := im.Generate(ctx, msgs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "internal error")
			convey.So(outMsg, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			testURL := "https://example.com/cat.png"
			Mock(GetMethod(im.client, "GenerateImages")).Return(
				model.ImagesResponse{
					Data: []*model.Image{
						{
							Url: &testURL,
						},
					},
					Usage: &model.GenerateImagesUsage{
						TotalTokens:  10,
						OutputTokens: 2,
					},
				}, nil).Build()

			outMsg, err := im.Generate(ctx, msgs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(outMsg, convey.ShouldNotBeNil)
			convey.So(outMsg.Role, convey.ShouldEqual, schema.Assistant)
			convey.So(len(outMsg.MultiContent), convey.ShouldEqual, 1)
			convey.So(outMsg.MultiContent[0].ImageURL.URL, convey.ShouldEqual, testURL)
		})
	})
}

// func TestImageGenerationGenerate(t *testing.T) {
// 	PatchConvey("test ImageGeneration Generate", t, func() {
// 		ctx := context.Background()
// 		im, err := NewImageGenerationModel(ctx, &ImageGenerationConfig{
// 			Model:  "test-image-model",
// 			APIKey: "test-api-key",
// 		})
// 		convey.So(err, convey.ShouldBeNil)
// 		msgs := []*schema.Message{
// 			{
// 				Role:    schema.User,
// 				Content: "a cat",
// 				UserInputMultiContent: []schema.MessageInputPart{
// 					{
// 						Type: schema.ChatMessagePartTypeImageURL,
// 						Image: &schema.MessageInputImage{
// 							MessagePartCommon: schema.MessagePartCommon{
// 								URL: ptrOf("a cat"),
// 							},
// 						},
// 					},
// 					{
// 						Type: schema.ChatMessagePartTypeText,
// 						Text: "a cat",
// 					},
// 					{
// 						Type: schema.ChatMessagePartTypeImageURL,
// 						Image: &schema.MessageInputImage{
// 							MessagePartCommon: schema.MessagePartCommon{
// 								Base64Data: ptrOf("a cat"),
// 								MIMEType:   "image/png",
// 							},
// 						},
// 					},
// 				},
// 			},
// 		}

// 		PatchConvey("test generate images error", func() {
// 			Mock(GetMethod(im.client, "GenerateImages")).Return(
// 				nil, errors.New("test generate error")).Build()

// 			outMsg, err := im.Generate(ctx, msgs)

// 			convey.So(err, convey.ShouldNotBeNil)
// 			convey.So(outMsg, convey.ShouldBeNil)
// 		})

// 		PatchConvey("test response with error", func() {
// 			Mock(GetMethod(im.client, "GenerateImages")).Return(
// 				model.ImagesResponse{
// 					Error: &model.GenerateImagesError{Code: "1", Message: "internal error"},
// 				}, nil).Build()

// 			outMsg, err := im.Generate(ctx, msgs)
// 			convey.So(err, convey.ShouldNotBeNil)
// 			convey.So(err.Error(), convey.ShouldContainSubstring, "internal error")
// 			convey.So(outMsg, convey.ShouldBeNil)
// 		})

// 		PatchConvey("test success", func() {
// 			testURL := "http://example.com/cat.png"
// 			Mock(GetMethod(im.client, "GenerateImages")).Return(
// 				model.ImagesResponse{
// 					Data: []*model.Image{
// 						{
// 							Url: &testURL,
// 						},
// 					},
// 					Usage: &model.GenerateImagesUsage{
// 						TotalTokens:  10,
// 						OutputTokens: 2,
// 					},
// 				}, nil).Build()

// 			outMsg, err := im.Generate(ctx, msgs)
// 			convey.So(err, convey.ShouldBeNil)
// 			convey.So(outMsg, convey.ShouldNotBeNil)
// 			convey.So(outMsg.Role, convey.ShouldEqual, schema.Assistant)
// 			convey.So(len(outMsg.AssistantGenMultiContent), convey.ShouldEqual, 1)
// 			convey.So(*outMsg.AssistantGenMultiContent[0].Image.URL, convey.ShouldEqual, testURL)
// 		})
// 	})
// }

func TestImageGenerationStream(t *testing.T) {
	PatchConvey("test ImageGeneration Stream", t, func() {
		ctx := context.Background()
		im, err := NewImageGenerationModel(ctx, &ImageGenerationConfig{
			Model:  "test-image-model",
			APIKey: "test-api-key",
		})
		convey.So(err, convey.ShouldBeNil)
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "a dog",
			},
		}

		PatchConvey("test stream creation error", func() {
			Mock(GetMethod(im.client, "GenerateImagesStreaming")).Return(
				nil, errors.New("test stream creation error")).Build()

			outStream, err := im.Stream(ctx, msgs)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(outStream, convey.ShouldBeNil)
		})

		PatchConvey("test stream recv success", func() {
			sr := &utils.ImageGenerationStreamReader{}
			Mock(GetMethod(im.client, "GenerateImagesStreaming")).Return(
				sr, nil).Build()

			times := 0
			Mock(GetMethod((*utils.ImageGenerationStreamReader)(nil), "Recv")).To(
				func() (response model.ImagesStreamResponse, err error) {
					if times >= 1 {
						return model.ImagesStreamResponse{}, io.EOF
					}
					times++
					testURL := "https://example.com/dog.png"
					return model.ImagesStreamResponse{
						Url: &testURL,
						Usage: &model.GenerateImagesUsage{
							TotalTokens: 10,
						},
					}, nil
				}).Build()

			Mock(GetMethod(sr, "Close")).Return(nil).Build()

			outStream, err := im.Stream(ctx, msgs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(outStream, convey.ShouldNotBeNil)
			defer outStream.Close()

			var receivedMsgs []*schema.Message
			for {
				item, e := outStream.Recv()
				if e != nil {
					errStr := e.Error()
					_ = errStr
					convey.So(e, convey.ShouldEqual, io.EOF)
					break
				}
				receivedMsgs = append(receivedMsgs, item)
			}

			convey.So(len(receivedMsgs), convey.ShouldEqual, 1)
			msg := receivedMsgs[0]
			convey.So(msg.Role, convey.ShouldEqual, schema.Assistant)
			convey.So(len(msg.MultiContent), convey.ShouldEqual, 1)
			convey.So(msg.MultiContent[0].ImageURL.URL, convey.ShouldEqual, "https://example.com/dog.png")
		})
	})
}
