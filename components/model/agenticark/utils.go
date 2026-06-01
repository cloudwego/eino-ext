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

package agenticark

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/eino-contrib/jsonschema"
)

func ptrIfNonZero[T any](v T) *T {
	if reflect.ValueOf(v).IsZero() {
		return nil
	}
	return &v
}

func coalesce[T any](x, y T) T {
	if !reflect.ValueOf(x).IsZero() {
		return x
	}
	return y
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

func int64ToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic: %v\nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}

func jsonschemaToMap(sc *jsonschema.Schema) (map[string]any, error) {
	if sc == nil {
		return nil, fmt.Errorf("JSON schema is nil")
	}

	val := reflect.ValueOf(sc)
	val = val.Elem()

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %v", val.Kind())
	}

	typ := val.Type()
	result := make(map[string]any)

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		if field.Name == "Extra" && !fieldValue.IsZero() {
			return nil, fmt.Errorf("'Extra' field must be nil")
		}

		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		tagParts := strings.Split(jsonTag, ",")
		keyName := tagParts[0]

		omitempty := false
		for _, opt := range tagParts[1:] {
			if opt == "omitempty" {
				omitempty = true
				break
			}
		}

		if omitempty && fieldValue.IsZero() {
			continue
		}

		result[keyName] = fieldValue.Interface()
	}

	if sc.Extras != nil {
		for k, v := range sc.Extras {
			if _, ok := result[k]; ok {
				return nil, fmt.Errorf("extra field %q is duplicated", k)
			}
			result[k] = v
		}
	}

	if sc.Type != "" {
		result["type"] = sc.Type
	} else if sc.TypeEnhanced != nil {
		result["type"] = sc.TypeEnhanced
	}

	return result, nil
}
