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

package devops

import (
	"reflect"

	"github.com/cloudwego/eino-ext/devops/internal/model"
)

// WithDevServerPort sets dev server port, default to 52538
func WithDevServerPort(port string) model.DevOption {
	return func(o *model.DevOpt) {
		o.DevServerPort = port
	}
}

// AppendType registers a concrete type that can be chosen as an implementation of an interface
// during mock debugging input in the Eino Dev plugin. The identifier is the type.String() value,
// and some generic types are also registered in github.com/cloudwego/eino-ext/devops/internal/model/types.go:registeredTypes,
// e.g.,
// `*schema.Message`, `schema.Message`, `[]*schema.Message`, `map[string]interface {}`.
//
// Example:
//
//	AppendType(&MyConcreteType{}) // Registers MyConcreteType as an option for interfaces it implements.
func AppendType(value any) model.DevOption {
	return func(o *model.DevOpt) {
		rt := reflect.TypeOf(value)
		o.GoTypes = append(o.GoTypes, model.RegisteredType{
			Identifier: rt.String(),
			Type:       rt,
		})
	}
}
