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

package model

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino-ext/devops/internal/utils/generic"
	"github.com/cloudwego/eino-ext/devops/model"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

const (
	einoGoType = "_eino_go_type"
	einoValue  = "_value"
)

var registeredTypeMap = make(map[string]reflect.Type)

func init() {
	for i, rt := range registeredTypes {
		registeredTypes[i].Schema = parseReflectTypeToJsonSchema(rt.Type)
		registeredTypeMap[rt.Identifier] = rt.Type
	}
}

type RegisteredType struct {
	// Identifier is the unique identifier for the type, which is type.String()
	Identifier string
	Type       reflect.Type
	Schema     *model.JsonSchema
}

var registeredTypes = []RegisteredType{
	{Identifier: "map[string]interface {}", Type: generic.TypeOf[map[string]interface{}]()},
	{Identifier: "*schema.Message", Type: generic.TypeOf[*schema.Message]()},
	{Identifier: "schema.Message", Type: generic.TypeOf[schema.Message]()},
	{Identifier: "[]*schema.Message", Type: generic.TypeOf[[]*schema.Message]()},
	{Identifier: "*schema.Document", Type: generic.TypeOf[*schema.Document]()},
	{Identifier: "schema.Document", Type: generic.TypeOf[schema.Document]()},
	{Identifier: "[]*schema.Document", Type: generic.TypeOf[[]*schema.Document]()},
	{Identifier: "*document.Source", Type: generic.TypeOf[*document.Source]()},
	{Identifier: "document.Source", Type: generic.TypeOf[document.Source]()},

	{Identifier: "string", Type: generic.TypeOf[string]()},
	{Identifier: "*string", Type: generic.TypeOf[*string]()},
	{Identifier: "float32", Type: generic.TypeOf[float32]()},
	{Identifier: "*float32", Type: generic.TypeOf[*float32]()},
	{Identifier: "float64", Type: generic.TypeOf[float64]()},
	{Identifier: "*float64", Type: generic.TypeOf[*float64]()},
	{Identifier: "int", Type: generic.TypeOf[int]()},
	{Identifier: "*int", Type: generic.TypeOf[*int]()},
	{Identifier: "int8", Type: generic.TypeOf[int8]()},
	{Identifier: "*int8", Type: generic.TypeOf[*int8]()},
	{Identifier: "int16", Type: generic.TypeOf[int16]()},
	{Identifier: "*int16", Type: generic.TypeOf[*int16]()},
	{Identifier: "int32", Type: generic.TypeOf[int32]()},
	{Identifier: "*int32", Type: generic.TypeOf[*int32]()},
	{Identifier: "int64", Type: generic.TypeOf[int64]()},
	{Identifier: "*int64", Type: generic.TypeOf[*int64]()},
	{Identifier: "uint", Type: generic.TypeOf[uint]()},
	{Identifier: "*uint", Type: generic.TypeOf[*uint]()},
	{Identifier: "uint8", Type: generic.TypeOf[uint8]()},
	{Identifier: "*uint8", Type: generic.TypeOf[*uint8]()},
	{Identifier: "uint16", Type: generic.TypeOf[uint16]()},
	{Identifier: "*uint16", Type: generic.TypeOf[*uint16]()},
	{Identifier: "uint32", Type: generic.TypeOf[uint32]()},
	{Identifier: "*uint32", Type: generic.TypeOf[*uint32]()},
	{Identifier: "uint64", Type: generic.TypeOf[uint64]()},
	{Identifier: "*uint64", Type: generic.TypeOf[*uint64]()},
	{Identifier: "bool", Type: generic.TypeOf[bool]()},
	{Identifier: "*bool", Type: generic.TypeOf[*bool]()},
}

func RegisterType(rt reflect.Type) {
	if _, ok := registeredTypeMap[rt.String()]; ok {
		return
	}
	registeredTypeMap[rt.String()] = rt

	registeredTypes = append([]RegisteredType{{
		Identifier: rt.String(),
		Type:       rt,
		Schema:     parseReflectTypeToJsonSchema(rt),
	}}, registeredTypes...)
}

func UnmarshalJson(b []byte, rt reflect.Type) (val reflect.Value, err error) {
	if generic.ComfortableKind(rt.Kind()) {
		ins := reflect.New(rt).Elem()
		if err = json.Unmarshal(b, ins.Addr().Interface()); err != nil {
			return val, fmt.Errorf("unmarshal failed, err=%v, str=%s", err.Error(), string(b))
		}
		return ins, nil
	}

	switch rt.Kind() {
	case reflect.Ptr:
		return unmarshalPtrInput(b, rt)
	case reflect.Slice, reflect.Array:
		return unmarshalSliceInput(b, rt)
	case reflect.Map:
		return unmarshalMapInput(b, rt)
	case reflect.Struct:
		return unmarshalStructInput(b, rt)
	case reflect.Interface:
		return unmarshalInterfaceInput(b)
	default:
		return val, fmt.Errorf("unsupported type=%s", rt.String())
	}
}

func unmarshalPtrInput(b []byte, rt reflect.Type) (val reflect.Value, err error) {
	elemRT := rt.Elem()
	ins, err := UnmarshalJson(b, elemRT)
	if err != nil {
		return val, err
	}

	val = reflect.New(ins.Type())
	val.Elem().Set(ins)

	return val, nil
}

func unmarshalSliceInput(b []byte, rt reflect.Type) (val reflect.Value, err error) {
	var rawMsg []json.RawMessage
	if err = json.Unmarshal(b, &rawMsg); err != nil {
		return val, fmt.Errorf("unmarshal failed, err=%v, str=%s", err.Error(), string(b))
	}

	targetVal := reflect.MakeSlice(rt, len(rawMsg), len(rawMsg))
	for i, raw := range rawMsg {
		fieldVal, err := UnmarshalJson(raw, rt.Elem())
		if err != nil {
			return val, err
		}
		targetVal.Index(i).Set(fieldVal)
	}

	return targetVal, nil
}

func unmarshalMapInput(b []byte, rt reflect.Type) (val reflect.Value, err error) {
	var rawMsg map[string]json.RawMessage
	if err = json.Unmarshal(b, &rawMsg); err != nil {
		return val, fmt.Errorf("unmarshal failed, err=%v, str=%s", err.Error(), string(b))
	}

	mapIns := reflect.MakeMap(rt)
	for key, raw := range rawMsg {
		if key == "" {
			continue
		}

		fieldVal, err := UnmarshalJson(raw, rt.Elem())
		if err != nil {
			return val, err
		}
		mapIns.SetMapIndex(reflect.ValueOf(key), fieldVal)
	}

	return mapIns, nil
}

func unmarshalInterfaceInput(b []byte) (val reflect.Value, err error) {
	var rawMsg map[string]json.RawMessage
	if err = json.Unmarshal(b, &rawMsg); err != nil {
		return val, fmt.Errorf("unmarshal failed, err=%v, str=%s", err.Error(), string(b))
	}

	goTypeStr, ok := rawMsg[einoGoType]
	if !ok {
		return val, fmt.Errorf("key '_eino_go_type' for interface not found, str=%s", string(b))
	}

	var goType string
	err = json.Unmarshal(goTypeStr, &goType)
	if err != nil {
		return val, fmt.Errorf("unmarshal failed, err=%v, str=%s", err.Error(), string(b))
	}

	implType, ok := registeredTypeMap[goType]
	if !ok {
		return val, fmt.Errorf("unregistered type `%s` for interface, str=%s", goType, string(b))
	}

	if implType.String() != goType {
		return val, fmt.Errorf("the registered type=%s is inconsistent with the _eino_go_type=%s, str=%s",
			implType.String(), goType, string(b))
	}

	valueRaw, ok := rawMsg[einoValue]
	if !ok {
		return val, fmt.Errorf("key 'value' for interface not found, str=%s", string(b))
	}

	implInsType := reflect.New(implType).Elem().Type()
	val, err = UnmarshalJson(valueRaw, implInsType)
	if err != nil {
		return val, err
	}

	return val, nil
}

func unmarshalStructInput(b []byte, rt reflect.Type) (val reflect.Value, err error) {
	var rawMsg map[string]json.RawMessage
	if err = json.Unmarshal(b, &rawMsg); err != nil {
		return val, fmt.Errorf("unmarshal failed, err=%v, str=%s", err.Error(), string(b))
	}

	ins := reflect.New(rt).Elem()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonName := generic.GetJsonName(field)
		if jsonName == "-" {
			continue
		}

		raw, ok := rawMsg[jsonName]
		if !ok {
			continue
		}

		fieldIns, err := UnmarshalJson(raw, field.Type)
		if err != nil {
			return val, err
		}

		ins.Field(i).Set(fieldIns)
	}

	return ins, nil
}

func GetRegisteredTypeJsonSchema() []*model.JsonSchema {
	schemas := make([]*model.JsonSchema, 0, len(registeredTypes))
	for _, rt := range registeredTypes {
		schemas = append(schemas, rt.Schema)
	}
	return schemas
}
