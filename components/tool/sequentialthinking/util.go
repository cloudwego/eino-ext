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
//   - branches: A map with string keys and []*thoughtData values
//
// Returns: A slice containing all the keys from the input map
func getKeys(branches map[string][]*thoughtData) []string {
	keys := make([]string, 0, len(branches))
	for k := range branches {
		keys = append(keys, k)
	}
	return keys
}
