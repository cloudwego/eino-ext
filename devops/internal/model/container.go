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
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	"github.com/cloudwego/eino-ext/devops/internal/utils/generic"
	devmodel "github.com/cloudwego/eino-ext/devops/model"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
	"golang.org/x/exp/slices"
)

type GraphContainer struct {
	// GraphID graph id.
	GraphID string
	// GraphName graph name.
	GraphName string
	// GraphInfo graph info, from graph compile callback.
	GraphInfo *GraphInfo
	// Canvas graph canvas.
	CanvasInfo *devmodel.CanvasInfo
	// NodesGraph NodeKey vs Graph, NodeKey is the node where debugging starts.
	NodesGraph map[string]*Graph
}

type GraphInfo struct {
	*compose.GraphInfo
	Option GraphOption
}

type GraphOption struct {
	GenState func(ctx context.Context) any
}

func initGraphInfo(gi *GraphInfo) *GraphInfo {
	return &GraphInfo{
		GraphInfo: &compose.GraphInfo{
			CompileOptions: slices.Clone(gi.GraphInfo.CompileOptions),
			Nodes:          make(map[string]compose.GraphNodeInfo, len(gi.Nodes)),
			Edges:          make(map[string][]string, len(gi.Edges)),
			Branches:       make(map[string][]compose.GraphBranch, len(gi.Branches)),
			InputType:      gi.InputType,
			OutputType:     gi.OutputType,
			Name:           gi.Name,
			GenStateFn:     gi.GenStateFn,
		},
		Option: GraphOption{
			GenState: gi.GenStateFn,
		},
	}
}

func BuildDevGraph(gi *GraphInfo, fromNode string) (g *Graph, err error) {
	if fromNode == compose.END {
		return nil, fmt.Errorf("can not start from end node")
	}

	if gi.Option.GenState != nil {
		g = &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(gi.Option.GenState))}
	} else {
		g = &Graph{Graph: compose.NewGraph[any, any]()}
	}

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

func (gi GraphInfo) BuildGraphSchema() (graph *devmodel.GraphSchema, err error) {
	graph = &devmodel.GraphSchema{
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
			for _, n := range sc.Nodes { // sub canvas can not operate
				n.AllowOperate = false
			}
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
			edgeID, err := uuid.NewRandom()
			if err != nil {
				return nil, nil, err
			}

			edges = append(edges, &devmodel.Edge{
				ID:            edgeID.String(),
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
		edgeID, err := uuid.NewRandom()
		if err != nil {
			return nil, nil, err
		}
		edges = append(edges, &devmodel.Edge{
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
			edges = append(edges, &devmodel.Edge{
				ID:            edgeID.String(),
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
			edgeID, err := uuid.NewRandom()
			if err != nil {
				return nil, nil, err
			}
			edges = append(edges, &devmodel.Edge{
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
				edges = append(edges, &devmodel.Edge{
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
func (gi GraphInfo) buildSubGraphSchema() (subGraphSchema map[string]*devmodel.GraphSchema, err error) {
	subGraphSchema = make(map[string]*devmodel.GraphSchema, len(gi.Nodes))
	for key, graphNodeInfo := range gi.Nodes {
		if graphNodeInfo.GraphInfo != nil {
			subG := GraphInfo{
				GraphInfo: graphNodeInfo.GraphInfo,
			}
			graphSchema, err := subG.BuildGraphSchema()
			if err != nil {
				return nil, err
			}
			subGraphSchema[key] = graphSchema
		}
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
					fieldJsonSchema = &devmodel.JsonSchema{
						Type:                 ts.Type,
						Title:                ts.Title,
						Properties:           ts.Properties,
						Items:                ts.Items,
						AdditionalProperties: ts.AdditionalProperties,
						Description:          field.Name,
					}
				} else {
					fieldJsonSchema = recursionParseReflectTypeToJsonSchema(field.Type, 0, visited)
					fieldJsonSchema.Description = field.Name
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

type FieldInfo struct {
	JSONKey string
	Schema  *devmodel.JsonSchema
}

func ConvertCodeToValue(code string, schema *devmodel.JsonSchema, inputType reflect.Type) (reflect.Value, error) {
	node, err := parser.ParseFile(token.NewFileSet(), "", "package main\n"+code, parser.ParseComments)
	if err != nil {
		return reflect.Value{}, err
	}

	var result interface{}
	ast.Inspect(node, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}

		for _, value := range vs.Values {
			var cl *ast.CompositeLit
			switch v := value.(type) {
			case *ast.UnaryExpr:
				cl, ok = v.X.(*ast.CompositeLit)
				if !ok {
					continue
				}
				result = parseCompositeLit(cl, schema)
			case *ast.CompositeLit:
				if schema.Type == devmodel.JsonTypeOfArray {
					result = parseArrayLit(v, schema)
				} else if schema.Type == devmodel.JsonTypeOfObject && schema.AdditionalProperties != nil {
					result = parseMapLit(v, schema)
				} else {
					result = parseCompositeLit(v, schema)
				}
			case *ast.BasicLit:
				result = parseBasicLit(v)
			case *ast.Ident:
				switch v.Name {
				case "true":
					result = true
				case "false":
					result = false
				case "nil":
					result = nil
				default:
					result = v.Name
				}
			default:
				continue
			}
		}
		return false
	})

	val := reflect.New(inputType)

	if err := convertToValue(result, val.Elem()); err != nil {
		return reflect.Value{}, err
	}

	return val.Elem(), nil
}

func parseCompositeLit(cl *ast.CompositeLit, schema *devmodel.JsonSchema) map[string]interface{} {
	data := make(map[string]interface{})
	fieldTagMap := buildFieldTagMap(schema)

	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		fieldName := keyIdent.Name

		fieldInfo, ok := fieldTagMap[fieldName]
		if !ok {
			continue
		}

		// todo key不设置为json key
		// jsonKey := fieldInfo.JSONKey
		jsonKey := fieldInfo.Schema.Description
		fieldSchema := fieldInfo.Schema
		value := parseExpr(kv.Value, fieldSchema)
		data[jsonKey] = value
	}
	return data
}

func parseExpr(expr ast.Expr, schema *devmodel.JsonSchema) interface{} {
	switch v := expr.(type) {
	case *ast.Ident:
		// 处理标识符
		switch v.Name {
		case "true":
			return true
		case "false":
			return false
		case "nil":
			return nil
		default:
			return v.Name
		}
	case *ast.BasicLit:
		return parseBasicLit(v)
	case *ast.CompositeLit:
		switch schema.Type {
		case devmodel.JsonTypeOfObject:
			if schema.AdditionalProperties != nil {
				return parseMapLit(v, schema)
			}
			return parseCompositeLit(v, schema)
		case devmodel.JsonTypeOfArray:
			return parseArrayLit(v, schema)
		default:
			return nil
		}
	case *ast.SelectorExpr:
		//todo useless, to delete
		return parseSelectorExpr(v)
	case *ast.UnaryExpr:
		if v.Op == token.AND {
			return parseExpr(v.X, schema)
		}
		return nil
	default:
		return nil
	}
}

func parseMapLit(cl *ast.CompositeLit, schema *devmodel.JsonSchema) map[string]interface{} {
	m := make(map[string]interface{})
	valueSchema := schema.AdditionalProperties

	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		// parse key
		var key string
		switch k := kv.Key.(type) {
		case *ast.BasicLit:
			if k.Kind == token.STRING {
				key, _ = strconv.Unquote(k.Value)
			} else {
				key = k.Value
			}
		case *ast.Ident:
			key = k.Name
		default:
			continue
		}

		// parse value
		var value interface{}
		switch v := kv.Value.(type) {
		case *ast.CompositeLit:
			if valueSchema.Type == devmodel.JsonTypeOfObject && valueSchema.AdditionalProperties != nil {
				value = parseMapLit(v, valueSchema)
			} else if valueSchema.Type == devmodel.JsonTypeOfArray {
				value = parseArrayLit(v, valueSchema)
			} else {
				value = parseCompositeLit(v, valueSchema)
			}
		default:
			value = parseExpr(kv.Value, valueSchema)
		}
		m[key] = value
	}
	return m
}

func buildFieldTagMap(schema *devmodel.JsonSchema) map[string]*FieldInfo {
	fieldTagMap := make(map[string]*FieldInfo)
	for jsonKey, propSchema := range schema.Properties {
		if propSchema.Description == "" {
			continue
		}

		fieldName := propSchema.Description
		fieldTagMap[fieldName] = &FieldInfo{
			JSONKey: jsonKey,
			Schema:  propSchema,
		}
	}
	return fieldTagMap
}

func parseArrayLit(cl *ast.CompositeLit, schema *devmodel.JsonSchema) []interface{} {
	var arr []interface{}
	itemSchema := schema.Items
	for _, elt := range cl.Elts {
		value := parseExpr(elt, itemSchema)
		arr = append(arr, value)
	}
	return arr
}

func parseBasicLit(bl *ast.BasicLit) interface{} {
	switch bl.Kind {
	case token.STRING:
		str, _ := strconv.Unquote(bl.Value)
		return str
	case token.INT:
		i, _ := strconv.Atoi(bl.Value)
		return i
	case token.FLOAT:
		f, _ := strconv.ParseFloat(bl.Value, 64)
		return f
	case token.CHAR:
		// convert a character to its Unicode code point value.
		str, _ := strconv.Unquote(strings.Replace(bl.Value, "'", "\"", -1))
		if len(str) > 0 {
			return int(str[0])
		}
		return 0
	default:
		return bl.Value
	}
}

func parseSelectorExpr(se *ast.SelectorExpr) string {
	x, ok := se.X.(*ast.Ident)
	if !ok {
		return se.Sel.Name
	}
	return x.Name + "." + se.Sel.Name
}

func convertToValue(src interface{}, dst reflect.Value) error {
	if src == nil {
		return nil
	}

	switch dst.Kind() {
	case reflect.Struct:
		srcMap, ok := src.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected map for struct, got %T", src)
		}
		for k, v := range srcMap {
			field := dst.FieldByName(k)
			if !field.IsValid() {
				continue
			}
			if err := convertToValue(v, field); err != nil {
				return err
			}
		}
	case reflect.Map:
		srcMap, ok := src.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected map, got %T", src)
		}
		if dst.IsNil() {
			dst.Set(reflect.MakeMap(dst.Type()))
		}
		for k, v := range srcMap {
			newVal := reflect.New(dst.Type().Elem()).Elem()
			if err := convertToValue(v, newVal); err != nil {
				return err
			}
			dst.SetMapIndex(reflect.ValueOf(k), newVal)
		}
	case reflect.Ptr:
		// 创建一个新的指针
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		// 递归处理指针指向的值
		return convertToValue(src, dst.Elem())

	case reflect.Slice, reflect.Array:
		srcSlice, ok := src.([]interface{})
		if !ok {
			return fmt.Errorf("expected slice, got %T", src)
		}
		slice := reflect.MakeSlice(dst.Type(), len(srcSlice), len(srcSlice))
		for i, v := range srcSlice {
			if slice.Index(i).Kind() == reflect.Ptr {
				elem := reflect.New(slice.Index(i).Type().Elem())
				if err := convertToValue(v, elem.Elem()); err != nil {
					return err
				}
				slice.Index(i).Set(elem)
			} else {
				if err := convertToValue(v, slice.Index(i)); err != nil {
					return err
				}
			}
		}
		dst.Set(slice)
	case reflect.String:
		str, ok := src.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", src)
		}
		dst.SetString(str)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		num, ok := src.(int)
		if !ok {
			return fmt.Errorf("expected int, got %T", src)
		}
		dst.SetInt(int64(num))
	case reflect.Float32, reflect.Float64:
		floatVal, ok := src.(float64)
		if !ok {
			return fmt.Errorf("expected float, got %T", src)
		}
		dst.SetFloat(floatVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		intVal, ok := src.(int)
		if !ok {
			return fmt.Errorf("expected uint, got %T", src)
		}
		dst.SetUint(uint64(intVal))

	case reflect.Bool:
		b, ok := src.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %T", src)
		}
		dst.SetBool(b)
	case reflect.Interface:
		if src == nil {
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}

		srcValue := reflect.ValueOf(src)

		// 如果源值可以直接赋值给目标接口
		if srcValue.Type().AssignableTo(dst.Type()) {
			dst.Set(srcValue)
			return nil
		}

		// 如果源值可以转换为目标接口类型
		if srcValue.Type().ConvertibleTo(dst.Type()) {
			dst.Set(srcValue.Convert(dst.Type()))
			return nil
		}

		return fmt.Errorf("cannot convert %T to interface type %v", src, dst.Type())
	default:
		return fmt.Errorf("unhandled case, src is %T, interface type %v", src, dst.Type())
	}
	return nil
}
