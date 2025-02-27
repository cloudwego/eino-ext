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

package internal

import "fmt"

var (
	// ErrPageNotFound is returned when the requested page is not found.
	ErrPageNotFound = fmt.Errorf("page not found")
	// ErrInvalidParameters is returned when the request parameters are invalid.
	ErrInvalidParameters = fmt.Errorf("invalid parameters")
	// ErrTooManyRedirects is returned when too many redirects are followed.
	ErrTooManyRedirects = fmt.Errorf("too many redirects")
)

// APIError represents an error returned by the Wikipedia API.
type APIError struct {
	Code   string `json:"code"`
	Info   string `json:"info"`
	DocRef string `json:"docref"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %s (code: %s)", e.Info, e.Code)
}
