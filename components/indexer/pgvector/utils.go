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

package pgvector

import (
	"fmt"
)

// chunk splits a slice into chunks of the specified size.
// This is useful for batch processing to avoid overwhelming the database.
// Example: chunk([1,2,3,4,5], 2) -> [[1,2], [3,4], [5]]
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

// iter transforms a slice using the provided function.
// This is useful for mapping one type to another efficiently.
// Example: iter([]int{1,2,3}, func(i int) string { return strconv.Itoa(i) })
func iter[T, D any](src []T, fn func(T) D) []D {
	resp := make([]D, len(src))
	for i := range src {
		resp[i] = fn(src[i])
	}
	return resp
}

// iterWithErr transforms a slice using the provided function that may return an error.
// Returns error immediately if any transformation fails.
func iterWithErr[T, D any](src []T, fn func(T) (D, error)) ([]D, error) {
	resp := make([]D, 0, len(src))
	for i := range src {
		d, err := fn(src[i])
		if err != nil {
			return nil, fmt.Errorf("iterWithErr failed at index %d: %w", i, err)
		}
		resp = append(resp, d)
	}
	return resp, nil
}
