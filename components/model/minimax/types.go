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

package minimax

import (
	"fmt"
	"net/http"
)

type Config struct {
	APIKey string

	BaseURL *string

	Model string

	MaxTokens int

	Temperature *float32

	TopP *float32

	HTTPClient *http.Client

	AdditionalHeaderFields map[string]string

	AdditionalRequestFields map[string]any
}

type APIError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("minimax error, status code: %d, type: %s, message: %s", e.HTTPStatus, e.Type, e.Message)
}

func convOrigAPIError(err error) error {
	if err == nil {
		return nil
	}

	return &APIError{Message: err.Error()}
}

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{info: info, stack: stack}
}
