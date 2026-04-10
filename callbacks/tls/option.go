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

package tls

import "reflect"

type options struct {
	parser           CallbackDataParser
	concatFuncs      map[reflect.Type]any
	enableAggrOutput bool
}

type Option func(*options)

func newOptions(opts ...Option) *options {
	o := &options{
		enableAggrOutput: true,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func WithCallbackDataParser(parser CallbackDataParser) Option {
	return func(o *options) {
		o.parser = parser
	}
}

func WithConcatFunction[T any](fn func([]T) (T, error)) Option {
	return func(o *options) {
		if o.concatFuncs == nil {
			o.concatFuncs = make(map[reflect.Type]any)
		}

		o.concatFuncs[reflect.TypeOf((*T)(nil)).Elem()] = fn
	}
}

func WithAggrMessageOutput(enable bool) Option {
	return func(o *options) {
		o.enableAggrOutput = enable
	}
}
