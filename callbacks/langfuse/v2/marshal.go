/*
 * Copyright 2026 CloudWeGo Authors
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

package langfuse

import (
	"encoding/json"
	"reflect"
	"unsafe"
)

// einoRunInputTypes lists the Eino values rendered as "{}": their whole state is
// unexported, and encoding/json silently skips such fields. Eino leaves these
// types unexported inside package adk, so they cannot be referenced here and the
// match has to be done by package path and name at run time. That also means a
// rename breaks it silently, which TestHandlerExportsUnexportedAgentRunInput
// catches because it runs a real agent.
var einoRunInputTypes = map[string]bool{
	"github.com/cloudwego/eino/adk.reactRunInput": true,
}

// marshalAttribute encodes a value about to be attached to a langfuse
// observation attribute. The standard encoder stays the primary path and only
// the types above are rebuilt; those are a Chain's input and the input of the
// Lambda node it starts with, so no output or metadata value changes shape.
func marshalAttribute(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(encoded) != 2 || encoded[0] != '{' || encoded[1] != '}' {
		return encoded, nil // not an empty object: the standard encoder handled it
	}
	rv, ok := einoRunInputValue(value)
	if !ok {
		return encoded, nil // not an Eino run input: keep the empty object
	}
	rt := rv.Type()
	fields := make(map[string]json.RawMessage, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field, err := json.Marshal(fieldValue(rv.Field(i)))
		if err != nil {
			continue // one unserializable field must not drop the whole payload
		}
		fields[rt.Field(i).Name] = field
	}
	return json.Marshal(fields)
}

// einoRunInputValue dereferences value and reports whether it is a listed Eino
// run input, ready for field access.
func einoRunInputValue(value any) (reflect.Value, bool) {
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return reflect.Value{}, false
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	rt := rv.Type()
	if !einoRunInputTypes[rt.PkgPath()+"."+rt.Name()] {
		return reflect.Value{}, false
	}
	if !rv.CanAddr() {
		// A value out of an interface is not addressable; copy it to take field addresses.
		addressable := reflect.New(rt).Elem()
		addressable.Set(rv)
		rv = addressable
	}
	return rv, true
}

// fieldValue returns a struct field's value, unexported ones included. Field()
// marks unexported fields read-only and every method call on them panics;
// re-deriving the value from its address clears the mark. Keep this a single
// expression, or checkptr aborts the process.
func fieldValue(fv reflect.Value) any {
	if fv.CanInterface() {
		return fv.Interface()
	}
	return reflect.NewAt(fv.Type(), unsafe.Pointer(fv.UnsafeAddr())).Elem().Interface()
}
