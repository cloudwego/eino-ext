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

package qianfan

import (
	"github.com/cloudwego/eino/schema"
)

const (
	minFps = 0.2
	maxFps = 5
)

const keyOfQianFanVideoFPS = "qianfan_video_fps"

// GetMessagePartVideoFPS extracts the video frames-per-second (FPS) from a message part.
// FPS determines the number of images to extract from a video per second.
// you can view the corresponding FPS value range in the document https://cloud.baidu.com/doc/qianfan-api/s/rm7u7qdiq
func GetMessagePartVideoFPS(part schema.MessagePartCommon) (float64, bool) {
	if part.Extra == nil {
		return 0, false
	}

	fps, ok := part.Extra[keyOfQianFanVideoFPS].(float64)
	if !ok {
		return 0, false
	}
	return fps, true

}

func SetMessageInputVideoFPS(part *schema.MessageInputVideo, fps float64) {
	if part.Extra == nil {
		part.Extra = make(map[string]interface{})
	}
	part.Extra[keyOfQianFanVideoFPS] = fps

}
