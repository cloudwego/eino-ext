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
	Version = "1.1.0"
)

type CanvasInfo struct {
	ID        string       `json:"id"`
	Version   string       `json:"version"`
	MainGraph *GraphSchema `json:"main_graph"`
	// Deprecated: use MainGraph instead.
	GraphSchema *GraphSchema   `json:",inline"`
	NodeCounter map[string]int `json:"nodeCounter"`
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
	ID   string `json:"id"`
	Name string `json:"name"`
	// Version returns the version of ComponentSchema
	Version   string               `json:"version"`
	Component components.Component `json:"component"`
	Nodes     []*Node              `json:"nodes"`
	Edges     []*Edge              `json:"edges"`
	Branches  []*Branch            `json:"branches"`

	// graph config option
	NodeTriggerMode NodeTriggerMode `json:"node_trigger_mode"`
	GenLocalState   *GenLocalState  `json:"gen_local_state,omitempty"`
	InputType       *JsonSchema     `json:"input_type"`
	OutputType      *JsonSchema     `json:"output_type"`

	// Deprecated: use NodeCounter defined in CanvasInfo instead.
	NodeCounter map[string]int `json:"nodeCounter"`
}

type GenLocalState struct {
	IsSet      bool        `json:"is_set"`
	OutputType *JsonSchema `json:"output_type"`
}

type Node struct {
	ID   string   `json:"id"`
	Key  string   `json:"key"`
	Name string   `json:"name"`
	Type NodeType `json:"type"`

	ComponentSchema *ComponentSchema `json:"component_schema,omitempty"`
	GraphSchema     *GraphSchema     `json:"graph_schema,omitempty"`

	// node options
	NodeOption *NodeOption `json:"node_option,omitempty"`

	AllowOperate bool `json:"allow_operate"` //  used to indicate whether the node can be operated on

	Extra map[string]any `json:"extra,omitempty"` // used to store extra information

	// For UI
	LayoutData *LayoutData `json:"layoutData"`
}

type LayoutData struct {
	Position *Position `json:"position"`
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type NodeOption struct {
	InputKey             *string `json:"input_key,omitempty"`
	OutputKey            *string `json:"output_key,omitempty"`
	UsedStatePreHandler  bool    `json:"used_state_pre_handler,omitempty"`
	UsedStatePostHandler bool    `json:"used_state_post_handler,omitempty"`
}

type Edge struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	SourceNodeKey string `json:"source_node_key,omitempty"`
	TargetNodeKey string `json:"target_node_key,omitempty"`

	SourceWorkFlowNodeID string `json:"sourceWorkflowNodeId,omitempty"`
	TargetWorkFlowNodeID string `json:"targetWorkflowNodeId,omitempty"`

	Extra map[string]any `json:"extra,omitempty"` // used to store extra information
}

type Branch struct {
	ID        string     `json:"id"`
	Condition *Condition `json:"condition"`

	SourceNodeKey  string   `json:"source_node_key"`
	TargetNodeKeys []string `json:"target_node_keys"`

	SourceWorkFlowNodeID  string   `json:"sourceWorkflowNodeId,omitempty"`
	TargetWorkFlowNodeIDs []string `json:"targetWorkflowNodeIds,omitempty"`

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
	// Name returns the displayed name of the component.
	Name string `json:"name"`
	// Version returns the version of ComponentSchema.
	Version string `json:"version"`
	// Component returns type of component (Lambda ChatModel....)
	Component components.Component `json:"component"`
	// ComponentSource returns the source of the component, such as official and custom.
	ComponentSource ComponentSource `json:"component_source"`
	// Identifier returns the identifier of the component implementation, such as eino-ext/model/ark.
	// Identifier will be instead of TypeID in the future.
	Identifier string `json:"identifier,omitempty"`
	// TypeID returns the id of component type, ensuring immutability.
	TypeID             string                    `json:"type_id"`
	InputType          *JsonSchema               `json:"input_type,omitempty"`
	OutputType         *JsonSchema               `json:"output_type,omitempty"`
	ComponentInterface *ComponentInterfaceSchema `json:"component_interface,omitempty"`

	Slots []*Slot `json:"slots,omitempty"`

	// Config returns the configuration for initializing the component.
	Config *ConfigSchema `json:"config,omitempty"`
	// ExtraProperty returns extra properties for the component without defined abstract interface.
	ExtraProperty   *ExtraPropertySchema `json:"extra_property,omitempty"`
	IsIOTypeMutable bool                 `json:"is_io_type_mutable"`

	CustomGenCodeExtraDesc   *CustomGenerateCodeExtraDesc   `json:"custom_gen_code_extra_desc,omitempty"`
	OfficialGenCodeExtraDesc *OfficialGenerateCodeExtraDesc `json:"official_gen_code_extra_desc,omitempty"`
	ModuleVersionInfo        *ModuleVersion                 `json:"module_version_info"`

	// Method returns the component construction method defined internally in DevOps and is not an externally exposed field.
	Method string `json:"method,omitempty"`
}

type ModuleVersion struct {
	Module           string `json:"module"`
	ModuleReleaseTag string `json:"module_release_tag"`
	EinoReleaseTag   string `json:"eino_release_tag"`
	CommitHash       string `json:"commit_hash"`
	CommitDate       string `json:"commit_date"`
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
	Component components.Component `json:"component"`

	// The path of the configuration field.
	// for example: if there is no nesting, it means Field, if there is a nested structure, it means Field.NestField.
	FieldLocPath   string             `json:"field_loc_path"`
	Multiple       bool               `json:"multiple"`
	Required       bool               `json:"required"`
	ComponentItems []*ComponentSchema `json:"component_items"`
	GoDefinition   *GoDefinition      `json:"go_definition,omitempty"`
}

type ComponentInterfaceSchema struct {
	Schema             *JsonSchema `json:"schema"`
	InterfaceInfoInput string      `json:"interface_info_input"`
}

type CustomGenerateCodeExtraDesc struct {
	ConstructorDesc *CustomConstructorDesc `json:"constructor_desc"`
}

type CustomConstructorDesc struct {
	// ConstructorSign returns the constructor signature.
	ConstructorSign *FunctionSignature `json:"constructor_sign"`
	// ImplInterfaces returns the list of abstract interfaces implemented by the component, which may include multiple interfaces, such as Tool.
	ImplInterfaces []*InterfaceDesc `json:"impl_interfaces"`
	// VarDesc returns the description of variables required for component initialization.
	VarDesc *VariableDesc `json:"var_desc"`

	StreamLambda *StreamLambda `json:"stream_lambda"`
}

type OfficialGenerateCodeExtraDesc struct {
	ConstructorDesc *OfficialConstructorDesc `json:"constructor_desc"`
}

type OfficialConstructorDesc struct {
	// ConstructorSign returns the constructor signature.
	ConstructorSign *FunctionSignature `json:"constructor_sign"`
	// VarDesc returns the description of variables required for component initialization.
	VarDesc *VariableDesc `json:"var_desc"`

	InitComponentFunc *CallFunc `json:"init_component_func"`
	InitLambdaFunc    *CallFunc `json:"init_lambda_func"`
}

type VariableDesc struct {
	ConfigDesc *ConfigVarDesc `json:"config_desc"`
}

type ConfigVarDesc struct {
	// Config returns the description of the configuration for initializing the component, which is mutually exclusive with CompositeConfig.
	Config *Parameter `json:"config"`
	// CompositeConfig returns the description of the configuration for initializing the component, which is mutually exclusive with Config.
	CompositeConfig []*Parameter `json:"composite_config"`
}

type InterfaceDesc struct {
	// InterfaceName Returns the interface name.
	// The top-level interface name is presented in the same enumeration as the ComponentInterfaceSchema.
	InterfaceName string `json:"interface_name"`
	// Methods return methods of the interface.
	Methods []*FunctionSignature `json:"methods"`
	// CompositeInterfaces return the list of combined interfaces.
	CompositeInterfaces []*InterfaceDesc `json:"composite_interfaces"`
}

type StreamLambda struct {
	Lib *Library `json:"lib"`
}

type FunctionSignature struct {
	FuncName string       `json:"func_name"`
	Params   []*Parameter `json:"params"`
	Rets     []*Parameter `json:"rets"`
}

type Parameter struct {
	ParamName  string   `json:"param_name"`
	TypeName   string   `json:"type_name"`
	IsPtr      bool     `json:"is_ptr"`
	IsEllipsis bool     `json:"is_ellipsis"`
	Lib        *Library `json:"lib"`
}

type CallFunc struct {
	FuncName string       `json:"func_name"`
	Args     []*Parameter `json:"args"`
	Rets     []*Parameter `json:"rets"`
	Lib      *Library     `json:"lib"`
}
