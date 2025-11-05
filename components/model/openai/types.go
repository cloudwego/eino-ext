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

package openai

import (
	"errors"
	"fmt"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	openai2 "github.com/meguminnnnnnnnn/go-openai"
)

type ReasoningEffortLevel openai.ReasoningEffortLevel

const (
	ReasoningEffortLevelLow    = ReasoningEffortLevel(openai.ReasoningEffortLevelLow)
	ReasoningEffortLevelMedium = ReasoningEffortLevel(openai.ReasoningEffortLevelMedium)
	ReasoningEffortLevelHigh   = ReasoningEffortLevel(openai.ReasoningEffortLevelHigh)
)

type APIError struct {
	Code           any     `json:"code,omitempty"`
	Message        string  `json:"message"`
	Param          *string `json:"param,omitempty"`
	Type           string  `json:"type"`
	HTTPStatus     string  `json:"-"`
	HTTPStatusCode int     `json:"-"`
}

func (e *APIError) Error() string {
	if e.HTTPStatusCode > 0 {
		return fmt.Sprintf("error, status code: %d, status: %s, message: %s", e.HTTPStatusCode, e.HTTPStatus, e.Message)
	}

	return e.Message
}

func convOrigAPIError(err error) error {
	apiErr := &openai2.APIError{}
	if errors.As(err, &apiErr) {
		return &APIError{
			Code:           apiErr.Code,
			Message:        apiErr.Message,
			Param:          apiErr.Param,
			Type:           apiErr.Type,
			HTTPStatus:     apiErr.HTTPStatus,
			HTTPStatusCode: apiErr.HTTPStatusCode,
		}
	}
	return err
}
