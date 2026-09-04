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
	"io"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
)

func DecodeMessages(buf []byte) ([]Message, bool, error) {
	var rawMsgs []json.RawMessage
	var msgs []Message
	if err := sonic.Unmarshal(buf, &rawMsgs); err == nil {
		for _, raw := range rawMsgs {
			m := new(message)
			if err := sonic.Unmarshal(raw, m); err != nil {
				return nil, true, err
			}
			msg, pErr := parseMessage(m)
			if pErr != nil {
				return nil, true, pErr
			}
			msgs = append(msgs, msg)
		}
		return msgs, true, nil
	}
	m := new(message)
	if err := sonic.Unmarshal(buf, m); err != nil {
		return nil, false, err
	}
	msg, pErr := parseMessage(m)
	if pErr != nil {
		return nil, false, pErr
	}
	msgs = append(msgs, msg)
	return msgs, false, nil
}

func DecodeMessage(reader io.Reader) (Message, error) {
	dec := decoder.NewStreamDecoder(reader)
	msg := new(message)
	if err := dec.Decode(msg); err != nil {
		return nil, err
	}
	return parseMessage(msg)
}

func EncodeMessage(msg Message) ([]byte, error) {
	return sonic.Marshal(msg)
}

func ReadMessage(buf []byte) (Message, error) {
	msg := new(message)
	if err := sonic.Unmarshal(buf, msg); err != nil {
		return nil, err
	}
	return parseMessage(msg)
}

func parseMessage(msg *message) (Message, error) {
	id, err := ParseID(msg.ID)
	if err != nil {
		return nil, err
	}
	// request or notification
	if msg.Method != "" {
		// notification
		if id.IsNil() {
			return &Notification{
				Version: msg.Version,
				Method:  msg.Method,
				Params:  msg.Params,
			}, nil
		}
		// request
		return &Request{
			Version: msg.Version,
			Method:  msg.Method,
			ID:      id,
			Params:  msg.Params,
		}, nil

	}
	// response
	return &Response{
		Version: msg.Version,
		ID:      id,
		Result:  msg.Result,
		Err:     msg.Error,
	}, nil
}

func Unmarshal(data []byte, v interface{}) error {
	return sonic.Unmarshal(data, v)
}

func Marshal(v interface{}) ([]byte, error) {
	return sonic.Marshal(v)
}

func NewRequest(method string, id ID, params interface{}) (*Request, error) {
	buf, err := sonic.Marshal(params)
	if err != nil {
		return nil, err
	}
	return &Request{
		Version: version,
		Method:  method,
		ID:      id,
		Params:  buf,
	}, nil
}

func NewNotification(method string, param interface{}) (*Notification, error) {
	buf, err := sonic.Marshal(param)
	if err != nil {
		return nil, err
	}
	return &Notification{
		Version: version,
		Method:  method,
		Params:  buf,
	}, nil
}

func NewFailureResponse(id ID, err *Error) *Response {
	return &Response{
		Version: version,
		ID:      id,
		Err:     err,
	}
}

func NewError(code int64, message string, data interface{}) *Error {
	err := &Error{
		Code:    code,
		Message: message,
	}
	if data != nil {
		dataBuf, _ := Marshal(data)
		// todo: ignore mErr temporarily
		err.Data = dataBuf
	}
	return err
}

func ParseID(raw interface{}) (ID, error) {
	switch raw.(type) {
	case string:
		return NewIDFromString(raw.(string)), nil
	case float64:
		return NewIDFromNumber(raw.(float64)), nil
	case nil:
		return ID{}, nil
	}
	return ID{}, errors.New("invalid ID")
}
