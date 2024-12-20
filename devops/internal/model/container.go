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

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino-ext/devops/internal/utils/generic"
)

type GraphContainer struct {
	// GraphID graph id.
	GraphID string
	// GraphName graph name.
	GraphName string
	// GraphInfo graph info, from graph compile callback.
	GraphInfo *GraphInfo
	// Canvas graph canvas.
	CanvasInfo *CanvasInfo
	// NodesRunnable NodeKey vs Runnable, NodeKey is the node where debugging starts.
	NodesRunnable map[string]*Runnable
}

type GraphInfo struct {
	*compose.GraphInfo
	Option GraphOption
}

type UnmarshalInput func(ctx context.Context, inputStr string) (input any, err error)

type NodeUnmarshalInput struct {
	NodeKey        string
	UnmarshalInput UnmarshalInput
}

type GraphOption struct {
	NodeInputUnmarshal []*NodeUnmarshalInput
	GenState           func(ctx context.Context) any
}

func (gi GraphInfo) BuildDevGraph(fromNode string) (g *Graph, err error) {
	if fromNode == compose.END {
		return nil, fmt.Errorf("can not start from end node")
	}

	if gi.Option.GenState != nil {
		g = &Graph{StateGraph: compose.NewStateGraph[any, any, any](gi.Option.GenState)}
	} else {
		genState := func(ctx context.Context) any { return nil }
		g = &Graph{StateGraph: compose.NewStateGraph[any, any, any](genState)}
	}

	var (
		queue    = []string{fromNode}
		sgNodes  = make(map[string]bool, len(gi.Nodes))
		addNodes = make(map[string]bool, len(gi.Nodes))
	)
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		if sgNodes[fn] || fn == compose.END {
			continue
		}

		if fn != compose.START && !addNodes[fn] {
			if err = g.addNode(fn, gi.Nodes[fn]); err != nil {
				return nil, err
			}
			addNodes[fn] = true
		}

		for _, tn := range gi.Edges[fn] {
			if !addNodes[tn] && tn != compose.END {
				if err = g.addNode(tn, gi.Nodes[tn]); err != nil {
					return nil, err
				}
				addNodes[tn] = true
			}
			if err = g.AddEdge(fn, tn); err != nil {
				return nil, err
			}
			queue = append(queue, tn)
		}

		for _, b := range gi.Branches[fn] {
			bt := b
			for tn := range bt.GetEndNode() {
				if !addNodes[tn] && tn != compose.END {
					if err = g.addNode(tn, gi.Nodes[tn]); err != nil {
						return nil, err
					}
					addNodes[tn] = true
				}
				queue = append(queue, tn)
			}
			if err = g.AddBranch(fn, &bt); err != nil {
				return nil, err
			}
		}

		sgNodes[fn] = true
	}

	if fromNode != compose.START {
		if err = g.AddEdge(compose.START, fromNode); err != nil {
			return nil, err
		}
	}

	return g, nil
}

func (gi GraphInfo) BuildCanvas() (canvas *Canvas, err error) {

	canvas = &Canvas{
		Nodes: make([]*Node, 0, len(gi.Nodes)+2),
		Edges: make([]*Edge, 0, len(gi.Nodes)+2),
	}
	nodes, err := gi.buildCanvasNodes()
	if err != nil {
		return nil, fmt.Errorf("[BuildCanvas] build canvas nodes failed, err=%w", err)
	}
	canvas.Nodes = append(canvas.Nodes, nodes...)
	edges, nodes, err := gi.buildCanvasEdges()
	if err != nil {
		return nil, fmt.Errorf("[BuildCanvas] build canvas edges failed, err=%w", err)
	}
	canvas.Nodes = append(canvas.Nodes, nodes...)
	canvas.Edges = append(canvas.Edges, edges...)
	edges, nodes, err = gi.buildCanvasBranches()
	if err != nil {
		return nil, fmt.Errorf("[BuildCanvas] build canvas branch failed, err=%w", err)
	}
	canvas.Nodes = append(canvas.Nodes, nodes...)
	canvas.Edges = append(canvas.Edges, edges...)
	subCanvas, err := gi.buildCanvasSubCanvas()
	if err != nil {
		return nil, fmt.Errorf("[BuildCanvas] build sub canvas failed, err=%w", err)
	}

	for _, node := range canvas.Nodes {
		if sc, ok := subCanvas[node.Key]; ok {
			for _, n := range sc.Nodes { // sub canvas can not operate
				n.ImplMeta.AllowOperate = false
			}
			node.Canvas = sc
		}
	}

	return canvas, err
}

func (gi GraphInfo) GetInputNonInterfaceType(nodeKeys []string) (reflectTypes map[string]reflect.Type, err error) {
	reflectTypes = make(map[string]reflect.Type, len(nodeKeys))
	for _, key := range nodeKeys {
		node, ok := gi.Nodes[key]
		if !ok {
			return nil, fmt.Errorf("node=%s not exist in graph", key)
		}
		reflectTypes[key] = node.InputType
	}
	return reflectTypes, nil
}

func (gi GraphInfo) buildCanvasNodes() (nodes []*Node, err error) {

	nodes = make([]*Node, 0, len(gi.Nodes)+2)
	startNode := &Node{
		Key:  compose.START,
		Name: compose.START,
		Type: NodeTypeOfStart,
	}

	implMeta, err := gi.inferStartNodeImplMeta()
	if err != nil {
		return nil, fmt.Errorf("[buildCanvasNodes] failed to infer start node impl meta, err=%w", err)
	}

	startNode.ImplMeta = implMeta
	nodes = append(nodes, startNode)

	nodes = append(nodes, &Node{
		Key:  compose.END,
		Name: compose.END,
		Type: NodeTypeOfEnd,
		ImplMeta: ImplMeta{
			AllowOperate: false,
		},
	})

	// add compose nodes
	for key, node := range gi.Nodes {
		fdlNode := &Node{
			Key:  key,
			Name: node.Name,
			Type: NodeType(node.Component),
		}

		fdlNode.ImplMeta = ImplMeta{
			AllowOperate: generic.ValidateInputReflectTypeSupported(node.InputType),
			Input:        reassembleTypeSchema(parseReflectTypeToTypeSchema(node.InputType), len(node.InputKey) != 0),
			Output:       reassembleTypeSchema(parseReflectTypeToTypeSchema(node.OutputType), len(node.OutputKey) != 0),
		}

		for _, nn := range gi.Option.NodeInputUnmarshal {
			if nn.NodeKey == key {
				fdlNode.ImplMeta.AllowOperate = true
			}
		}

		nodes = append(nodes, fdlNode)
	}

	return nodes, err

}

func (gi GraphInfo) buildCanvasEdges() (edges []*Edge, nodes []*Node, err error) {
	nodes = make([]*Node, 0)
	edges = make([]*Edge, 0, len(gi.Nodes)+2)
	parallelID := 0
	for sourceNodeKey, targetNodeKeys := range gi.Edges {
		if len(targetNodeKeys) == 0 {
			continue
		}

		if len(targetNodeKeys) == 1 {
			// only one target node
			targetNodeKey := targetNodeKeys[0]
			edgeID, err := uuid.NewRandom()
			if err != nil {
				return nil, nil, err
			}

			edges = append(edges, &Edge{
				ID:            edgeID.String(),
				Name:          canvasEdgeName(sourceNodeKey, targetNodeKey),
				SourceNodeKey: sourceNodeKey,
				TargetNodeKey: targetNodeKey,
			})

			continue
		}

		// If it is concurrent, add a virtual concurrent node first
		parallelNode := &Node{
			Key:  fmt.Sprintf("from:%s", sourceNodeKey),
			Name: string(NodeTypeOfParallel),
			Type: NodeTypeOfParallel,
		}
		parallelID++
		nodes = append(nodes, parallelNode)
		edgeID, err := uuid.NewRandom()
		if err != nil {
			return nil, nil, err
		}
		edges = append(edges, &Edge{
			ID:            edgeID.String(),
			Name:          canvasEdgeName(sourceNodeKey, parallelNode.Key),
			SourceNodeKey: sourceNodeKey,
			TargetNodeKey: parallelNode.Key,
		})

		for _, targetNodeKey := range targetNodeKeys {
			edgeID, err := uuid.NewRandom()
			if err != nil {
				return nil, nil, err
			}
			edges = append(edges, &Edge{
				ID:            edgeID.String(),
				Name:          canvasEdgeName(parallelNode.Key, targetNodeKey),
				SourceNodeKey: parallelNode.Key,
				TargetNodeKey: targetNodeKey,
			})
		}
	}
	return edges, nodes, err

}
func (gi GraphInfo) buildCanvasBranches() (edges []*Edge, nodes []*Node, err error) {
	nodes = make([]*Node, 0)
	edges = make([]*Edge, 0)
	branchID := 0
	for sourceNodeKey, branches := range gi.Branches {
		for _, branch := range branches {
			// Each branch needs to generate a virtual branch node
			branchNode := &Node{
				Key:  fmt.Sprintf("from:%s", sourceNodeKey),
				Name: string(NodeTypeOfBranch),
				Type: NodeTypeOfBranch,
			}
			branchID++
			nodes = append(nodes, branchNode)
			edgeID, err := uuid.NewRandom()
			if err != nil {
				return nil, nil, err
			}
			edges = append(edges, &Edge{
				ID:            edgeID.String(),
				Name:          canvasEdgeName(sourceNodeKey, branchNode.Key),
				SourceNodeKey: sourceNodeKey,
				TargetNodeKey: branchNode.Key,
			})

			branchEndNodes := branch.GetEndNode()
			for branchNodeTargetKey := range branchEndNodes {
				edgeID, err := uuid.NewRandom()
				if err != nil {
					return nil, nil, err
				}
				edges = append(edges, &Edge{
					ID:            edgeID.String(),
					Name:          canvasEdgeName(branchNode.Key, branchNodeTargetKey),
					SourceNodeKey: branchNode.Key,
					TargetNodeKey: branchNodeTargetKey,
				})
			}
		}
	}

	return edges, nodes, err
}
func (gi GraphInfo) buildCanvasSubCanvas() (subCanvas map[string]*Canvas, err error) {

	subCanvas = make(map[string]*Canvas, len(gi.Nodes))
	for key, graphNodeInfo := range gi.Nodes {
		if graphNodeInfo.GraphInfo != nil {
			subG := GraphInfo{
				GraphInfo: graphNodeInfo.GraphInfo,
			}
			canvas, err := subG.BuildCanvas()
			if err != nil {
				return nil, err
			}
			subCanvas[key] = canvas
		}

	}

	return subCanvas, err
}

type Graph struct {
	*compose.StateGraph[any, any, any]
}

func (g *Graph) Compile(opts ...compose.GraphCompileOption) (Runnable, error) {
	r, err := g.Graph.Compile(context.Background(), opts...)
	return Runnable{r: r}, err
}

func (g *Graph) addNode(node string, gni compose.GraphNodeInfo, opts ...compose.GraphAddNodeOpt) error {
	newOpts := append(gni.GraphAddNodeOpts, opts...)
	switch gni.Component {
	case components.ComponentOfEmbedding:
		ins, ok := gni.Instance.(embedding.Embedder)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddEmbeddingNode(node, ins, newOpts...)
	case components.ComponentOfRetriever:
		ins, ok := gni.Instance.(retriever.Retriever)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddRetrieverNode(node, ins, newOpts...)
	case components.ComponentOfLoaderSplitter:
		ins, ok := gni.Instance.(document.LoaderSplitter)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddLoaderSplitterNode(node, ins, newOpts...)
	case components.ComponentOfIndexer:
		ins, ok := gni.Instance.(indexer.Indexer)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddIndexerNode(node, ins, newOpts...)
	case components.ComponentOfChatModel:
		ins, ok := gni.Instance.(model.ChatModel)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddChatModelNode(node, ins, newOpts...)
	case components.ComponentOfPrompt:
		ins, ok := gni.Instance.(prompt.ChatTemplate)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddChatTemplateNode(node, ins, newOpts...)
	case compose.ComponentOfToolsNode:
		ins, ok := gni.Instance.(*compose.ToolsNode)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddToolsNode(node, ins, newOpts...)
	case compose.ComponentOfLambda:
		ins, ok := gni.Instance.(*compose.Lambda)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddLambdaNode(node, ins, newOpts...)
	case compose.ComponentOfPassthrough:
		return g.AddPassthroughNode(node, newOpts...)
	case compose.ComponentOfGraph, compose.ComponentOfChain, compose.ComponentOfStateGraph:
		ins, ok := gni.Instance.(compose.AnyGraph)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddGraphNode(node, ins, newOpts...)
	default:
		return fmt.Errorf("unsupported component=%s", gni.Component)
	}
}

func (gi GraphInfo) inferStartNodeImplMeta() (ImplMeta, error) {
	implMeta := ImplMeta{}
	implMeta.Input = parseReflectTypeToTypeSchema(gi.InputType) //

	inferGraphType, support, err := gi.InferGraphInputType(compose.START)
	if err != nil {
		return implMeta, err
	}

	implMeta.AllowOperate = support
	for _, nn := range gi.Option.NodeInputUnmarshal {
		if nn.NodeKey == compose.START {
			implMeta.AllowOperate = true
		}
	}

	if len(inferGraphType.InputTypes) == 0 {
		implMeta.InferInput = parseReflectTypeToTypeSchema(inferGraphType.InputType)
		return implMeta, nil
	}

	var parseGraphInferTypeToTypeSchema func(inferType GraphInferType) *TypeSchema
	parseGraphInferTypeToTypeSchema = func(inferType GraphInferType) *TypeSchema {
		typeSchema := &TypeSchema{
			BasicType:  BasicTypeOfObject,
			Title:      reflect.TypeOf(map[string]interface{}{}).String(),
			Required:   make([]string, 0, len(inferGraphType.InputTypes)),
			Properties: make(map[string]*TypeSchema, len(inferGraphType.InputTypes)),
		}
		for inputKey, reflectType := range inferType.InputTypes {
			typeSchema.Properties[inputKey] = parseReflectTypeToTypeSchema(reflectType)
			typeSchema.Required = append(typeSchema.Required, inputKey)
		}
		for nodeKey, gInferType := range inferType.ComplicatedGraphInferType {
			if node, ok := gi.Nodes[nodeKey]; ok {
				inputKey := node.InputKey
				if len(inputKey) > 0 {
					typeSchema.Properties[inputKey] = parseGraphInferTypeToTypeSchema(gInferType)
				}
			}
		}
		return typeSchema
	}
	implMeta.InferInput = parseGraphInferTypeToTypeSchema(inferGraphType)
	return implMeta, nil

}

func parseReflectTypeToTypeSchema(reflectType reflect.Type) (typeSchema *TypeSchema) {

	processedTypes := make(map[reflect.Type]bool)

	var recursionParseReflectTypeToTypeSchema func(reflectType reflect.Type) (typeSchema *TypeSchema)

	recursionParseReflectTypeToTypeSchema = func(reflectType reflect.Type) (typeSchema *TypeSchema) {
		if processedTypes[reflectType] {
			return recursionParseReflectTypeToTypeSchema(reflect.TypeOf(map[string]interface{}{}))
		}
		if reflectType.Kind() == reflect.Struct {
			processedTypes[reflectType] = true

		}
		typeSchema = &TypeSchema{}
		typeSchema.BasicType = BasicTypeOfUndefined
		switch reflectType.Kind() {
		case reflect.Struct:
			typeSchema.BasicType = BasicTypeOfObject
			typeSchema.Title = reflectType.String()
			typeSchema.Properties = make(map[string]*TypeSchema, reflectType.NumField())
			typeSchema.PropertyOrder = make([]string, 0, reflectType.NumField())
			typeSchema.Required = make([]string, 0, reflectType.NumField())
			structFieldsTypeSchemaCache := make(map[reflect.Type]*TypeSchema, reflectType.NumField())
			for i := 0; i < reflectType.NumField(); i++ {
				field := reflectType.Field(i)
				if !field.IsExported() || field.Type.Kind() == reflect.Interface {
					continue
				}

				var fieldTypeSchema *TypeSchema
				if ts, ok := structFieldsTypeSchemaCache[field.Type]; ok {
					fieldTypeSchema = ts
				} else {
					fieldTypeSchema = recursionParseReflectTypeToTypeSchema(field.Type)
					structFieldsTypeSchemaCache[field.Type] = fieldTypeSchema
				}

				jsonName := generic.GetJsonTag(field)
				typeSchema.Properties[jsonName] = fieldTypeSchema
				typeSchema.PropertyOrder = append(typeSchema.PropertyOrder, jsonName)
				if generic.HasRequired(field) {
					typeSchema.Required = append(typeSchema.Required, jsonName)
				}

			}
			return typeSchema
		case reflect.Pointer:
			typeSchema = recursionParseReflectTypeToTypeSchema(reflectType.Elem())
			return
		case reflect.Map:
			typeSchema.BasicType = BasicTypeOfObject
			typeSchema.Title = reflectType.String()
			typeSchema.AdditionalProperties = recursionParseReflectTypeToTypeSchema(reflectType.Elem())

			return typeSchema

		case reflect.Slice, reflect.Array:
			typeSchema.BasicType = BasicTypeOfArray
			typeSchema.Title = reflectType.String()
			typeSchema.Items = recursionParseReflectTypeToTypeSchema(reflectType.Elem())
			return
		case reflect.String:
			typeSchema.BasicType = BasicTypeOfString
			typeSchema.Title = reflectType.String()
			return typeSchema
		case reflect.Bool:
			typeSchema.BasicType = BasicTypeOfBoolean
			typeSchema.Title = reflectType.String()
			return typeSchema
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Float32, reflect.Float64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			typeSchema.BasicType = BasicTypeOfNumber
			typeSchema.Title = reflectType.String()
			return typeSchema
		case reflect.Interface:
			typeSchema.BasicType = ""
			typeSchema.AnyOf = make([]*TypeSchema, 0, 5)
			typeSchema.AnyOf = append(typeSchema.AnyOf, &TypeSchema{BasicType: BasicTypeOfBoolean})
			typeSchema.AnyOf = append(typeSchema.AnyOf, &TypeSchema{BasicType: BasicTypeOfString})
			typeSchema.AnyOf = append(typeSchema.AnyOf, &TypeSchema{BasicType: BasicTypeOfNumber})
			typeSchema.AnyOf = append(typeSchema.AnyOf, &TypeSchema{BasicType: BasicTypeOfArray})
			typeSchema.AnyOf = append(typeSchema.AnyOf, &TypeSchema{BasicType: BasicTypeOfObject})
			return typeSchema
		default:
			return typeSchema
		}
	}

	return recursionParseReflectTypeToTypeSchema(reflectType)

}

func reassembleTypeSchema(typeSchema *TypeSchema, hasInputOrOutputKey bool) *TypeSchema {
	if !hasInputOrOutputKey {
		return typeSchema
	}
	typeSchema = &TypeSchema{
		BasicType:            BasicTypeOfObject,
		Title:                reflect.TypeOf(map[string]interface{}{}).String(),
		AdditionalProperties: typeSchema,
	}
	return typeSchema

}

func canvasEdgeName(source, target string) string {
	return fmt.Sprintf("%v_to_%v", source, target)
}

// TODO@maronghong: improve design, too complicated now.
type GraphInferType struct {
	// InputType If start nodes have no inputKey, inputType is not nil.
	InputType reflect.Type
	// InputTypes InputKey vs inputType. If start nodes have inputKey, inputTypes is not nil.
	InputTypes map[string]reflect.Type
	// ComplicatedGraphInferType NodeKey vs subgraph inferType,
	// it will set when the node is a subgraph with inputKey and its start nodes with inputKey.
	ComplicatedGraphInferType map[string]GraphInferType
}

func (gi GraphInfo) defaultInferInputType() GraphInferType {
	return GraphInferType{
		InputType: gi.InputType,
	}
}

func (gi GraphInfo) InferGraphInputType(node string) (inferType GraphInferType, supported bool, err error) {
	if node == compose.END {
		return inferType, false, fmt.Errorf("cannot infer inputType for end node")
	}

	if node == compose.START {
		return gi.inferInputType()
	}

	ni, ok := gi.Nodes[node]
	if !ok {
		return inferType, false, fmt.Errorf("node=%s not found", node)
	}

	if ni.GraphInfo != nil {
		inferType, supported, err = GraphInfo{GraphInfo: ni.GraphInfo}.inferInputType()
		if err != nil {
			return inferType, false, err
		}
		if ni.InputKey == "" {
			return inferType, supported, nil
		}

		if inferType.InputType != nil {
			inferType = GraphInferType{
				InputTypes: map[string]reflect.Type{
					ni.InputKey: inferType.InputType,
				},
			}
		} else {
			inferType = GraphInferType{
				InputTypes: map[string]reflect.Type{
					ni.InputKey: generic.TypeOf[map[string]any](),
				},
				ComplicatedGraphInferType: map[string]GraphInferType{
					node: {
						InputTypes:                inferType.InputTypes,
						ComplicatedGraphInferType: inferType.ComplicatedGraphInferType,
					},
				},
			}
		}

		return inferType, supported, nil
	}

	if generic.ValidateInputReflectTypeSupported(ni.InputType) {
		if ni.InputKey == "" {
			inferType = GraphInferType{
				InputType: ni.InputType,
			}
		} else {
			inferType = GraphInferType{
				InputTypes: map[string]reflect.Type{
					ni.InputKey: ni.InputType,
				},
			}
		}
		return inferType, true, nil
	}

	return inferType, false, nil
}

func (gi GraphInfo) inferInputType() (inferType GraphInferType, supported bool, err error) {
	if generic.ValidateInputReflectTypeSupported(gi.InputType) {
		return gi.defaultInferInputType(), true, nil
	}

	gTyp := gi.InputType
	for gTyp.Kind() == reflect.Pointer {
		gTyp = gTyp.Elem()
	}

	if gTyp.Kind() == reflect.Interface || gTyp.Kind() == reflect.Map {
		return gi.inferGraphInputTypeByNodes()
	}

	return gi.defaultInferInputType(), false, nil
}

// TODO@maronghong: log error
func (gi GraphInfo) inferGraphInputTypeByNodes() (git GraphInferType, supported bool, err error) {
	defaultTyp := gi.defaultInferInputType()

	etns := gi.Edges[compose.START]
	bs := gi.Branches[compose.START]

	startNodes := make(map[string]compose.GraphNodeInfo, len(etns)+len(bs)*2)
	for _, tn := range etns {
		startNodes[tn] = gi.Nodes[tn]
	}
	for _, b := range bs {
		for tn := range b.GetEndNode() {
			startNodes[tn] = gi.Nodes[tn]
		}
	}

	git = GraphInferType{
		ComplicatedGraphInferType: make(map[string]GraphInferType, len(startNodes)),
		InputTypes:                make(map[string]reflect.Type, len(startNodes)),
	}

	withInputKeyNodes := make(map[string]compose.GraphNodeInfo, len(startNodes))
	for nk, ni := range startNodes {
		// If one of nodes has inputKey, all nodes should have inputKey for einodev.
		if ni.GraphInfo == nil && len(withInputKeyNodes) > 0 && ni.InputKey == "" {
			return defaultTyp, false, nil
		}
		// Although the inputKey is the same, the type may be different.
		// Eino will check in runtime, not check here.
		if ni.InputKey != "" {
			withInputKeyNodes[nk] = ni
		}
	}

	if len(withInputKeyNodes) == 0 && gi.InputType.Kind() == reflect.Map {
		return defaultTyp, false, nil
	}

	// handle withInputKey scene
	if len(withInputKeyNodes) > 0 {
		// handle with InputKey node
		for nk, ni := range withInputKeyNodes {
			if ni.GraphInfo == nil {
				if !generic.ValidateInputReflectTypeSupported(ni.InputType) {
					return defaultTyp, false, nil
				}
				git.InputTypes[ni.InputKey] = ni.InputType
				continue
			}

			git_, ok, err := GraphInfo{GraphInfo: ni.GraphInfo}.inferInputType()
			if err != nil {
				return defaultTyp, false, err
			}
			if !ok {
				return defaultTyp, false, nil
			}

			if git_.InputType != nil {
				git.InputTypes[ni.InputKey] = git_.InputType
			} else {
				git.InputTypes[ni.InputKey] = generic.TypeOf[map[string]any]()
				git.ComplicatedGraphInferType[nk] = git_
			}
		}

		// hande without InputKey node, but with GraphInfo
		for nk, ni := range startNodes {
			if ni.GraphInfo == nil {
				continue
			}
			if _, ok := withInputKeyNodes[nk]; ok {
				continue
			}

			git_, ok, err := GraphInfo{GraphInfo: ni.GraphInfo}.inferInputType()
			if err != nil {
				return defaultTyp, false, err
			}
			if !ok {
				return defaultTyp, false, nil
			}
			// If parent graph's start nodes have inputKey
			// the subgraph's start nodes should have inputKey for einodev.
			if len(git_.InputTypes) == 0 {
				return defaultTyp, false, nil
			}
			// merge inputTypes
			for ipk, t := range git_.InputTypes {
				git.InputTypes[ipk] = t
			}
		}

		return git, true, nil
	}

	// handle withoutInputKey scene
	for _, ni := range startNodes {
		typ := ni.InputType
		if ni.GraphInfo != nil {
			git_, ok, err := GraphInfo{GraphInfo: ni.GraphInfo}.inferInputType()
			if err != nil {
				return defaultTyp, false, err
			}
			if !ok {
				return defaultTyp, false, nil
			}
			// If parent graph's start nodes have no inputKey,
			// the subgraph's start nodes have no inputKey for einodev.
			if len(git_.InputTypes) > 0 {
				return defaultTyp, false, nil
			}
			typ = git_.InputType
		}

		if git.InputType == nil {
			git.InputType = typ
			if !generic.ValidateInputReflectTypeSupported(typ) {
				return defaultTyp, false, nil
			}
		}
		if git.InputType != typ {
			return defaultTyp, false, nil
		}
	}

	return git, true, nil
}

func (git GraphInferType) UnmarshalJson(jsonStr string) (reflect.Value, error) {
	if git.InputType != nil {
		return unmarshalJsonWithReflectType(jsonStr, git.InputType)
	}
	return unmarshalJsonWithGraphInferType(jsonStr, git)
}

func unmarshalJsonWithReflectType(jsonStr string, rt reflect.Type) (reflect.Value, error) {
	it := rt
	ptrLevel := 0
	for it.Kind() == reflect.Pointer {
		ptrLevel++
		it = it.Elem()
	}

	input := reflect.New(it).Elem()
	err := json.Unmarshal([]byte(jsonStr), input.Addr().Interface())
	if err != nil {
		return reflect.Value{}, err
	}

	input = getPtrValue(input, ptrLevel)

	return input, nil
}

func unmarshalJsonWithGraphInferType(jsonStr string, git GraphInferType) (reflect.Value, error) {
	if len(git.InputTypes) == 0 {
		return reflect.Value{}, fmt.Errorf("inputTypes is nil")
	}

	var inputs map[string]json.RawMessage
	err := json.Unmarshal([]byte(jsonStr), &inputs)
	if err != nil {
		return reflect.Value{}, err
	}

	input := reflect.MakeMap(reflect.TypeOf(map[string]any{}))

	for inputKey, v := range inputs {
		it, ok := git.InputTypes[inputKey]
		if !ok {
			continue
		}

		var inputVal reflect.Value
		if !generic.IsMapType[string, any](it) {
			inputVal, err = unmarshalJsonWithReflectType(string(v), it)
			if err != nil {
				return reflect.Value{}, err
			}
		} else {
			for _, nit := range git.ComplicatedGraphInferType {
				inputVal, err = unmarshalJsonWithGraphInferType(string(v), nit)
				if err != nil {
					return reflect.Value{}, err
				}
				break
			}
		}

		input.SetMapIndex(reflect.ValueOf(inputKey), inputVal)
	}

	return input, nil
}
