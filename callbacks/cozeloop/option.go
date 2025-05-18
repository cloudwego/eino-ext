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

package cozeloop

import (
	"reflect"

	"github.com/coze-dev/cozeloop-go"
)

type EinoVersionFn func() string

type options struct {
	enableTracing    bool
	parser           CallbackDataParser
	logger           cozeloop.Logger
	einoVersionFn    EinoVersionFn
	concatFuncs      map[reflect.Type]any
	enableAggrOutput bool
}

type Option func(o *options)

func WithEnableTracing(enable bool) Option {
	return func(o *options) {
		o.enableTracing = enable
	}
}

func WithCallbackDataParser(parser CallbackDataParser) Option {
	return func(o *options) {
		o.parser = parser
	}
}

func WithLogger(logger cozeloop.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

func WithEinoVersionFn(fn EinoVersionFn) Option {
	return func(o *options) {
		o.einoVersionFn = fn
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
