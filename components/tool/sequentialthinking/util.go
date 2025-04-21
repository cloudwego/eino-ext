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

package sequentialthinking

import "strings"

// padEnd pads the end of a string with spaces until it reaches the specified length.
// If the string is already longer than or equal to the specified length, it returns the original string.
// Parameters:
//   - str: The string to pad
//   - length: The target length of the resulting string
//
// Returns: The padded string
func padEnd(str string, length int) string {
	return str + strings.Repeat(" ", max(0, length-len(str)))
}

// getKeys extracts all keys from a map and returns them as a slice of strings.
// Parameters:
//   - branches: A map with string keys and []*ThoughtRequest values
//
// Returns: A slice containing all the keys from the input map
func getKeys(branches map[string][]*ThoughtRequest) []string {
	keys := make([]string, 0, len(branches))
	for k := range branches {
		keys = append(keys, k)
	}
	return keys
}
