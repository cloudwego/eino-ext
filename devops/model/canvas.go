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

const (
	Version = "2.0.0"
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
	Name      string    `json:"name"`
	Component Component `json:"component"`
	Nodes     []*Node   `json:"nodes,omitempty"`
	Edges     []*Edge   `json:"edges,omitempty"`
	Branches  []*Branch `json:"branches"`

	// graph config option
	NodeTriggerMode     NodeTriggerMode `json:"node_trigger_mode"`
	GenLocalStateMethod *string         `json:"gen_local_state_method,omitempty"`
}

type Node struct {
	Key  string   `json:"key"`
	Name string   `json:"name"`
	Type NodeType `json:"type"`

	ComponentSchema *ComponentSchema `json:"component_schema,omitempty"`
	GraphSchema     *GraphSchema     `json:"graph_schema,omitempty"`

	// node options
	NodeOption *NodeOption `json:"node_option,omitempty"`

	InferInput   *JsonSchema `json:"infer_input,omitempty"` // inferred input parameters of TypeMeta, currently only used when start run
	AllowOperate bool        `json:"allow_operate"`         //  used to indicate whether the node can be operated on

	Extra map[string]any `json:"extra,omitempty"` // used to store extra information
}

type NodeOption struct {
	InputKey         *string `json:"input_key,omitempty"`
	OutputKey        *string `json:"output_key,omitempty"`
	StatePreHandler  *string `json:"state_pre_handler,omitempty"`
	StatePostHandler *string `json:"state_post_handler,omitempty"`
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
	Method   string `json:"method"`
	IsStream bool   `json:"is_stream"`
}

type JsonType string

const (
	TypeOfBoolean JsonType = "boolean"
	TypeOfString  JsonType = "string"
	TypeOfNumber  JsonType = "number"
	TypeOfObject  JsonType = "object"
	TypeOfArray   JsonType = "array"
	TypeOfNull    JsonType = "null"
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
	Library       Library  `json:"library,omitempty"`
}

type Library struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	Source  string `json:"source"`
}

type Component string

const (
	ComponentOfLambda       Component = "Lambda"
	ComponentOfLoader       Component = "Loader"
	ComponentOfTransformer  Component = "Transformer"
	ComponentOfTool         Component = "Tool"
	ComponentOfChatModel    Component = "ChatModel"
	ComponentOfChatTemplate Component = "ChatTemplate"
	ComponentOfIndexer      Component = "Indexer"
	ComponentOfEmbedder     Component = "Embedder"
	ComponentOfRetriever    Component = "Retriever"
	ComponentOfPassthrough  Component = "Passthrough"
	ComponentOfGraph        Component = "Graph"
)

type ComponentSource string

const (
	SourceOfCustom   ComponentSource = "custom"
	SourceOfOfficial ComponentSource = "official"
)

type ComponentSchema struct {
	Component       Component       `json:"component"`           // type of component (Lambda ChatModel....)
	ComponentSource ComponentSource `json:"component_source"`    // component properties, official components, custom components
	ImplType        string          `json:"impl_type,omitempty"` // The specific implementer name of the component type. For example, openai is the specific implementer of ChatModel.
	InputType       *JsonSchema     `json:"input_type,omitempty"`
	OutputType      *JsonSchema     `json:"output_type,omitempty"`
	Method          string          `json:"method,omitempty"` // component initialization generates the corresponding function name (official components support cloning creation, custom components only support referencing existing components)

	ConfigSchema *JsonSchema `json:"config_schema,omitempty"`
	Config       string      `json:"config"`

	ExtraPropertySchema *JsonSchema `json:"extra_property_schema,omitempty"`
	ExtraProperty       string      `json:"extra_property"`
}
