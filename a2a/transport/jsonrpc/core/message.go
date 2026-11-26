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

package core

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
)

type ObjectType string

const (
	ObjectTypeRequest      ObjectType = "request"
	ObjectTypeResponse     ObjectType = "response"
	ObjectTypeNotification ObjectType = "notification"
)

var (
	ErrorParse          = NewError(ParseErrorCode, "Parse error", nil)
	ErrorInvalidRequest = NewError(InvalidRequestCode, "Invalid Request", nil)
	ErrorMethodNotFound = NewError(MethodNotFoundCode, "Method not found", nil)
	ErrorInvalidParams  = NewError(InvalidParamsCode, "Invalid params", nil)
	ErrorInternalError  = NewError(InternalErrorCode, "Internal error", nil)
)

const (
	ParseErrorCode     = -32700
	InvalidRequestCode = -32600
	MethodNotFoundCode = -32601
	InvalidParamsCode  = -32602
	InternalErrorCode  = -32603
)

type Message interface {
	Type() ObjectType
}

type Empty struct{}

type message struct {
	objType ObjectType

	Version string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

func (msg *message) Type() ObjectType {
	return msg.objType
}

type Error struct {
	// Code indicates the error type that occurred.
	Code int64 `json:"code"`
	// Message provides a short description of the error.
	Message string `json:"message"`
	// Data contains additional information about the error.
	Data json.RawMessage `json:"data,omitempty"`
}

func (err *Error) Error() string {
	return err.Message
}

type Request struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      ID              `json:"id"`
	Params  json.RawMessage `json:"params"`
}

func (req *Request) Type() ObjectType {
	return ObjectTypeRequest
}

type Notification struct {
	Version   string          `json:"jsonrpc"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
	relatedID ID
}

func (notif *Notification) Type() ObjectType {
	return ObjectTypeNotification
}

func (notif *Notification) RelatedID() ID {
	return notif.relatedID
}

type Response struct {
	Version string          `json:"jsonrpc"`
	ID      ID              `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Err     *Error          `json:"error,omitempty"`
}

func (resp *Response) Type() ObjectType {
	return ObjectTypeResponse
}

func NewResponse(id ID, res interface{}) (*Response, error) {
	buf, err := sonic.Marshal(res)
	if err != nil {
		return nil, err
	}
	return &Response{
		Version: version,
		ID:      id,
		Result:  buf,
	}, nil
}

type ID struct {
	Str *string
	Num *float64
}

func NewIDFromString(s string) ID {
	return ID{Str: &s}
}

func NewIDFromNumber(n float64) ID {
	return ID{Num: &n}
}

func (id ID) IsNil() bool {
	return id.Str == nil && id.Num == nil
}

func (id ID) UnmarshalJSON(data []byte) error {
	if data == nil {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		id.Str = &s
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		id.Num = &f
		return nil
	}
	return errors.New("invalid JSON-RPC ID")
}

func (id ID) MarshalJSON() ([]byte, error) {
	if id.Str != nil {
		return json.Marshal(*id.Str)
	}
	if id.Num != nil {
		return json.Marshal(*id.Num)
	}
	return nil, fmt.Errorf("invalid ID: empty")
}

func (id ID) String() string {
	if id.Str != nil {
		return *id.Str
	}
	if id.Num != nil {
		return fmt.Sprintf("%v", *id.Num)
	}
	return ""
}
