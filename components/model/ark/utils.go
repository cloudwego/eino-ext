/*
 * Copyright 2024 CloudWeGo Authors
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
	"fmt"

	"github.com/openai/openai-go/packages/param"
)

const typ = "Ark"

func getType() string {
	return typ
}

func dereferenceOrZero[T any](v *T) T {
	if v == nil {
		var t T
		return t
	}

	return *v
}

func ptrFromOrZero[T any](v *T) T {
	if v == nil {
		var t T
		return t
	}
	return *v
}

func ptrOf[T any](v T) *T {
	return &v
}

func newOpenaiIntOpt(optVal *int) param.Opt[int64] {
	if optVal == nil {
		return param.Opt[int64]{}
	}
	return param.NewOpt(int64(*optVal))
}

func newOpenaiFloatOpt(optVal *float32) param.Opt[float64] {
	if optVal == nil {
		return param.Opt[float64]{}
	}
	return param.NewOpt(float64(*optVal))
}

func newOpenaiStringOpt(optVal *string) param.Opt[string] {
	if optVal == nil {
		return param.Opt[string]{}
	}
	return param.NewOpt(*optVal)
}

func newOpenaiBoolOpt(optVal *bool) param.Opt[bool] {
	if optVal == nil {
		return param.Opt[bool]{}
	}
	return param.NewOpt(*optVal)
}

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}
