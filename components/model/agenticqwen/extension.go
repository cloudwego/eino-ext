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

package agenticqwen

import (
	"context"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

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
		if ext == nil {
			continue
		}
		if ext.FinishReason != "" {
			ret.FinishReason = ext.FinishReason
		}
		if ext.LogProbs != nil {
			if ret.LogProbs == nil {
				ret.LogProbs = &schema.LogProbs{}
			}
			ret.LogProbs.Content = append(ret.LogProbs.Content, ext.LogProbs.Content...)
		}
	}

	return ret, nil
}

const extraKeyResponseMetaExtension = "_eino_ext_agenticqwen_response_meta_ext"

func applyResponseMetaExtension(msg *schema.Message) {
	if msg != nil && msg.ResponseMeta != nil {
		setMsgExtra(msg, extraKeyResponseMetaExtension, &ResponseMetaExtension{
			FinishReason: msg.ResponseMeta.FinishReason,
			LogProbs:     msg.ResponseMeta.LogProbs,
		})
	}
}

func responseMetaModifier() model.Option {
	return openai.WithResponseMessageModifier(
		func(ctx context.Context, msg *schema.Message, rawBody []byte) (*schema.Message, error) {
			applyResponseMetaExtension(msg)
			return msg, nil
		},
	)
}

func responseMetaChunkModifier() model.Option {
	return openai.WithResponseChunkMessageModifier(
		func(ctx context.Context, msg *schema.Message, rawBody []byte, end bool) (*schema.Message, error) {
			applyResponseMetaExtension(msg)
			return msg, nil
		},
	)
}

func extractResponseMetaExtension(out *schema.AgenticMessage) {
	if out.Extra == nil {
		return
	}
	ext, ok := out.Extra[extraKeyResponseMetaExtension].(*ResponseMetaExtension)
	if !ok {
		return
	}
	if out.ResponseMeta == nil {
		out.ResponseMeta = &schema.AgenticResponseMeta{}
	}
	out.ResponseMeta.Extension = ext
}

func setMsgExtra(msg *schema.Message, key string, value any) {
	if msg.Extra == nil {
		msg.Extra = make(map[string]any)
	}
	msg.Extra[key] = value
}
