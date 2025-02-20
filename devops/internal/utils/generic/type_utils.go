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

package generic

import (
	"reflect"
	"strings"
)

func GetJsonName(field reflect.StructField) string {
	tag, ok := field.Tag.Lookup("json")
	if !ok {
		return field.Name
	}
	tagList := strings.Split(tag, ",")
	return tagList[0]
}

func HasRequired(field reflect.StructField) bool {
	tag, ok := field.Tag.Lookup("binding")
	if !ok {
		return false
	}
	return SliceContains(strings.Split(tag, ","), "required")
}

func IsMapType[K, V any](t reflect.Type) bool {
	if t.Kind() != reflect.Map {
		return false
	}
	if t.Key().Kind() != typeOf[K]().Kind() {
		return false
	}
	if t.Elem().Kind() != typeOf[V]().Kind() {
		return false
	}
	return true
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

var comfortableKind = map[reflect.Kind]bool{
	reflect.String:  true,
	reflect.Bool:    true,
	reflect.Int:     true,
	reflect.Int8:    true,
	reflect.Int16:   true,
	reflect.Int32:   true,
	reflect.Int64:   true,
	reflect.Uint:    true,
	reflect.Uint8:   true,
	reflect.Uint16:  true,
	reflect.Uint32:  true,
	reflect.Uint64:  true,
	reflect.Float32: true,
	reflect.Float64: true,
}

func ComfortableKind(kind reflect.Kind) bool {
	return comfortableKind[kind]
}

var unsupportedInputKind = map[reflect.Kind]bool{
	reflect.Invalid:       true,
	reflect.Complex64:     true,
	reflect.Complex128:    true,
	reflect.Chan:          true,
	reflect.Func:          true,
	reflect.UnsafePointer: true,
}

func UnsupportedInputKind(kind reflect.Kind) bool {
	return unsupportedInputKind[kind]
}

func ValidateInputReflectTypeSupported(typ reflect.Type) (supported bool) {
	if typ.Kind() == reflect.Pointer {
		return ValidateInputReflectTypeSupported(typ.Elem())
	}

	switch typ.Kind() {
	case reflect.Map:
		if !ValidateInputReflectTypeSupported(typ.Key()) {
			return false
		}
		return ValidateInputReflectTypeSupported(typ.Elem())

	case reflect.Slice, reflect.Array:
		return ValidateInputReflectTypeSupported(typ.Elem())

	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.IsExported() {
				if ValidateInputReflectTypeSupported(field.Type) {
					return true
				}
			}
		}
		return false

	default:
		if comfortableKind[typ.Kind()] {
			return true
		}
		return false
	}
}

func TypeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}
