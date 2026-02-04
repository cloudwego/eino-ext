/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/openai/openai-go/v3/packages/param"
)

func newOpenaiOpt[T comparable](optVal *T) param.Opt[T] {
	if optVal == nil {
		return param.Opt[T]{}
	}
	return param.NewOpt(*optVal)
}

func newOpenaiStrOpt(optVal string) param.Opt[string] {
	if optVal == "" {
		return param.Opt[string]{}
	}
	return param.NewOpt(optVal)
}

func coalesce[T any](x, y T) T {
	if !reflect.ValueOf(x).IsZero() {
		return x
	}
	return y
}

func ptrOf[T any](v T) *T {
	return &v
}

func int64ToStr(i int64) string {
	return strconv.FormatInt(i, 10)
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
