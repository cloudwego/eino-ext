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

package volc_vikingdb

import (
	"encoding/json"
	"fmt"
)

func GetType() string {
	return typ
}

func tryMarshalJsonString(input any) string {
	if b, err := json.Marshal(input); err == nil {
		return string(b)
	}

	return ""
}

func interfaceTof64Slice(raw interface{}) ([]float64, error) {
	rawSlice, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("raw type not []interface, raw=%v", raw)
	}

	resp := make([]float64, len(rawSlice))
	for i := range rawSlice {
		var (
			f64 float64
			err error
		)

		switch v := rawSlice[i].(type) {
		case float64:
			f64 = v
		case json.Number:
			f64, err = v.Float64()
		default:
			return nil, fmt.Errorf("item[%d] unexpected type, item=%v, raw slice=%v", i, rawSlice[i], raw)
		}

		if err != nil {
			return nil, err
		}

		resp[i] = f64
	}

	return resp, nil
}

func dereferenceOrZero[T any](v *T) T {
	if v == nil {
		var t T
		return t
	}

	return *v
}

func ptrOf[T any](v T) *T {
	return &v
}
