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

package sqlitevec

import "github.com/cloudwego/eino/components/retriever"

type implOptions struct {
	MaxDistance *float64
}

// WithMaxDistance filters retrieved documents whose raw sqlite-vec distance is greater than maxDistance.
func WithMaxDistance(maxDistance float64) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *implOptions) {
		o.MaxDistance = &maxDistance
	})
}
