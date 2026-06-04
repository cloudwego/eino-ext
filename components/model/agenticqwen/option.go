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
	"github.com/cloudwego/eino/components/model"
)

type options struct {
	EnableThinking   *bool
	PreserveThinking *bool
}

// WithEnableThinking is the option to set the enable thinking for the model.
func WithEnableThinking(enableThinking bool) model.Option {
	return model.WrapImplSpecificOptFn(func(opt *options) {
		opt.EnableThinking = &enableThinking
	})
}

// WithPreserveThinking is the option to set the preserve thinking for the model.
func WithPreserveThinking(preserveThinking bool) model.Option {
	return model.WrapImplSpecificOptFn(func(opt *options) {
		opt.PreserveThinking = &preserveThinking
	})
}
