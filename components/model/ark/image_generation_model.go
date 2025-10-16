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
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	autils "github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type ImageGenerationConfig struct {
	// For authentication, APIKey is required as the image generation API only supports API Key authentication.
	// For authentication details, see: https://www.volcengine.com/docs/82379/1298459
	// Required
	APIKey string `json:"api_key"`

	// Model specifies the ID of endpoint on ark platform
	// Required
	Model string `json:"model"`

	// Timeout specifies the maximum duration to wait for API responses
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: 10 minutes
	Timeout *time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// RetryTimes specifies the number of retry attempts for failed API calls
	// Optional. Default: 2
	RetryTimes *int `json:"retry_times"`

	// BaseURL specifies the base URL for Ark service
	// Optional. Default: "https://ark.cn-beijing.volces.com/api/v3"
	BaseURL string `json:"base_url"`

	// Region specifies the region where Ark service is located
	// Optional. Default: "cn-beijing"
	Region string `json:"region"`

	// The following fields correspond to Ark's image generation API parameters
	// Ref: https://www.volcengine.com/docs/82379/1541523

	// Size specifies the dimensions of the generated image.
	// It can be a resolution keyword (e.g., "1K", "2K", "4K") or a custom resolution
	// in "{width}x{height}" format (e.g., "1920x1080").
	// When using custom resolutions, the total pixels must be between 1280x720 and 4096x4096,
	// and the aspect ratio (width/height) must be between 1/16 and 16.
	// Optional. Defaults to "2048x2048".
	Size string `json:"size"`

	// SequentialImageGeneration determines if the model should generate a sequence of images.
	// Possible values:
	//  - "auto": The model decides whether to generate multiple images based on the prompt.
	//  - "disabled": Only a single image is generated.
	// Optional. Defaults to "disabled".
	SequentialImageGeneration SequentialImageGeneration `json:"sequential_image_generation"`

	// SequentialImageGenerationOption sets the maximum number of images to generate when
	// SequentialImageGeneration is set to "auto".
	// The value must be between 1 and 15.
	// Optional. Defaults to 15.
	SequentialImageGenerationOption *model.SequentialImageGenerationOptions `json:"sequential_image_generation_option"`

	// ResponseFormat specifies how the generated image data is returned.
	// Possible values:
	//  - "url": A temporary URL to download the image (valid for 24 hours).
	//  - "b64_json": The image data encoded as a Base64 string in the response.
	// Optional. Defaults to "url".
	ResponseFormat ImageResponseFormat `json:"response_format"`

	// DisableWatermark, if set to true, removes the "AI Generated" watermark
	// from the bottom-right corner of the image.
	// Optional. Defaults to false.
	DisableWatermark bool `json:"disable_watermark"`
}

type ImageGenerationModel struct {
	client *arkruntime.Client

	model                           string
	size                            *string
	sequentialImageGeneration       SequentialImageGeneration
	sequentialImageGenerationOption *model.SequentialImageGenerationOptions
	responseFormat                  ImageResponseFormat
	disableWatermark                bool
}

type ImageResponseFormat string

const (
	ImageResponseFormatURL ImageResponseFormat = "url"
	ImageResponseFormatB64 ImageResponseFormat = "b64_json"
)

type SequentialImageGeneration string

const (
	SequentialImageGenerationDisabled SequentialImageGeneration = "disabled"
	SequentialImageGenerationAuto     SequentialImageGeneration = "auto"
)

func NewImageGenerationModel(_ context.Context, config *ImageGenerationConfig) (*ImageGenerationModel, error) {
	if config == nil {
		return nil, fmt.Errorf("image generation model requires config")
	}

	imageModel, err := buildImageGenerationModel(config)
	if err != nil {
		return nil, err
	}
	return imageModel, nil
}

func buildImageGenerationModel(config *ImageGenerationConfig) (*ImageGenerationModel, error) {
	baseURL := defaultBaseURL
	if config.BaseURL != "" {
		baseURL = config.BaseURL
	}
	region := defaultRegion
	if config.Region != "" {
		region = config.Region
	}
	timeout := defaultTimeout
	if config.Timeout != nil {
		timeout = *config.Timeout
	}
	retryTimes := defaultRetryTimes
	if config.RetryTimes != nil {
		retryTimes = *config.RetryTimes
	}

	opts := []arkruntime.ConfigOption{
		arkruntime.WithRetryTimes(retryTimes),
		arkruntime.WithBaseUrl(baseURL),
		arkruntime.WithRegion(region),
		arkruntime.WithTimeout(timeout),
	}
	if config.HTTPClient != nil {
		opts = append(opts, arkruntime.WithHTTPClient(config.HTTPClient))
	}

	var client *arkruntime.Client
	if len(config.APIKey) > 0 {
		client = arkruntime.NewClientWithApiKey(config.APIKey, opts...)
	} else {
		return nil, fmt.Errorf("image generation model requires APIKey")
	}

	var (
		seq    SequentialImageGeneration
		seqOpt *model.SequentialImageGenerationOptions
	)

	if config.SequentialImageGeneration == "" {
		seq = SequentialImageGenerationDisabled
	} else {
		seq = config.SequentialImageGeneration
	}

	if seq == SequentialImageGenerationDisabled {
		seqOpt = nil
	}

	responseFormat := config.ResponseFormat
	if responseFormat == "" {
		responseFormat = ImageResponseFormatURL
	}

	disableWatermark := config.DisableWatermark

	size := config.Size
	if size == "" {
		size = "2048x2048"
	}

	return &ImageGenerationModel{
		client:                          client,
		model:                           config.Model,
		size:                            &size,
		sequentialImageGeneration:       seq,
		sequentialImageGenerationOption: seqOpt,
		responseFormat:                  responseFormat,
		disableWatermark:                disableWatermark,
	}, nil
}

func (im *ImageGenerationModel) Generate(ctx context.Context, in []*schema.Message, opts ...einoModel.Option) (outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, getType(), components.ComponentOfChatModel)

	options := einoModel.GetCommonOptions(&einoModel.Options{
		Model: &im.model,
	}, opts...)

	req, err := im.genRequest(in, options)
	if err != nil {
		return nil, err
	}

	reqConf := &einoModel.Config{
		Model: req.Model,
	}

	ctx = callbacks.OnStart(ctx, &einoModel.CallbackInput{
		Messages: in,
		Config:   reqConf,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	resp, err := im.client.GenerateImages(ctx, *req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("image generation failed, errCode: %v, errMsg: %v", resp.Error.Code, resp.Error.Message)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("image generation failed, image data is empty")
	}
	// imageParts := make([]schema.MessageOutputPart, 0, len(resp.Data))
	// for _, image := range resp.Data {
	// 	img := &schema.MessageOutputImage{
	// 		ConcatMode: schema.Full,
	// 		MIMEType:   "image/jpeg",
	// 	}
	// 	if image.Url != nil {
	// 		img.URL = image.Url
	// 	} else if image.B64Json != nil {
	// 		img.Base64Data = image.B64Json
	// 	} else {
	// 		continue // Skip if no image data is found
	// 	}

	// 	// Set additional info
	// 	SetImageSize(img, image.Size)

	// 	imageParts = append(imageParts, schema.MessageOutputPart{
	// 		Type:  schema.ChatMessagePartTypeImageURL,
	// 		Image: img,
	// 	})
	// }

	imageURLs := make([]schema.ChatMessagePart, 0, len(resp.Data))
	for _, image := range resp.Data {
		// The `response_format` field in the API documentation specifies that the model currently only returns images in jpeg format.
		// Ref: https://www.volcengine.com/docs/82379/1541523
		imageURL := &schema.ChatMessageImageURL{
			MIMEType: "image/jpeg",
		}
		if image.Url != nil {
			imageURL.URL = *image.Url
		} else if image.B64Json != nil {
			imageURL.URL = *image.B64Json
		} else {
			continue // Skip if no image data is found
		}

		// Set additional info
		SetImageSize(imageURL, image.Size)

		imageURLs = append(imageURLs, schema.ChatMessagePart{
			Type:     schema.ChatMessagePartTypeImageURL,
			ImageURL: imageURL,
		})
	}

	outMsg = &schema.Message{
		Role:         schema.Assistant,
		MultiContent: imageURLs,
	}

	callbacks.OnEnd(ctx, &einoModel.CallbackOutput{
		Message:    outMsg,
		Config:     reqConf,
		TokenUsage: im.toTokenUsage(resp.Usage),
	})
	return outMsg, nil
}

func (im *ImageGenerationModel) Stream(ctx context.Context, in []*schema.Message, opts ...einoModel.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, getType(), components.ComponentOfChatModel)

	options := einoModel.GetCommonOptions(&einoModel.Options{
		Model: &im.model,
	}, opts...)

	req, err := im.genRequest(in, options)
	if err != nil {
		return nil, err
	}

	reqConf := &einoModel.Config{
		Model: req.Model,
	}

	ctx = callbacks.OnStart(ctx, &einoModel.CallbackInput{
		Messages: in,
		Config:   reqConf,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	stream, err := im.client.GenerateImagesStreaming(ctx, *req)
	if err != nil {
		return nil, err
	}

	sr, sw := schema.Pipe[*einoModel.CallbackOutput](1)
	go func() {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				_ = sw.Send(nil, newPanicErr(panicErr, debug.Stack()))
			}

			sw.Close()
			_ = im.closeArkStreamReader(stream)
		}()

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				_ = sw.Send(nil, err)
				return
			}

			msg, msgFound, e := im.resolveStreamResponse(resp)
			if e != nil {
				_ = sw.Send(nil, e)
				return
			}

			if !msgFound {
				continue
			}

			closed := sw.Send(&einoModel.CallbackOutput{
				Message:    msg,
				Config:     reqConf,
				TokenUsage: im.toTokenUsage(resp.Usage),
			}, nil)
			if closed {
				return
			}
		}
	}()

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr, func(src *einoModel.CallbackOutput) (callbacks.CallbackOutput, error) {
		return src, nil
	}))

	outStream = schema.StreamReaderWithConvert(nsr,
		func(src callbacks.CallbackOutput) (*schema.Message, error) {
			s := src.(*einoModel.CallbackOutput)
			if s.Message == nil {
				return nil, schema.ErrNoValue
			}

			return s.Message, nil
		},
	)

	return outStream, nil
}

func (im *ImageGenerationModel) genRequest(in []*schema.Message, options *einoModel.Options) (req *model.GenerateImagesRequest, err error) {
	req = &model.GenerateImagesRequest{
		Model:     dereferenceOrZero(options.Model),
		Size:      im.size,
		Watermark: ptrOf(!im.disableWatermark),
	}

	if im.sequentialImageGeneration == SequentialImageGenerationAuto {
		req.SequentialImageGeneration = ptrOf(model.SequentialImageGeneration(im.sequentialImageGeneration))
		req.SequentialImageGenerationOptions = im.sequentialImageGenerationOption
	}

	if im.responseFormat != "" {
		req.ResponseFormat = ptrOf(string(im.responseFormat))
	}

	prompt, images, err := toPromptAndImages(in)
	if err != nil {
		return nil, err
	}
	req.Prompt = prompt
	if images != nil {
		req.Image = images
	}

	return req, nil
}

func toPromptAndImages(in []*schema.Message) (prompt string, images interface{}, err error) {
	var promptBuilder strings.Builder
	var imageURLs = make([]string, 0, len(in))
	for _, msg := range in {
		if msg.Role == schema.System || msg.Role == schema.User {
			if len(msg.Content) > 0 {
				if promptBuilder.Len() > 0 {
					promptBuilder.WriteByte('\n')
				}
				promptBuilder.WriteString(msg.Content)
			}
			// if len(msg.UserInputMultiContent) > 0 {
			// 	for _, part := range msg.UserInputMultiContent {
			// 		if part.Type == schema.ChatMessagePartTypeImageURL {
			// 			if part.Image != nil && part.Image.URL != nil {
			// 				imageURLs = append(imageURLs, *part.Image.URL)
			// 			} else if part.Image != nil && part.Image.Base64Data != nil {
			// 				imageURLs = append(imageURLs, ensureImageDataURL(*part.Image.Base64Data, part.Image.MIMEType))
			// 			}
			// 		} else if part.Type == schema.ChatMessagePartTypeText {
			// 			if len(part.Text) > 0 {
			// 				if promptBuilder.Len() > 0 {
			// 					promptBuilder.WriteByte(' ')
			// 				}
			// 				promptBuilder.WriteString(part.Text)
			// 			}
			// 		}
			// 	}
			// }
			if len(msg.MultiContent) > 0 {
				for _, part := range msg.MultiContent {
					if part.Type == schema.ChatMessagePartTypeImageURL {
						if part.ImageURL != nil {
							imageURLs = append(imageURLs, ensureImageDataURL(part.ImageURL.URL, part.ImageURL.MIMEType))
						}
					} else if part.Type == schema.ChatMessagePartTypeText {
						if len(part.Text) > 0 {
							promptBuilder.WriteString(part.Text)
						}
					}
				}
			}
		} else {
			return "", nil, fmt.Errorf("image generation model only support user and system message, but got %v", msg.Role)
		}
	}
	prompt = promptBuilder.String()
	if len(imageURLs) == 1 {
		images = imageURLs[0]
	} else if len(imageURLs) > 1 {
		images = imageURLs
	}

	return prompt, images, nil
}

func ensureImageDataURL(data, mimeType string) string {
	switch {
	case data == "":
		return ""
	case strings.HasPrefix(data, "data:"):
		return data
	case strings.HasPrefix(data, "http"):
		return data
	case mimeType == "":
		return data
	}

	return fmt.Sprintf("data:%s;base64,%s", mimeType, data)
}

func (im *ImageGenerationModel) resolveStreamResponse(resp model.ImagesStreamResponse) (*schema.Message, bool, error) {
	if resp.Error != nil {
		return nil, false, fmt.Errorf("image generation failed, errCode: %v, errMsg: %v", resp.Error.Code, resp.Error.Message)
	}

	// image := &schema.MessageOutputImage{
	// 	ConcatMode: schema.Full,
	// 	MIMEType:   "image/jpeg",
	// }
	// if resp.Url != nil {
	// 	image.URL = resp.Url
	// } else if resp.B64Json != nil {
	// 	image.Base64Data = resp.B64Json
	// } else {
	// 	return nil, false, nil
	// }

	// SetImageSize(image, resp.Size)

	// return &schema.Message{
	// 	Role: schema.Assistant,
	// 	AssistantGenMultiContent: []schema.MessageOutputPart{
	// 		{
	// 			Type:  schema.ChatMessagePartTypeImageURL,
	// 			Image: image,
	// 		},
	// 	},
	// }, true, nil

	// The `response_format` field in the API documentation specifies that the model currently only returns images in jpeg format.
	// Ref: https://www.volcengine.com/docs/82379/1541523
	imageURL := &schema.ChatMessageImageURL{
		MIMEType: "image/jpeg",
	}

	if resp.Url != nil {
		imageURL.URL = *resp.Url
	} else if resp.B64Json != nil {
		imageURL.URL = *resp.B64Json
	} else {
		return nil, false, nil
	}

	SetImageSize(imageURL, resp.Size)

	return &schema.Message{
		Role: schema.Assistant,
		MultiContent: []schema.ChatMessagePart{
			{
				Type:     schema.ChatMessagePartTypeImageURL,
				ImageURL: imageURL,
			},
		},
	}, true, nil
}

func (im *ImageGenerationModel) closeArkStreamReader(r *autils.ImageGenerationStreamReader) error {
	if r == nil || r.Response == nil || r.Response.Body == nil {
		return nil
	}
	return r.Close()
}

func (im *ImageGenerationModel) toTokenUsage(usage *model.GenerateImagesUsage) *einoModel.TokenUsage {
	if usage == nil {
		return nil
	}
	return &einoModel.TokenUsage{
		PromptTokens:     int(usage.TotalTokens) - int(usage.OutputTokens),
		CompletionTokens: int(usage.OutputTokens),
		TotalTokens:      int(usage.TotalTokens),
	}
}
