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

package pinecone

func f64To32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, f := range f64 {
		f32[i] = float32(f)
	}

	return f32
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
