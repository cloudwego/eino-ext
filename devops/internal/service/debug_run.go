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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	model2 "github.com/cloudwego/eino-ext/devops/model"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"sync"

	"github.com/google/uuid"

	"github.com/cloudwego/eino-ext/devops/internal/model"
	"github.com/cloudwego/eino-ext/devops/internal/utils/log"
	"github.com/cloudwego/eino-ext/devops/internal/utils/safego"
	"github.com/cloudwego/eino/compose"
)

// TODO@liujian: implement debug run service

var _ DebugService = &debugServiceImpl{}

//go:generate mockgen -source=debug_run.go -destination=../mock/debug_run_mock.go -package=mock
type DebugService interface {
	CreateDebugThread(ctx context.Context, graphID string) (threadID string, err error)
	DebugRun(ctx context.Context, m *model.DebugRunMeta, userInput string) (debugID string, stateCh chan *model.NodeDebugState, errCh chan error, err error)
}

type debugServiceImpl struct {
	mu sync.RWMutex
	// debugGraphs: graphID vs DebugGraph
	debugGraphs map[string]*model.DebugGraph
}

func newDebugService() DebugService {
	return &debugServiceImpl{
		mu:          sync.RWMutex{},
		debugGraphs: make(map[string]*model.DebugGraph, 10),
	}
}

func (d *debugServiceImpl) CreateDebugThread(ctx context.Context, graphID string) (threadID string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	dg := d.debugGraphs[graphID]
	if dg == nil {
		dg = &model.DebugGraph{
			DT: make([]*model.DebugThread, 0, 10),
		}
		d.debugGraphs[graphID] = dg
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generate thread id failed, err=%w", err)
	}
	threadID = id.String()

	dg.DT = append(dg.DT, &model.DebugThread{ID: threadID})

	return threadID, nil
}

func (d *debugServiceImpl) DebugRun(ctx context.Context, rm *model.DebugRunMeta, userInput string) (debugID string,
	stateCh chan *model.NodeDebugState, errCh chan error, err error) {
	d.mu.RLock()
	dg := d.debugGraphs[rm.GraphID]
	if dg == nil {
		d.mu.RUnlock()
		return "", nil, nil, fmt.Errorf("graph=%s not exist", rm.GraphID)
	}
	d.mu.RUnlock()

	_, ok := dg.GetDebugThread(rm.ThreadID)
	if !ok {
		return "", nil, nil, fmt.Errorf("thread=%s not exist", rm.ThreadID)
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return "", nil, nil, err
	}
	debugID = id.String()

	devGraph, ok := ContainerSVC.GetDevGraph(rm.GraphID, rm.FromNode)
	if !ok {
		devGraph, err = ContainerSVC.CreateDevGraph(rm.GraphID, rm.FromNode)
		if err != nil {
			return "", nil, nil, fmt.Errorf("create runnable failed, err=%w", err)
		}
	}

	inputType := devGraph.GraphInfo.InputType
	if rm.FromNode != compose.START {
		fromNode, ok := devGraph.GraphInfo.Nodes[rm.FromNode]
		if !ok {
			return "", nil, nil, fmt.Errorf("node %s not found", rm.FromNode)
		}
		inputType = fromNode.InputType
	}

	nodeInfo, ok := ContainerSVC.GetNodeInfo(rm.GraphID, rm.FromNode)
	if !ok {
		return "", nil, nil, fmt.Errorf("graph %s, node_key %s not found", rm.GraphID, rm.FromNode)
	}

	userInput, err = getJsonDate(userInput, nodeInfo.ComponentSchema.InputType)
	if err != nil {
		return "", nil, nil, err
	}

	input, err := model.UnmarshalJson([]byte(userInput), inputType)
	if err != nil {
		return "", nil, nil, err
	}

	stateCh = make(chan *model.NodeDebugState, 100)

	opts, err := d.getInvokeOptions(devGraph.GraphInfo, rm.ThreadID, stateCh)
	if err != nil {
		close(stateCh)
		return "", nil, nil, fmt.Errorf("get invoke option failed, err=%w", err)
	}

	errCh = make(chan error, 1)
	safego.Go(ctx, func() {
		defer close(stateCh)
		defer close(errCh)

		r, e := devGraph.Compile()
		if e != nil {
			errCh <- e
			log.Errorf("Compile failed, fromNode=%s\nerr=%s", rm.FromNode, e)
			return
		}

		_, e = r.Invoke(ctx, input, opts...)
		if e != nil {
			errCh <- e
			log.Errorf("invoke failed, userInput=%s\nerr=%s", userInput, e)
			return
		}
	})

	return debugID, stateCh, errCh, nil
}

func (d *debugServiceImpl) getInvokeOptions(gi *model.GraphInfo, threadID string, stateCh chan *model.NodeDebugState) (opts []compose.Option, err error) {
	opts = make([]compose.Option, 0, len(gi.Nodes))
	for key, node := range gi.Nodes {
		opts = append(opts, newCallbackOption(key, threadID, node, stateCh))
	}

	return opts, nil
}

func getJsonDate(code string, schema *model2.JsonSchema) (jsonDate string, err error) {
	code = `
    var input = schema.Message{
		content: "hello from code",
		Role:    schema.User,
		Name:    "input",
		Num:     1,
		Extra: map[string]interface{}{
			"a": "b",
		},
		ToolCalls: []schema.ToolCall{
			{
				ID:   "1",
				Type: "1",
			},
		},
	}
    `

	// 1. 解析代码生成 AST
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", "package main\n"+code, parser.ParseComments)
	if err != nil {
		return "", err
	}

	// 2. 遍历 AST 提取信息
	var result map[string]interface{}
	ast.Inspect(node, func(n ast.Node) bool {
		// 查找变量声明
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, value := range vs.Values {
			var cl *ast.CompositeLit
			switch v := value.(type) {
			case *ast.UnaryExpr:
				// 指针类型
				cl, ok = v.X.(*ast.CompositeLit)
				if !ok {
					continue
				}
			case *ast.CompositeLit:
				// 非指针类型
				cl = v
			default:
				continue
			}
			result = parseCompositeLit(cl)
		}
		return false
	})

	// 3. 序列化为 JSON
	jsonData, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		return "", err
	}
	return string(jsonData), err
}

// 解析复合字面量
func parseCompositeLit(cl *ast.CompositeLit) map[string]interface{} {
	data := make(map[string]interface{})
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key := kv.Key.(*ast.Ident).Name
		value := parseExpr(kv.Value)
		data[key] = value
	}
	return data
}

// 解析表达式
func parseExpr(expr ast.Expr) interface{} {
	switch v := expr.(type) {
	case *ast.BasicLit:
		// 基本字面量，如字符串、数字
		return parseBasicLit(v)
	case *ast.CompositeLit:
		// 复合字面量，如结构体、数组、映射
		if _, ok := v.Type.(*ast.MapType); ok {
			// 映射类型
			return parseMapLit(v)
		} else if _, ok := v.Type.(*ast.ArrayType); ok {
			// 数组或切片类型
			return parseArrayLit(v)
		} else {
			// 结构体类型
			return parseCompositeLit(v)
		}
	case *ast.SelectorExpr:
		// 选择器表达式，如 schema.User
		return parseSelectorExpr(v)
	case *ast.UnaryExpr:
		// 一元表达式，处理取地址符号 &
		if v.Op == token.AND {
			return parseExpr(v.X)
		}
		return nil
	default:
		return nil
	}
}

// 解析基本字面量
func parseBasicLit(bl *ast.BasicLit) interface{} {
	switch bl.Kind {
	case token.STRING:
		// 去除引号
		str, _ := strconv.Unquote(bl.Value)
		return str
	case token.INT:
		i, _ := strconv.Atoi(bl.Value)
		return i
	default:
		return bl.Value
	}
}

// 解析选择器表达式
func parseSelectorExpr(se *ast.SelectorExpr) string {
	x, ok := se.X.(*ast.Ident)
	if !ok {
		return se.Sel.Name
	}
	return x.Name + "." + se.Sel.Name
}

// 解析映射字面量
func parseMapLit(cl *ast.CompositeLit) map[string]interface{} {
	m := make(map[string]interface{})
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key := parseExpr(kv.Key).(string)
		value := parseExpr(kv.Value)
		m[key] = value
	}
	return m
}

// 解析数组或切片字面量
func parseArrayLit(cl *ast.CompositeLit) []interface{} {
	var arr []interface{}
	for _, elt := range cl.Elts {
		value := parseExpr(elt)
		arr = append(arr, value)
	}
	return arr
}
