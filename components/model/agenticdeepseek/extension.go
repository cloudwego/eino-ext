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
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func init() {
	schema.RegisterName[*ResponseMetaExtension]("_eino_ext_deepseek_response_meta_extension")
	compose.RegisterStreamChunkConcatFunc(concatResponseMetaExtensions)
}

type ResponseMetaExtension struct {
	FinishReason string           `json:"finish_reason,omitempty"`
	LogProbs     *schema.LogProbs `json:"logprobs,omitempty"`
}

func concatResponseMetaExtensions(chunks []*ResponseMetaExtension) (*ResponseMetaExtension, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}

	ret := &ResponseMetaExtension{}
	for _, ext := range chunks {
		if ext.FinishReason != "" {
			ret.FinishReason = ext.FinishReason
		}
		if ext.LogProbs != nil {
			ret.LogProbs = ext.LogProbs
		}
	}

	return ret, nil
}
