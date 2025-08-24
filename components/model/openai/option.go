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

package openai

import (
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
)

// WithExtraFields is used to set extra body fields for the request.
func WithExtraFields(extraFields map[string]any) model.Option {
	return openai.WithExtraFields(extraFields)
}

// WithExtraHeader is used to set extra headers for the request.
func WithExtraHeader(header map[string]string) model.Option {
	return openai.WithExtraHeader(header)
}

func WithReasoningEffort(effort ReasoningEffortLevel) model.Option {
	return openai.WithReasoningEffort(openai.ReasoningEffortLevel(effort))
}
