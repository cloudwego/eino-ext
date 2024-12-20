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

package model

type NodeType string
type CanvasVersion string

const (
	CanvasGraphVersionV1 = "1.0.0"
)
const (
	NodeTypeOfStart    NodeType = "start"
	NodeTypeOfEnd      NodeType = "end"
	NodeTypeOfBranch   NodeType = "branch"
	NodeTypeOfParallel NodeType = "parallel"
)
const (
	BasicTypeOfUndefined BasicType = "undefined"
	BasicTypeOfBoolean   BasicType = "boolean"
	BasicTypeOfString    BasicType = "string"
	BasicTypeOfNumber    BasicType = "number"
	BasicTypeOfObject    BasicType = "object"
	BasicTypeOfArray     BasicType = "array"
)

type Edge struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	SourceNodeKey string `json:"source_node_key,omitempty"`
	TargetNodeKey string `json:"target_node_key,omitempty"`
}

type CanvasInfo struct {
	Name    string        `json:"name"`
	Version CanvasVersion `json:"version"`
	*Canvas `json:",inline"`
}

type Canvas struct {
	Nodes []*Node `json:"nodes,omitempty"`
	Edges []*Edge `json:"edges,omitempty"`
}

type Node struct {
	Key string `json:"key"`

	Name string `json:"name"`

	Type NodeType `json:"type"`

	ImplMeta ImplMeta `json:"impl_meta,omitempty"`

	Canvas *Canvas `json:"canvas,omitempty"`
}

type ImplMeta struct {
	AllowOperate bool        `json:"allow_operate"`
	Input        *TypeSchema `json:"input,omitempty"`
	InferInput   *TypeSchema `json:"infer_input,omitempty"` //Inferred input parameters of TypeMeta, currently only used when start run
	Output       *TypeSchema `json:"output,omitempty"`
}

type BasicType string

type TypeSchema struct {
	BasicType            BasicType              `json:"type,omitempty"`
	Title                string                 `json:"title,omitempty"`
	Items                *TypeSchema            `json:"items,omitempty"`
	Properties           map[string]*TypeSchema `json:"properties,omitempty"`
	AnyOf                []*TypeSchema          `json:"anyOf,omitempty"`
	AdditionalProperties *TypeSchema            `json:"additionalProperties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	PropertyOrder        []string               `json:"propertyOrder,omitempty"`
}
