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

package ark

import (
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func setOptionalCount(msg *schema.Message, key string, value int64, present bool) {
	if present {
		msg.Extra[key] = metadataInt64(value)
	} else {
		delete(msg.Extra, key)
	}
}

func toModelCallbackUsage(meta *schema.ResponseMeta) *fmodel.TokenUsage {
	if meta == nil || meta.Usage == nil {
		return nil
	}
	u := meta.Usage
	return &fmodel.TokenUsage{PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens, TotalTokens: u.TotalTokens,
		PromptTokenDetails:      fmodel.PromptTokenDetails{CachedTokens: u.PromptTokenDetails.CachedTokens},
		CompletionTokensDetails: fmodel.CompletionTokensDetails{ReasoningTokens: u.CompletionTokensDetails.ReasoningTokens},
	}
}
