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
	"fmt"
	"reflect"

	"github.com/matoous/go-nanoid"

	"github.com/cloudwego/eino-ext/devops/internal/utils/generic"
	devmodel "github.com/cloudwego/eino-ext/devops/model"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
)

type GraphContainer struct {
	// GraphID graph id.
	GraphID string
	// Name graph display name.
	Name string
	// GraphInfo graph info, from graph compile callback.
	GraphInfo *GraphInfo
	// CanvasInfo graph canvas.
	CanvasInfo *devmodel.CanvasInfo

	// NodeGraphs NodeKey vs Graph, NodeKey is the node where debugging starts.
	NodeGraphs map[string]*Graph
}

type GraphInfo struct {
	*compose.GraphInfo
	// SubGraphNodes NodeKey vs Subgraph Node Info.
	SubGraphNodes map[string]*SubGraphNode
}

type SubGraphNode struct {
	ID            string
	SubGraphNodes map[string]*SubGraphNode
}

func initGraphInfo(gi *GraphInfo) *GraphInfo {
	newCompileOptions := make([]compose.GraphCompileOption, len(gi.GraphInfo.CompileOptions))
	copy(newCompileOptions, gi.GraphInfo.CompileOptions)
	return &GraphInfo{
		GraphInfo: &compose.GraphInfo{
			CompileOptions: newCompileOptions,
			Nodes:          make(map[string]compose.GraphNodeInfo, len(gi.Nodes)),
			Edges:          make(map[string][]string, len(gi.Edges)),
			Branches:       make(map[string][]compose.GraphBranch, len(gi.Branches)),
			InputType:      gi.InputType,
			OutputType:     gi.OutputType,
			Name:           gi.Name,
			GenStateFn:     gi.GenStateFn,
		},
	}
}

func BuildDevGraph(gi *GraphInfo, fromNode string) (g *Graph, err error) {
	if fromNode == compose.END {
		return nil, fmt.Errorf("can not start from end node")
	}

	g = &Graph{Graph: compose.NewGraph[any, any](gi.NewGraphOptions...)}

	var (
		newGI    = initGraphInfo(gi)
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
			node := gi.Nodes[fn]
			if err = g.addNode(fn, node); err != nil {
				return nil, err
			}
			newGI.Nodes[fn] = node
			addNodes[fn] = true
		}

		for _, tn := range gi.Edges[fn] {
			if !addNodes[tn] && tn != compose.END {
				node := gi.Nodes[tn]
				if err = g.addNode(tn, node); err != nil {
					return nil, err
				}
				newGI.Nodes[tn] = node
				addNodes[tn] = true
			}
			if err = g.AddEdge(fn, tn); err != nil {
				return nil, err
			}
			newGI.Edges[fn] = append(newGI.Edges[fn], tn)
			queue = append(queue, tn)
		}

		for _, b := range gi.Branches[fn] {
			bt := b
			for tn := range bt.GetEndNode() {
				if !addNodes[tn] && tn != compose.END {
					node := gi.Nodes[tn]
					if err = g.addNode(tn, node); err != nil {
						return nil, err
					}
					newGI.Nodes[tn] = node
					addNodes[tn] = true
				}
				queue = append(queue, tn)
			}
			if err = g.AddBranch(fn, &bt); err != nil {
				return nil, err
			}
			newGI.Branches[fn] = append(newGI.Branches[fn], bt)
		}

		sgNodes[fn] = true
	}

	if fromNode != compose.START {
		if err = g.AddEdge(compose.START, fromNode); err != nil {
			return nil, err
		}
		newGI.Edges[compose.START] = append(newGI.Edges[compose.START], fromNode)
	}

	g.GraphInfo = newGI

	return g, nil
}

func (gi GraphInfo) BuildGraphSchema(graphName, graphID string) (graph *devmodel.GraphSchema, err error) {
	graph = &devmodel.GraphSchema{
		ID:    graphID,
		Name:  graphName,
		Nodes: make([]*devmodel.Node, 0, len(gi.Nodes)+2),
		Edges: make([]*devmodel.Edge, 0, len(gi.Nodes)+2),
	}
	nodes, err := gi.buildGraphNodes()
	if err != nil {
		return nil, fmt.Errorf("[BuildCanvas] build canvas nodes failed, err=%w", err)
	}
	graph.Nodes = append(graph.Nodes, nodes...)
	edges, nodes, err := gi.buildGraphEdges()
	if err != nil {
		return nil, fmt.Errorf("[BuildCanvas] build canvas edges failed, err=%w", err)
	}
	graph.Nodes = append(graph.Nodes, nodes...)
	graph.Edges = append(graph.Edges, edges...)
	edges, nodes, err = gi.buildGraphBranches()
	if err != nil {
		return nil, fmt.Errorf("[BuildCanvas] build canvas branch failed, err=%w", err)
	}
	graph.Nodes = append(graph.Nodes, nodes...)
	graph.Edges = append(graph.Edges, edges...)
	subGraphSchema, err := gi.buildSubGraphSchema()
	if err != nil {
		return nil, fmt.Errorf("[BuildCanvas] build sub canvas failed, err=%w", err)
	}

	for _, node := range graph.Nodes {
		if sc, ok := subGraphSchema[node.Key]; ok {
			node.GraphSchema = sc
		}
	}

	return graph, err
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

func (gi GraphInfo) buildGraphNodes() (nodes []*devmodel.Node, err error) {
	nodes = make([]*devmodel.Node, 0, len(gi.Nodes)+2)

	nodes = append(nodes,
		&devmodel.Node{
			Key:  compose.START,
			Name: compose.START,
			Type: devmodel.NodeTypeOfStart,
			ComponentSchema: &devmodel.ComponentSchema{
				Component:  compose.ComponentOfGraph,
				InputType:  parseReflectTypeToJsonSchema(gi.InputType),
				OutputType: parseReflectTypeToJsonSchema(gi.OutputType),
			},
			AllowOperate: !generic.UnsupportedInputKind(gi.InputType.Kind()),
		},
		&devmodel.Node{
			Key:          compose.END,
			Name:         compose.END,
			Type:         devmodel.NodeTypeOfEnd,
			AllowOperate: false,
		},
	)

	// add compose nodes
	for key, node := range gi.Nodes {
		fdlNode := &devmodel.Node{
			Key:  key,
			Name: node.Name,
			Type: devmodel.NodeType(node.Component),
		}

		fdlNode.AllowOperate = !generic.UnsupportedInputKind(node.InputType.Kind())

		fdlNode.ComponentSchema = &devmodel.ComponentSchema{
			Component:  node.Component,
			InputType:  parseReflectTypeToJsonSchema(node.InputType),
			OutputType: parseReflectTypeToJsonSchema(node.OutputType),
		}

		fdlNode.ComponentSchema.Name = string(node.Component)
		if implType, ok := components.GetType(node.Instance); ok && implType != "" {
			fdlNode.ComponentSchema.Name = implType
		}

		nodes = append(nodes, fdlNode)
	}

	return nodes, err
}

func (gi GraphInfo) buildGraphEdges() (edges []*devmodel.Edge, nodes []*devmodel.Node, err error) {
	nodes = make([]*devmodel.Node, 0)
	edges = make([]*devmodel.Edge, 0, len(gi.Nodes)+2)
	parallelID := 0
	for sourceNodeKey, targetNodeKeys := range gi.Edges {
		if len(targetNodeKeys) == 0 {
			continue
		}

		if len(targetNodeKeys) == 1 {
			// only one target node
			targetNodeKey := targetNodeKeys[0]
			edges = append(edges, &devmodel.Edge{
				ID:            gonanoid.MustID(6),
				Name:          canvasEdgeName(sourceNodeKey, targetNodeKey),
				SourceNodeKey: sourceNodeKey,
				TargetNodeKey: targetNodeKey,
			})

			continue
		}

		// If it is concurrent, add a virtual concurrent node first
		parallelNode := &devmodel.Node{
			Key:  fmt.Sprintf("from:%s", sourceNodeKey),
			Name: string(devmodel.NodeTypeOfParallel),
			Type: devmodel.NodeTypeOfParallel,
		}
		parallelID++
		nodes = append(nodes, parallelNode)
		edges = append(edges, &devmodel.Edge{
			ID:            gonanoid.MustID(6),
			Name:          canvasEdgeName(sourceNodeKey, parallelNode.Key),
			SourceNodeKey: sourceNodeKey,
			TargetNodeKey: parallelNode.Key,
		})

		for _, targetNodeKey := range targetNodeKeys {
			edges = append(edges, &devmodel.Edge{
				ID:            gonanoid.MustID(6),
				Name:          canvasEdgeName(parallelNode.Key, targetNodeKey),
				SourceNodeKey: parallelNode.Key,
				TargetNodeKey: targetNodeKey,
			})
		}
	}

	return edges, nodes, err
}

func (gi GraphInfo) buildGraphBranches() (edges []*devmodel.Edge, nodes []*devmodel.Node, err error) {
	nodes = make([]*devmodel.Node, 0)
	edges = make([]*devmodel.Edge, 0)
	branchID := 0
	for sourceNodeKey, branches := range gi.Branches {
		for _, branch := range branches {
			// Each branch needs to generate a virtual branch node
			branchNode := &devmodel.Node{
				Key:  fmt.Sprintf("from:%s", sourceNodeKey),
				Name: string(devmodel.NodeTypeOfBranch),
				Type: devmodel.NodeTypeOfBranch,
			}
			branchID++
			nodes = append(nodes, branchNode)
			edges = append(edges, &devmodel.Edge{
				ID:            gonanoid.MustID(6),
				Name:          canvasEdgeName(sourceNodeKey, branchNode.Key),
				SourceNodeKey: sourceNodeKey,
				TargetNodeKey: branchNode.Key,
			})

			branchEndNodes := branch.GetEndNode()
			for branchNodeTargetKey := range branchEndNodes {
				edges = append(edges, &devmodel.Edge{
					ID:            gonanoid.MustID(6),
					Name:          canvasEdgeName(branchNode.Key, branchNodeTargetKey),
					SourceNodeKey: branchNode.Key,
					TargetNodeKey: branchNodeTargetKey,
				})
			}
		}
	}

	return edges, nodes, err
}

func (gi GraphInfo) buildSubGraphSchema() (subGraphSchema map[string]*devmodel.GraphSchema, err error) {
	subGraphSchema = make(map[string]*devmodel.GraphSchema, len(gi.Nodes))
	for nk, sgi := range gi.Nodes {
		if sgi.GraphInfo == nil {
			continue
		}

		subG := GraphInfo{
			GraphInfo:     sgi.GraphInfo,
			SubGraphNodes: gi.SubGraphNodes[nk].SubGraphNodes,
		}
		graphSchema, err := subG.BuildGraphSchema(nk, gi.SubGraphNodes[nk].ID)
		if err != nil {
			return nil, err
		}

		subGraphSchema[nk] = graphSchema
	}

	return subGraphSchema, err
}

type Graph struct {
	*compose.Graph[any, any]
	GraphInfo *GraphInfo
}

func (g *Graph) Compile() (Runnable, error) {
	r, err := g.Graph.Compile(context.Background(), g.GraphInfo.CompileOptions...)
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
	case compose.ComponentOfGraph, compose.ComponentOfChain:
		ins, ok := gni.Instance.(compose.AnyGraph)
		if !ok {
			return fmt.Errorf("component is %s, but get unexpected instance=%v", gni.Component, reflect.TypeOf(gni.Instance))
		}
		return g.AddGraphNode(node, ins, newOpts...)
	default:
		return fmt.Errorf("unsupported component=%s", gni.Component)
	}
}

func parseReflectTypeToJsonSchema(reflectType reflect.Type) (jsonSchema *devmodel.JsonSchema) {
	var processPointer func(title string, ptrLevel int) (newTitle string)
	processPointer = func(title string, ptrLevel int) (newTitle string) {
		for i := 0; i < ptrLevel; i++ {
			title = "*" + title
		}
		return title
	}

	var recursionParseReflectTypeToJsonSchema func(reflectType reflect.Type, ptrLevel int, visited map[reflect.Type]bool) (jsonSchema *devmodel.JsonSchema)

	recursionParseReflectTypeToJsonSchema = func(rt reflect.Type, ptrLevel int, visited map[reflect.Type]bool) (jsc *devmodel.JsonSchema) {
		jsc = &devmodel.JsonSchema{}
		jsc.Type = devmodel.JsonTypeOfNull

		switch rt.Kind() {
		case reflect.Struct:
			if visited[rt] {
				return
			}

			visited[rt] = true

			jsc.Type = devmodel.JsonTypeOfObject
			jsc.Title = processPointer(rt.String(), ptrLevel)
			jsc.Properties = make(map[string]*devmodel.JsonSchema, rt.NumField())
			jsc.PropertyOrder = make([]string, 0, rt.NumField())
			jsc.Required = make([]string, 0, rt.NumField())
			structFieldsJsonSchemaCache := make(map[reflect.Type]*devmodel.JsonSchema, rt.NumField())

			for i := 0; i < rt.NumField(); i++ {
				field := rt.Field(i)
				if !field.IsExported() {
					continue
				}

				var fieldJsonSchema *devmodel.JsonSchema
				if ts, ok := structFieldsJsonSchemaCache[field.Type]; ok {
					fieldJsonSchema = ts
				} else {
					fieldJsonSchema = recursionParseReflectTypeToJsonSchema(field.Type, 0, visited)
					structFieldsJsonSchemaCache[field.Type] = fieldJsonSchema
				}

				jsonName := generic.GetJsonName(field)

				if jsonName == "-" {
					continue
				}

				jsc.Properties[jsonName] = fieldJsonSchema
				jsc.PropertyOrder = append(jsc.PropertyOrder, jsonName)
				if generic.HasRequired(field) {
					jsc.Required = append(jsc.Required, jsonName)
				}
			}

			visited[rt] = false

			return jsc

		case reflect.Pointer:
			return recursionParseReflectTypeToJsonSchema(rt.Elem(), ptrLevel+1, visited)
		case reflect.Map:
			jsc.Type = devmodel.JsonTypeOfObject
			jsc.Title = processPointer(rt.String(), ptrLevel)
			jsc.AdditionalProperties = recursionParseReflectTypeToJsonSchema(rt.Elem(), 0, visited)
			return jsc

		case reflect.Slice, reflect.Array:
			jsc.Type = devmodel.JsonTypeOfArray
			jsc.Title = processPointer(rt.String(), ptrLevel)
			jsc.Items = recursionParseReflectTypeToJsonSchema(rt.Elem(), 0, visited)
			return jsc

		case reflect.String:
			jsc.Type = devmodel.JsonTypeOfString
			jsc.Title = processPointer(rt.String(), ptrLevel)
			return jsc

		case reflect.Bool:
			jsc.Type = devmodel.JsonTypeOfBoolean
			jsc.Title = processPointer(rt.String(), ptrLevel)
			return jsc

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Float32, reflect.Float64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			jsc.Type = devmodel.JsonTypeOfNumber
			jsc.Title = processPointer(rt.String(), ptrLevel)
			return jsc

		case reflect.Interface:
			jsc.Type = devmodel.JsonTypeOfInterface
			return jsc

		default:
			return jsc
		}
	}

	return recursionParseReflectTypeToJsonSchema(reflectType, 0, make(map[reflect.Type]bool))
}

func canvasEdgeName(source, target string) string {
	return fmt.Sprintf("%v_to_%v", source, target)
}
