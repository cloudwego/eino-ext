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

package ddgsearch

import (
	"errors"
	"fmt"
)

// SearchError represents an error that occurred during a search operation.
// It wraps the original error and provides additional context.
type SearchError struct {
	Message string // Human readable error message
	Err     error  // Original error
}

func (e *SearchError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *SearchError) Unwrap() error {
	return e.Err
}

// NewSearchError creates a new SearchError with the given message and error.
func NewSearchError(message string, err error) error {
	return &SearchError{Message: message, Err: err}
}

// Common errors returned by the library
var (
	// ErrRateLimit is returned when DuckDuckGo rate limits the request.
	// This typically happens when too many requests are made in a short period.
	ErrRateLimit = &SearchError{Message: "rate limit exceeded"}

	// ErrTimeout is returned when a request to DuckDuckGo times out.
	// This can happen due to network issues or server-side delays.
	ErrTimeout = &SearchError{Message: "request timeout"}

	// ErrNoResults is returned when the search yields no results.
	// This can happen with very specific queries or when DuckDuckGo has no matching content.
	ErrNoResults = &SearchError{Message: "no results found"}

	// ErrInvalidResponse is returned when the response from DuckDuckGo cannot be parsed.
	// This can happen if the API changes or returns an unexpected format.
	ErrInvalidResponse = &SearchError{Message: "invalid response from DuckDuckGo"}

	// ErrInvalidRegion is returned when an unsupported region code is provided.
	// Use one of the predefined Region constants.
	ErrInvalidRegion = &SearchError{Message: "invalid region code"}

	// ErrInvalidSafeSearch is returned when an unsupported safe search level is provided.
	// Use one of the predefined SafeSearch constants.
	ErrInvalidSafeSearch = &SearchError{Message: "invalid safe search level"}

	// ErrInvalidTimeRange is returned when an unsupported time range is provided.
	// Use one of the predefined TimeRange constants.
	ErrInvalidTimeRange = &SearchError{Message: "invalid time range"}
)

// IsRateLimitErr checks if the error is a rate limit error.
//
// Example:
//
//	if IsRateLimitErr(err) {
//		time.Sleep(time.Second * 5)
//		// retry request
//	}
func IsRateLimitErr(err error) bool {
	var searchErr *SearchError
	if err == nil {
		return false
	}
	if ok := errors.As(err, &searchErr); ok {
		return searchErr == ErrRateLimit
	}
	return false
}

// IsTimeoutErr checks if the error is a timeout error.
//
// Example:
//
//	if IsTimeoutErr(err) {
//		// increase timeout and retry
//		client.SetTimeout(30 * time.Second)
//	}
func IsTimeoutErr(err error) bool {
	var searchErr *SearchError
	if err == nil {
		return false
	}
	if ok := errors.As(err, &searchErr); ok {
		return searchErr == ErrTimeout
	}
	return false
}

// IsNoResultsErr checks if the error indicates no results were found.
//
// Example:
//
//	if IsNoResultsErr(err) {
//		fmt.Println("No results found, try different keywords")
//	}
func IsNoResultsErr(err error) bool {
	var searchErr *SearchError
	if err == nil {
		return false
	}
	if ok := errors.As(err, &searchErr); ok {
		return searchErr == ErrNoResults
	}
	return false
}
