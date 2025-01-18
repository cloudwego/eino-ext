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

package model

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/compose"
)

type Runnable struct {
	r compose.Runnable[any, any]
}

func (dr Runnable) Invoke(ctx context.Context, input reflect.Value, opts ...compose.Option) (output any, err error) {
	callArgs := make([]reflect.Value, 0, len(opts))
	callArgs = append(callArgs, reflect.ValueOf(ctx), input)
	for _, opt := range opts {
		callArgs = append(callArgs, reflect.ValueOf(opt))
	}

	res := reflect.ValueOf(dr.r).MethodByName("Invoke").Call(callArgs)
	if !res[1].IsNil() {
		return nil, res[1].Interface().(error)
	}
	if res[0].IsNil() {
		return nil, fmt.Errorf("output is nil")
	}

	return res[0].Interface(), nil
}

func getPtrValue(typ reflect.Value, level int) reflect.Value {
	for i := 0; i < level; i++ {
		newInput := reflect.New(typ.Type())
		newInput.Elem().Set(typ)
		typ = newInput
	}
	return typ
}
