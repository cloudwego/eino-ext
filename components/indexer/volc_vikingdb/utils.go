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

func chunk[T any](slice []T, size int) [][]T {
	if size <= 0 {
		return nil
	}

	var chunks [][]T
	for size < len(slice) {
		slice, chunks = slice[size:], append(chunks, slice[0:size:size])
	}

	if len(slice) > 0 {
		chunks = append(chunks, slice)
	}

	return chunks
}

func iter[T, D any](src []T, fn func(T) D) []D {
	resp := make([]D, len(src))
	for i := range src {
		resp[i] = fn(src[i])
	}

	return resp
}

func iterWithErr[T, D any](src []T, fn func(T) (D, error)) ([]D, error) {
	resp := make([]D, 0, len(src))
	for i := range src {
		d, err := fn(src[i])
		if err != nil {
			return nil, err
		}

		resp = append(resp, d)
	}

	return resp, nil
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

func interfaceToSparse(raw interface{}) (map[string]interface{}, error) {
	sparse, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("raw type not map[string]interface{}, raw=%v", raw)
	}

	return sparse, nil
}
