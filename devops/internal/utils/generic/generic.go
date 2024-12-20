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

func MapKeys[K comparable, V any](m map[K]V) []K {
	ret := make([]K, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}
	return ret
}

func CopySlice[T any](s []T) []T {
	ret := make([]T, len(s))
	copy(ret, s)
	return ret
}

// SliceContains returns whether the element occur in slice.
//
// 🚀 EXAMPLE:
//
//	SliceContains([]int{0, 1, 2, 3, 4}, 1) ⏩ true
//	SliceContains([]int{0, 1, 2, 3, 4}, 5) ⏩ false
//	SliceContains([]int{}, 5)              ⏩ false

func SliceContains[T comparable](s []T, v T) bool {
	for _, vv := range s {
		if v == vv {
			return true
		}
	}
	return false
}
