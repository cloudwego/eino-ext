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

/*
 * This file is used to define the structure of the canvas information.
 * User should not import this file.
 */

package model

import (
	"github.com/cloudwego/eino/components"
)

const (
	Version = "1.0.0"
)

type CanvasInfo struct {
	Version      string `json:"version"`
	*GraphSchema `json:",inline"`
}

type NodeType string

const (
	NodeTypeOfStart    NodeType = "start"
	NodeTypeOfEnd      NodeType = "end"
	NodeTypeOfBranch   NodeType = "branch"
	NodeTypeOfParallel NodeType = "parallel"
)

type NodeTriggerMode string

const (
	AnyPredecessor NodeTriggerMode = "AnyPredecessor"
	AllPredecessor NodeTriggerMode = "AllPredecessor"
)

type GraphSchema struct {
	// ID returns the unique id of the graph.
	ID string `json:"id"`
	// Name returns the name of the graph.
	Name      string               `json:"name"`
	Component components.Component `json:"component"`
	Nodes     []*Node              `json:"nodes,omitempty"`
	Edges     []*Edge              `json:"edges,omitempty"`
	Branches  []*Branch            `json:"branches"`

	// graph config option
	NodeTriggerMode NodeTriggerMode `json:"node_trigger_mode"`
	GenLocalState   *GenLocalState  `json:"gen_local_state,omitempty"`
	InputType       *JsonSchema     `json:"input_type"`
	OutputType      *JsonSchema     `json:"output_type"`
}

type GenLocalState struct {
	IsSet      bool        `json:"is_set"`
	OutputType *JsonSchema `json:"output_type"`
}

type Node struct {
	Key  string   `json:"key"`
	Name string   `json:"name"`
	Type NodeType `json:"type"`

	ComponentSchema *ComponentSchema `json:"component_schema,omitempty"`
	GraphSchema     *GraphSchema     `json:"graph_schema,omitempty"`

	// node options
	NodeOption *NodeOption `json:"node_option,omitempty"`

	AllowOperate bool `json:"allow_operate"` //  used to indicate whether the node can be operated on

	Extra map[string]any `json:"extra,omitempty"` // used to store extra information
}

type NodeOption struct {
	InputKey             *string `json:"input_key,omitempty"`
	OutputKey            *string `json:"output_key,omitempty"`
	UsedStatePreHandler  bool    `json:"used_state_pre_handler,omitempty"`
	UsedStatePostHandler bool    `json:"used_state_post_handler,omitempty"`
}

type Edge struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	SourceNodeKey string `json:"source_node_key,omitempty"`
	TargetNodeKey string `json:"target_node_key,omitempty"`

	Extra map[string]any `json:"extra,omitempty"` // used to store extra information
}

type Branch struct {
	ID             string     `json:"id"`
	Condition      *Condition `json:"condition"`
	SourceNodeKey  string     `json:"source_node_key"`
	TargetNodeKeys []string   `json:"target_node_keys"`

	Extra map[string]any `json:"extra,omitempty"` // used to store extra information
}

type Condition struct {
	Method    string      `json:"method"`
	IsStream  bool        `json:"is_stream"`
	InputType *JsonSchema `json:"input_type"`
}

type JsonType string

const (
	JsonTypeOfBoolean JsonType = "boolean"
	JsonTypeOfString  JsonType = "string"
	JsonTypeOfNumber  JsonType = "number"
	JsonTypeOfObject  JsonType = "object"
	JsonTypeOfArray   JsonType = "array"
	JsonTypeOfNull    JsonType = "null"

	JsonTypeOfInterface JsonType = "interface"
)

type JsonSchema struct {
	Type                 JsonType               `json:"type,omitempty"`
	Title                string                 `json:"title,omitempty"`
	Description          string                 `json:"description"`
	Items                *JsonSchema            `json:"items,omitempty"`
	Properties           map[string]*JsonSchema `json:"properties,omitempty"`
	AnyOf                []*JsonSchema          `json:"anyOf,omitempty"`
	AdditionalProperties *JsonSchema            `json:"additionalProperties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Enum                 []any                  `json:"enum,omitempty"`

	// Custom Field
	PropertyOrder []string `json:"propertyOrder,omitempty"`
	// GoDefinition returns a field supplementary description for Go.
	GoDefinition *GoDefinition `json:"goDefinition,omitempty"`
}

type GoDefinition struct {
	LibraryRef Library `json:"libraryRef,omitempty"`
	// TypeName returns a string representation of the type.
	// The string representation may use shortened package names
	// (e.g., base64 instead of "encoding/base64") and is not
	// guaranteed to be unique among types. To test for type identity,
	// compare the Types directly.
	TypeName string `json:"typeName"`
	// Kind exclude any pointer kind, such as Pointer, UnsafePointer, etc.
	Kind string `json:"kind"`
	// IsPtr whether the type is a pointer type.
	IsPtr bool `json:"isPtr"`
}

type Library struct {
	Version string `json:"version"`
	Module  string `json:"module"`
	// PkgPath returns a defined type's package path, that is, the import path
	// that uniquely identifies the package, such as "encoding/base64".
	// If the type was predeclared (string, error) or not defined (*T, struct{},
	// []int, or A where A is an alias for a non-defined type), the package path
	// will be the empty string.
	PkgPath string `json:"pkgPath"`
}

type ComponentSource string

const (
	SourceOfCustom   ComponentSource = "custom"
	SourceOfOfficial ComponentSource = "official"
)

type ComponentSchema struct {
	// Name returns the displayed name of the component
	Name string `json:"name"`
	// Component returns type of component (Lambda ChatModel....)
	Component components.Component `json:"component"`
	// ComponentSource returns the source of the component, such as official and custom.
	ComponentSource ComponentSource `json:"component_source"`
	// Identifier returns the identifier of the component implementation, such as eino-ext/model/ark
	Identifier string      `json:"identifier,omitempty"`
	InputType  *JsonSchema `json:"input_type,omitempty"`
	OutputType *JsonSchema `json:"output_type,omitempty"`
	Method     string      `json:"method,omitempty"` // component initialization generates the corresponding function name (official components support cloning creation, custom components only support referencing existing components)

	Slots []Slot `json:"slots,omitempty"`

	Config          *ConfigSchema        `json:"config,omitempty"`
	ExtraProperty   *ExtraPropertySchema `json:"extra_property,omitempty"`
	IsIOTypeMutable bool                 `json:"is_io_type_mutable"`

	Version string `json:"version"`
}

type ConfigSchema struct {
	Description string      `json:"description"`
	Schema      *JsonSchema `json:"schema"`
	ConfigInput string      `json:"config_input"`
}

type ExtraPropertySchema struct {
	Schema             *JsonSchema `json:"schema"`
	ExtraPropertyInput string      `json:"extra_property_input"`
}

type Slot struct {
	Component string `json:"component"`

	// The path of the configuration field.
	// for example: if there is no nesting, it means Field, if there is a nested structure, it means Field.NestField.
	FieldLocPath   string            `json:"field_loc_path"`
	Multiple       bool              `json:"multiple"`
	Required       bool              `json:"required"`
	ComponentItems []ComponentSchema `json:"component_items"`
	GoDefinition   *GoDefinition     `json:"go_definition,omitempty"`
}
