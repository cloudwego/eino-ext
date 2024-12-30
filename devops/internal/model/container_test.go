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
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/devops/internal/utils/generic"
	devmodel "github.com/cloudwego/eino-ext/devops/model"
)

type mockContainer interface {
	Name() string
}

type mockContainerV2 interface {
	Name() string
}

type mockContainerImpl struct {
	NN  string `json:"nn"`
	Age int    `json:"age"`
}

func (m mockContainerImpl) Name() string {
	return m.NN
}

type mockContainerImplV2 struct {
	NN  string `json:"nn"`
	Age int    `json:"age"`
}

func (m mockContainerImplV2) Name() string {
	return m.NN
}

type testCtxKey struct{}

type testCallback struct {
	gi       *GraphInfo
	genState func(ctx context.Context) any
}

func (tt *testCallback) OnFinish(ctx context.Context, graphInfo *compose.GraphInfo) {
	c, ok := ctx.Value(testCtxKey{}).(*testCallback)
	if !ok {
		return
	}
	c.gi = &GraphInfo{
		GraphInfo: graphInfo,
		Option: GraphOption{
			GenState: c.genState,
		},
	}
}

func Test_GraphInfo_BuildDevGraph(t *testing.T) {
	t.Run("graph-chain: add chain, stateGraph，graph node", func(t *testing.T) {
		type mockInputType struct {
			Input string `json:"input"`
		}

		g := compose.NewGraph[*mockInputType, []string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input *mockInputType) (output []string, err error) {
			return []string{input.Input, fmt.Sprintf("out_lambda_1")}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_2", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_2"))
			return output, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_3"))
			return output, nil
		}))
		assert.NoError(t, err)

		sg := compose.NewGraph[[]string, []string]()
		err = sg.AddLambdaNode("sg_node_1", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("sub_graph_out_lambda_1"))
			return output, nil
		}))
		assert.NoError(t, err)
		err = sg.AddEdge(compose.START, "sg_node_1")
		assert.NoError(t, err)
		err = sg.AddEdge("sg_node_1", compose.END)
		assert.NoError(t, err)
		err = g.AddGraphNode("node_4", sg)
		assert.NoError(t, err)

		sc := compose.NewChain[[]string, []string]()
		sc.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("sub_chain_out_lambda_1"))
			return output, nil
		}))
		err = g.AddGraphNode("node_5", sc)
		assert.NoError(t, err)

		ssg := compose.NewGraph[[]string, []string](compose.WithGenLocalState(func(ctx context.Context) (state string) {
			return ""
		}))
		err = ssg.AddLambdaNode("ssg_node_1", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("sub_state_graph_out_lambda_1"))
			return output, nil
		}))
		assert.NoError(t, err)
		err = ssg.AddEdge(compose.START, "ssg_node_1")
		assert.NoError(t, err)
		err = ssg.AddEdge("ssg_node_1", compose.END)
		assert.NoError(t, err)
		err = g.AddGraphNode("node_6", ssg)
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_2")
		assert.NoError(t, err)
		err = g.AddEdge("node_2", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", "node_4")
		assert.NoError(t, err)
		err = g.AddEdge("node_4", "node_5")
		assert.NoError(t, err)
		err = g.AddEdge("node_5", "node_6")
		assert.NoError(t, err)
		err = g.AddEdge("node_6", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `{"input":"mock_input"}`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		respStr, err := json.Marshal(resp)
		assert.NoError(t, err)

		ground := []string{"mock_input", "out_lambda_1", "out_lambda_2", "out_lambda_3", "sub_graph_out_lambda_1", "sub_chain_out_lambda_1", "sub_state_graph_out_lambda_1"}
		groundStr, err := json.Marshal(ground)
		assert.NoError(t, err)

		assert.Equal(t, groundStr, respStr)
	})

	t.Run("graph-chain: input with inputKey", func(t *testing.T) {
		type mockInputType struct {
			Input string   `json:"input"`
			Array []string `json:"array"`
		}

		g := compose.NewGraph[map[string]any, []string]()
		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input *mockInputType) (output []string, err error) {
			output = append(input.Array, input.Input, fmt.Sprintf("out_A"))
			return output, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)
		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_B"))
			return output, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_C"))
			return output, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge("A", "B")
		assert.NoError(t, err)
		err = g.AddEdge("B", "C")
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `{"A":{"input":"mock_input_3", "array":["mock_input_1", "mock_input_2"]}}`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.NoError(t, err)

		respStr, err := json.Marshal(resp)
		assert.NoError(t, err)

		ground := []string{"mock_input_1", "mock_input_2", "mock_input_3", "out_A", "out_B", "out_C"}
		groundStr, err := json.Marshal(ground)
		assert.NoError(t, err)

		assert.Equal(t, groundStr, respStr)
	})

	t.Run("graph-chain: input struct", func(t *testing.T) {
		type mockInputType struct {
			Input string `json:"input"`
		}

		g := compose.NewGraph[*mockInputType, []string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input *mockInputType) (output []string, err error) {
			return []string{input.Input, fmt.Sprintf("out_lambda_1")}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_2", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_2"))
			return output, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_3"))
			return output, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_2")
		assert.NoError(t, err)
		err = g.AddEdge("node_2", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `{"input":"mock_input"}`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		respStr, err := json.Marshal(resp)
		assert.NoError(t, err)

		ground := []string{"mock_input", "out_lambda_1", "out_lambda_2", "out_lambda_3"}
		groundStr, err := json.Marshal(ground)
		assert.NoError(t, err)

		assert.Equal(t, groundStr, respStr)
	})

	t.Run("graph-chain: first node is branch", func(t *testing.T) {
		g := compose.NewGraph[int, []string]()
		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input int) (output []string, err error) {
			return []string{strconv.Itoa(input), fmt.Sprintf("out_A")}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input int) (output []string, err error) {
			return []string{strconv.Itoa(input), fmt.Sprintf("out_B")}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_C"))
			return output, nil
		}))
		assert.NoError(t, err)

		b := compose.NewGraphBranch(func(ctx context.Context, input int) (to string, err error) {
			if input < 0 {
				return "A", nil
			}
			return "B", nil
		}, map[string]bool{
			"A": true,
			"B": true,
		})

		err = g.AddBranch(compose.START, b)
		assert.NoError(t, err)
		err = g.AddEdge("A", "C")
		assert.NoError(t, err)
		err = g.AddEdge("B", "C")
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `-1`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		respStr, err := json.Marshal(resp)
		assert.NoError(t, err)

		ground := []string{"-1", "out_A", "out_C"}
		groundStr, err := json.Marshal(ground)
		assert.NoError(t, err)

		assert.Equal(t, groundStr, respStr)
	})

	t.Run("graph-chain: input map[string]string", func(t *testing.T) {
		g := compose.NewGraph[map[string]string, []string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input map[string]string) (output []string, err error) {
			return []string{input["input"], fmt.Sprintf("out_lambda_1")}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_2", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_2"))
			return output, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_3"))
			return output, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_2")
		assert.NoError(t, err)
		err = g.AddEdge("node_2", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `{"input":"mock_input"}`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		respStr, err := json.Marshal(resp)
		assert.NoError(t, err)

		ground := []string{"mock_input", "out_lambda_1", "out_lambda_2", "out_lambda_3"}
		groundStr, err := json.Marshal(ground)
		assert.NoError(t, err)

		assert.Equal(t, groundStr, respStr)
	})

	t.Run("graph-chain: input plain text", func(t *testing.T) {
		g := compose.NewGraph[string, []string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input string) (output []string, err error) {
			return []string{input, fmt.Sprintf("out_lambda_1")}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_2", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_2"))
			return output, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_3"))
			return output, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_2")
		assert.NoError(t, err)
		err = g.AddEdge("node_2", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `mock_input`
		userMsgMarshal, _ := json.Marshal(userMsg)
		input, err := ift.UnmarshalJson(string(userMsgMarshal))
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		respStr, err := json.Marshal(resp)
		assert.NoError(t, err)

		ground := []string{"mock_input", "out_lambda_1", "out_lambda_2", "out_lambda_3"}
		groundStr, err := json.Marshal(ground)
		assert.NoError(t, err)

		assert.Equal(t, groundStr, respStr)
	})

	t.Run("graph-chain: input int", func(t *testing.T) {
		g := compose.NewGraph[int, []string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input int) (output []string, err error) {
			return []string{strconv.Itoa(input), fmt.Sprintf("out_lambda_1")}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_2", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_2"))
			return output, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_3"))
			return output, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_2")
		assert.NoError(t, err)
		err = g.AddEdge("node_2", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `1`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		respStr, err := json.Marshal(resp)
		assert.NoError(t, err)

		ground := []string{"1", "out_lambda_1", "out_lambda_2", "out_lambda_3"}
		groundStr, err := json.Marshal(ground)
		assert.NoError(t, err)

		assert.Equal(t, groundStr, respStr)
	})

	t.Run("stateGraph-chain", func(t *testing.T) {
		genState := func(ctx context.Context) any {
			return func(ctx context.Context) map[string]any {
				t.Log("enter gen state")
				return map[string]any{}
			}(ctx)
		}
		tc := &testCallback{
			genState: genState,
		}

		g := compose.NewGraph[int, []string](compose.WithGenLocalState(genState))
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input int) (output []string, err error) {
			return []string{strconv.Itoa(input), fmt.Sprintf("out_lambda_1")}, nil
		}))
		assert.NoError(t, err)

		ph := func(ctx context.Context, in []string, state map[string]any) ([]string, error) {
			t.Log("enter pre handler")
			return in, nil
		}
		err = g.AddLambdaNode("node_2", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_2"))
			return output, nil
		}), compose.WithStatePreHandler(ph))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_3"))
			return output, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_2")
		assert.NoError(t, err)
		err = g.AddEdge("node_2", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.NoError(t, err)

		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `1`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		respStr, err := json.Marshal(resp)
		assert.NoError(t, err)

		ground := []string{"1", "out_lambda_1", "out_lambda_2", "out_lambda_3"}
		groundStr, err := json.Marshal(ground)
		assert.NoError(t, err)

		assert.Equal(t, groundStr, respStr)
	})

	t.Run("graph-parallel", func(t *testing.T) {
		g := compose.NewGraph[int, map[string][]string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input int) (output []string, err error) {
			return []string{strconv.Itoa(input), fmt.Sprintf("out_lambda_1")}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_21", compose.InvokableLambda(func(ctx context.Context, input []string) (output map[string][]string, err error) {
			return map[string][]string{
				"node_21": append(input, fmt.Sprintf("out_lambda_21")),
			}, nil
		}))
		err = g.AddLambdaNode("node_22", compose.InvokableLambda(func(ctx context.Context, input []string) (output map[string][]string, err error) {
			return map[string][]string{
				"node_22": append(input, fmt.Sprintf("out_lambda_22")),
			}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input map[string][]string) (output map[string][]string, err error) {
			return map[string][]string{
				"out_lambda_3": input["node_21"],
			}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_4", compose.InvokableLambda(func(ctx context.Context, input map[string][]string) (output map[string][]string, err error) {
			return map[string][]string{
				"out_lambda_4": input["node_22"],
			}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_5", compose.InvokableLambda(func(ctx context.Context, input map[string][]string) (output map[string][]string, err error) {
			return map[string][]string{
				"out_lambda_5": append(input["node_21"], input["node_22"]...),
			}, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_21")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_22")
		assert.NoError(t, err)
		err = g.AddEdge("node_21", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_22", "node_4")
		assert.NoError(t, err)
		err = g.AddEdge("node_21", "node_5")
		assert.NoError(t, err)
		err = g.AddEdge("node_22", "node_5")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("node_4", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("node_5", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `1`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		_, err = json.Marshal(resp)
		assert.NoError(t, err)

		ground := map[string][]string{
			"out_lambda_3": {"1", "out_lambda_1", "out_lambda_21"},
			"out_lambda_4": {"1", "out_lambda_1", "out_lambda_22"},
			"out_lambda_5": {"1", "out_lambda_1", "out_lambda_21", "1", "out_lambda_1", "out_lambda_22"},
		}

		assert.True(t, reflect.DeepEqual(ground, resp))
	})

	t.Run("graph-parallel: has compositeType", func(t *testing.T) {
		g := compose.NewGraph[mockContainerImpl, map[string][]string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input mockContainerImpl) (output mockContainer, err error) {
			assert.Equal(t, input.NN, "start")
			assert.Equal(t, input.Age, -1)

			return mockContainerImpl{
				NN:  "node_1",
				Age: 1,
			}, nil
		}), compose.WithOutputKey("node_1"))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_2", compose.InvokableLambda(func(ctx context.Context, input mockContainer) (output mockContainer, err error) {
			return mockContainerImpl{
				NN:  "node_2",
				Age: 1,
			}, nil
		}), compose.WithOutputKey("node_2"))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input map[string]any) (output mockContainerImpl, err error) {
			assert.Equal(t, input["node_1"].(mockContainerImpl).NN, "node_1")
			assert.Equal(t, input["node_2"].(mockContainerImpl).NN, "node_2")
			return mockContainerImpl{
				NN:  "node_3",
				Age: 1,
			}, nil
		}))
		assert.NoError(t, err)

		sg := compose.NewGraph[mockContainerImpl, map[string][]string]()
		err = sg.AddLambdaNode("sub_node_1", compose.InvokableLambda(func(ctx context.Context, input mockContainerImpl) (output map[string]any, err error) {
			return map[string]any{
				"sub_node_1": []string{input.NN, fmt.Sprintf("sub_out_lambda_1")},
			}, nil
		}))
		err = sg.AddLambdaNode("sub_node_2", compose.InvokableLambda(func(ctx context.Context, input mockContainer) (output map[string]any, err error) {
			return map[string]any{
				"sub_node_2": []string{input.Name(), fmt.Sprintf("sub_out_lambda_2")},
			}, nil
		}))
		assert.NoError(t, err)
		err = sg.AddLambdaNode("sub_node_3", compose.InvokableLambda(func(ctx context.Context, input map[string]any) (output map[string][]string, err error) {
			output = map[string][]string{}
			for k, v := range input {
				output[k] = v.([]string)
			}
			return output, nil
		}))
		assert.NoError(t, err)

		err = sg.AddEdge(compose.START, "sub_node_1")
		assert.NoError(t, err)
		err = sg.AddEdge(compose.START, "sub_node_2")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_node_1", "sub_node_3")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_node_2", "sub_node_3")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_node_3", compose.END)
		assert.NoError(t, err)

		err = g.AddGraphNode("sub_graph", sg)
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "node_2")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_2", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", "sub_graph")
		assert.NoError(t, err)
		err = g.AddEdge("sub_graph", compose.END)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `{"nn": "start", "age": -1}`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		_, err = json.Marshal(resp)
		assert.NoError(t, err)

		ground := map[string][]string{
			"sub_node_1": {"node_3", "sub_out_lambda_1"},
			"sub_node_2": {"node_3", "sub_out_lambda_2"},
		}

		assert.True(t, reflect.DeepEqual(ground, resp))
	})

	t.Run("graph-parallel: start from here, input is struct type", func(t *testing.T) {
		type testTyp struct {
			Node []string
		}

		g := compose.NewGraph[int, map[string][]string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input int) (output *testTyp, err error) {
			return &testTyp{
				Node: []string{"node_1"},
			}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_21", compose.InvokableLambda(func(ctx context.Context, input *testTyp) (output *testTyp, err error) {
			input.Node = append(input.Node, "node_21")
			return input, nil
		}))
		err = g.AddLambdaNode("node_22", compose.InvokableLambda(func(ctx context.Context, input *testTyp) (output *testTyp, err error) {
			input.Node = append(input.Node, "node_22")
			return output, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input *testTyp) (output map[string][]string, err error) {
			return map[string][]string{
				"out_lambda_3": append(input.Node, "node_3"),
			}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_4", compose.InvokableLambda(func(ctx context.Context, input *testTyp) (output map[string][]string, err error) {
			return map[string][]string{
				"out_lambda_4": append(input.Node, "node_4"),
			}, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_21")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_22")
		assert.NoError(t, err)
		err = g.AddEdge("node_21", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_22", "node_4")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("node_4", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph("node_21")
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType("node_21")
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `{"node":["from_node_21"]}`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		ground := map[string][]string{
			"out_lambda_3": {"from_node_21", "node_21", "node_3"},
		}

		assert.True(t, reflect.DeepEqual(ground, resp))
	})

	t.Run("graph-chain: start from here", func(t *testing.T) {
		g := compose.NewGraph[int, []string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input int) (output []string, err error) {
			return []string{strconv.Itoa(input), fmt.Sprintf("out_lambda_1")}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_2", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_2"))
			return output, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_3"))
			return output, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_2")
		assert.NoError(t, err)
		err = g.AddEdge("node_2", "node_3")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		ng, err := tc.gi.BuildDevGraph("node_2")
		assert.NoError(t, err)

		r, err := ng.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		ift, ok, err := tc.gi.InferGraphInputType("node_2")
		assert.NoError(t, err)
		assert.True(t, ok)

		userMsg := `["from_node_2"]`
		input, err := ift.UnmarshalJson(userMsg)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)

		respStr, err := json.Marshal(resp)
		assert.NoError(t, err)

		ground := []string{"from_node_2", "out_lambda_2", "out_lambda_3"}
		groundStr, err := json.Marshal(ground)
		assert.NoError(t, err)

		assert.Equal(t, groundStr, respStr)
	})

	t.Run("graph-branch: start from here, to_end", func(t *testing.T) {
		g := compose.NewGraph[int, int]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input int) (output []int, err error) {
			return []int{input}, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_21", compose.InvokableLambda(func(ctx context.Context, input []int) (output int, err error) {
			return input[0], nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_22", compose.InvokableLambda(func(ctx context.Context, input []int) (output int, err error) {
			return input[0], nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input int) (output int, err error) {
			return 333, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("node_4", compose.InvokableLambda(func(ctx context.Context, input int) (output int, err error) {
			return 4444, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_21")
		assert.NoError(t, err)
		err = g.AddEdge("node_1", "node_22")
		assert.NoError(t, err)

		branch1 := compose.NewGraphBranch(
			func(ctx context.Context, input int) (output string, err error) {
				if input > 10000 {
					return "node_3", nil
				}
				return "end", nil
			},
			map[string]bool{
				"node_3": true,
				"end":    true,
			},
		)

		err = g.AddBranch("node_21", branch1)
		assert.NoError(t, err)
		err = g.AddEdge("node_22", "node_4")
		assert.NoError(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("node_4", compose.END)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		t.Run("graph-branch: to_node_3", func(t *testing.T) {
			_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
			assert.NoError(t, err)

			ng, err := tc.gi.BuildDevGraph("node_21")
			assert.NoError(t, err)

			r, err := ng.Compile(tc.gi.CompileOptions...)
			assert.NoError(t, err)

			ift, ok, err := tc.gi.InferGraphInputType("node_21")
			assert.NoError(t, err)
			assert.True(t, ok)

			userMsg := `[20000]`
			input, err := ift.UnmarshalJson(userMsg)
			assert.NoError(t, err)
			resp, err := r.Invoke(ctx, input)

			respStr, err := json.Marshal(resp)
			assert.NoError(t, err)

			assert.Equal(t, string(respStr), "333")
		})

		t.Run("graph-branch: to_end", func(t *testing.T) {
			_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
			assert.NoError(t, err)

			ng, err := tc.gi.BuildDevGraph("node_21")
			assert.NoError(t, err)

			r, err := ng.Compile(tc.gi.CompileOptions...)
			assert.NoError(t, err)

			ift, ok, err := tc.gi.InferGraphInputType("node_21")
			assert.NoError(t, err)
			assert.True(t, ok)

			userMsg := `[1]`
			input, err := ift.UnmarshalJson(userMsg)
			assert.NoError(t, err)
			resp, err := r.Invoke(ctx, input)

			respStr, err := json.Marshal(resp)
			assert.NoError(t, err)

			assert.Equal(t, string(respStr), "1")
		})
	})

	t.Run("graph-branch: branch loop", func(t *testing.T) {
		g := compose.NewGraph[int, int]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input int) (output int, err error) {
			return input, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input int) (output int, err error) {
			return input, nil
		}))
		assert.NoError(t, err)
		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input int) (output int, err error) {
			return 1, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge("B", "C")
		assert.NoError(t, err)
		err = g.AddEdge("C", "B")
		assert.NoError(t, err)

		branch1 := compose.NewGraphBranch(
			func(ctx context.Context, input int) (output string, err error) {
				if input < 0 {
					return "B", nil
				}
				return "end", nil
			},
			map[string]bool{
				"B":   true,
				"end": true,
			},
		)
		err = g.AddBranch("A", branch1)
		assert.NoError(t, err)

		branch2 := compose.NewGraphBranch(
			func(ctx context.Context, input int) (output string, err error) {
				if input < 0 {
					return "C", nil
				}
				return "A", nil
			},
			map[string]bool{
				"C": true,
				"A": true,
			},
		)
		err = g.AddBranch("B", branch2)
		assert.NoError(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		t.Run("graph-branch: start from here, C", func(t *testing.T) {
			_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
			assert.NoError(t, err)

			ng, err := tc.gi.BuildDevGraph("C")
			assert.NoError(t, err)

			r, err := ng.Compile(tc.gi.CompileOptions...)
			assert.NoError(t, err)

			ift, ok, err := tc.gi.InferGraphInputType("C")
			assert.NoError(t, err)
			assert.True(t, ok)

			userMsg := `20000`
			input, err := ift.UnmarshalJson(userMsg)
			assert.NoError(t, err)
			resp, err := r.Invoke(ctx, input)

			respStr, err := json.Marshal(resp)
			assert.NoError(t, err)

			assert.Equal(t, string(respStr), "1")
		})

		t.Run("graph-branch: start from here, start", func(t *testing.T) {
			_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
			assert.NoError(t, err)

			ng, err := tc.gi.BuildDevGraph(compose.START)
			assert.NoError(t, err)

			r, err := ng.Compile(tc.gi.CompileOptions...)
			assert.NoError(t, err)

			ift, ok, err := tc.gi.InferGraphInputType(compose.START)
			assert.NoError(t, err)
			assert.True(t, ok)

			userMsg := `-1`
			input, err := ift.UnmarshalJson(userMsg)
			assert.NoError(t, err)
			resp, err := r.Invoke(ctx, input)

			respStr, err := json.Marshal(resp)
			assert.NoError(t, err)

			assert.Equal(t, string(respStr), "1")
		})

	})

	t.Run("chain-parallel: start from here", func(t *testing.T) {
		c := compose.NewChain[int, map[string]any]()
		c.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input int) (output []string, err error) {
			return []string{strconv.Itoa(input), fmt.Sprintf("out_lambda_1")}, nil
		}))
		c.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_21"))
			return output, nil
		}))
		c.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_22"))
			return output, nil
		}))
		par := compose.NewParallel()
		par.AddLambda("out_lambda_3", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			return input, nil
		}))
		par.AddLambda("out_lambda_4", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			return input, nil
		}))
		c.AppendParallel(par)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err := c.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		t.Run("Chain[2]_Lambda", func(t *testing.T) {
			newChain, err := tc.gi.BuildDevGraph("Chain[2]_Lambda")
			assert.NoError(t, err)

			r, err := newChain.Compile(tc.gi.CompileOptions...)
			assert.NoError(t, err)

			ift, ok, err := tc.gi.InferGraphInputType("Chain[2]_Lambda")
			assert.NoError(t, err)
			assert.True(t, ok)

			userMsg := `["from_Chain[2]_Lambda"]`
			input, err := ift.UnmarshalJson(userMsg)
			assert.NoError(t, err)
			resp, err := r.Invoke(ctx, input)

			_, err = json.Marshal(resp)
			assert.NoError(t, err)

			ground := map[string]any{
				"out_lambda_3": []string{"from_Chain[2]_Lambda", "out_lambda_22"},
				"out_lambda_4": []string{"from_Chain[2]_Lambda", "out_lambda_22"},
			}

			assert.True(t, reflect.DeepEqual(ground, resp))
		})

		t.Run("Chain[3]_Parallel[0]_Lambda", func(t *testing.T) {
			newChain, err := tc.gi.BuildDevGraph("Chain[3]_Parallel[0]_Lambda")
			assert.NoError(t, err)

			r, err := newChain.Compile(tc.gi.CompileOptions...)
			assert.NoError(t, err)

			ift, ok, err := tc.gi.InferGraphInputType("Chain[3]_Parallel[0]_Lambda")
			assert.NoError(t, err)
			assert.True(t, ok)

			userMsg := `["from_p0"]`
			input, err := ift.UnmarshalJson(userMsg)
			assert.NoError(t, err)
			resp, err := r.Invoke(ctx, input)

			_, err = json.Marshal(resp)
			assert.NoError(t, err)

			ground := map[string]any{
				"out_lambda_3": []string{"from_p0"},
			}

			assert.True(t, reflect.DeepEqual(ground, resp))
		})

		t.Run("Chain[3]_Parallel[1]_Lambda", func(t *testing.T) {
			newChain, err := tc.gi.BuildDevGraph("Chain[3]_Parallel[1]_Lambda")
			assert.NoError(t, err)

			r, err := newChain.Compile(tc.gi.CompileOptions...)
			assert.NoError(t, err)

			ift, ok, err := tc.gi.InferGraphInputType("Chain[3]_Parallel[1]_Lambda")
			assert.NoError(t, err)
			assert.True(t, ok)

			userMsg := `["from_p1"]`
			input, err := ift.UnmarshalJson(userMsg)
			assert.NoError(t, err)
			resp, err := r.Invoke(ctx, input)

			_, err = json.Marshal(resp)
			assert.NoError(t, err)

			ground := map[string]any{
				"out_lambda_4": []string{"from_p1"},
			}

			assert.True(t, reflect.DeepEqual(ground, resp))
		})
	})

	t.Run("chain-branch: start from here", func(t *testing.T) {
		c := compose.NewChain[int, map[string][]string]()
		c.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input int) (output []string, err error) {
			return []string{strconv.Itoa(input), fmt.Sprintf("out_lambda_1")}, nil
		}))

		c.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_21"))
			return output, nil
		}))
		c.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_22"))
			return output, nil
		}))

		branchCond := func(ctx context.Context, input []string) (string, error) {
			if input[0] == "b1" {
				return "b1", nil
			} else {
				return "b2", nil
			}
		}
		b1 := compose.InvokableLambda(func(ctx context.Context, input []string) (output map[string][]string, err error) {
			return map[string][]string{
				"out_lambda_3": input,
			}, nil
		})
		b2 := compose.InvokableLambda(func(ctx context.Context, input []string) (output map[string][]string, err error) {
			return map[string][]string{
				"out_lambda_4": input,
			}, nil
		})
		c.AppendBranch(compose.NewChainBranch[[]string](branchCond).AddLambda("b1", b1).AddLambda("b2", b2))

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err := c.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		t.Run("Chain[3]_Branch[b1]_Lambda", func(t *testing.T) {
			newChain, err := tc.gi.BuildDevGraph("Chain[3]_Branch[b1]_Lambda")
			assert.NoError(t, err)

			r, err := newChain.Compile(tc.gi.CompileOptions...)
			assert.NoError(t, err)

			ift, ok, err := tc.gi.InferGraphInputType("Chain[3]_Branch[b1]_Lambda")
			assert.NoError(t, err)
			assert.True(t, ok)

			userMsg := `["b1"]`
			input, err := ift.UnmarshalJson(userMsg)
			assert.NoError(t, err)
			resp, err := r.Invoke(ctx, input)

			_, err = json.Marshal(resp)
			assert.NoError(t, err)

			ground := map[string][]string{
				"out_lambda_3": {"b1"},
			}

			assert.True(t, reflect.DeepEqual(ground, resp))
		})

		t.Run("Chain[3]_Branch[b2]_Lambda", func(t *testing.T) {
			newChain, err := tc.gi.BuildDevGraph("Chain[3]_Branch[b2]_Lambda")
			assert.NoError(t, err)

			r, err := newChain.Compile(tc.gi.CompileOptions...)
			assert.NoError(t, err)

			ift, ok, err := tc.gi.InferGraphInputType("Chain[3]_Branch[b2]_Lambda")
			assert.NoError(t, err)
			assert.True(t, ok)

			userMsg := `["b2"]`
			input, err := ift.UnmarshalJson(userMsg)
			assert.NoError(t, err)
			resp, err := r.Invoke(ctx, input)

			_, err = json.Marshal(resp)
			assert.NoError(t, err)

			ground := map[string][]string{
				"out_lambda_4": {"b2"},
			}

			assert.True(t, reflect.DeepEqual(ground, resp))
		})
	})
}

func Test_Graph_addNode(t *testing.T) {
	genState := func(ctx context.Context) any {
		return nil
	}

	t.Run("instance not Embedding", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: components.ComponentOfEmbedding,
			Instance:  nil,
		}

		err := g.addNode("node_1", gni)
		assert.Contains(t, err.Error(), "but get unexpected instance")
	})

	t.Run("instance not Retriever", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: components.ComponentOfRetriever,
			Instance:  nil,
		}

		err := g.addNode("node_1", gni)
		assert.Contains(t, err.Error(), "but get unexpected instance")
	})

	t.Run("instance not LoaderSplitter", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: components.ComponentOfLoaderSplitter,
			Instance:  nil,
		}
		err := g.addNode("node_1", gni)
		assert.Contains(t, err.Error(), "but get unexpected instance")
	})

	t.Run("instance not Indexer", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: components.ComponentOfIndexer,
			Instance:  nil,
		}
		err := g.addNode("node_1", gni)
		assert.Contains(t, err.Error(), "but get unexpected instance")
	})

	t.Run("instance not ChatModel", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: components.ComponentOfChatModel,
			Instance:  nil,
		}
		err := g.addNode("node_1", gni)
		assert.Contains(t, err.Error(), "but get unexpected instance")
	})

	t.Run("Prompt", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: components.ComponentOfPrompt,
			Instance:  &prompt.DefaultChatTemplate{},
		}
		err := g.addNode("node_1", gni)
		assert.NoError(t, err)
	})

	t.Run("ToolsNode", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: compose.ComponentOfToolsNode,
			Instance:  &compose.ToolsNode{},
		}
		err := g.addNode("node_1", gni)
		assert.NoError(t, err)
	})

	t.Run("Graph", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: compose.ComponentOfStateGraph,
			Instance:  compose.NewGraph[string, string](),
		}
		err := g.addNode("node_1", gni)
		assert.NoError(t, err)
	})

	t.Run("Chain", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: compose.ComponentOfChain,
			Instance:  compose.NewChain[string, string](),
		}
		err := g.addNode("node_1", gni)
		assert.NoError(t, err)
	})

	t.Run("StateGraph", func(t *testing.T) {
		g := &Graph{Graph: compose.NewGraph[any, any](compose.WithGenLocalState(genState))}
		gni := compose.GraphNodeInfo{
			Component: compose.ComponentOfStateGraph,
			Instance: compose.NewGraph[string, string](compose.WithGenLocalState(func(ctx context.Context) string {
				return ""
			})),
		}
		err := g.addNode("node_1", gni)
		assert.NoError(t, err)
	})
}

type canvasCallBack struct {
	t *testing.T
}

func (c *canvasCallBack) OnFinish(ctx context.Context, info *compose.GraphInfo) {
	t := c.t

	notAllowOperateNodes := map[string]bool{
		"node2":  true,
		"node31": true,
		"start":  false,
		"end":    true,
	}
	g := GraphInfo{
		GraphInfo: info,
	}
	graphSchema, err := g.BuildGraphSchema()
	assert.NoError(t, err)
	assert.Equal(t, 15, len(graphSchema.Nodes))
	for _, edge := range graphSchema.Edges {
		names := strings.Split(edge.Name, "_to_")
		assert.Equal(t, names[0], edge.SourceNodeKey)
		assert.Equal(t, names[1], edge.TargetNodeKey)
	}
	for _, node := range graphSchema.Nodes {
		if ok := notAllowOperateNodes[node.Name]; ok {
			assert.False(t, node.AllowOperate)
		}
		if node.GraphSchema != nil {
			for _, n := range node.GraphSchema.Nodes {
				assert.False(t, n.AllowOperate)
			}
		}
	}

	return
}

func TestGraphInfo_BuildCanvas(t *testing.T) {
	g, err := newGraph()
	assert.NoError(t, err)
	_, err = g.Compile(context.Background(), compose.WithGraphCompileCallbacks(&canvasCallBack{t: t}))
	assert.NoError(t, err)
}

type canvasCallbackStruct struct {
	Name string `json:"name"`
	Job  struct {
		JobName string `json:"job_name"`
	} `json:"job"`
}
type canvasCallbackInterface interface {
	GetName() string
}

func (*canvasCallbackStruct) GetName() string {
	return "canvasCallbackStruct"
}

// Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error)
type mockRetrieve struct {
}

func (*mockRetrieve) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	return []*schema.Document{}, nil
}

func newGraph() (g *compose.Graph[map[string]any, any], err error) {
	g = compose.NewGraph[map[string]any, any]()

	node1 := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return input, nil
	})

	node2 := compose.InvokableLambda(func(ctx context.Context, input any) (*canvasCallbackStruct, error) {
		return &canvasCallbackStruct{}, nil
	})

	node3 := compose.InvokableLambda(func(ctx context.Context, input *canvasCallbackStruct) (canvasCallbackInterface, error) {
		return &canvasCallbackStruct{}, nil
	})

	node31 := compose.InvokableLambda(func(ctx context.Context, input canvasCallbackInterface) (string, error) {
		return "", nil
	})

	// Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error)
	mockRetriever := &mockRetrieve{}

	node4 := compose.InvokableLambda(func(ctx context.Context, input []*schema.Document) (map[string]any, error) {
		return map[string]any{}, nil
	})

	bNode5 := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})

	bNode6 := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})

	branch1 := compose.NewGraphBranch[map[string]any](func(ctx context.Context, in map[string]any) (endNode string, err error) {
		return "bNode5", nil
	}, map[string]bool{"bNode5": true, "bNode6": true})

	branch2 := compose.NewGraphBranch[map[string]any](func(ctx context.Context, in map[string]any) (endNode string, err error) {
		return "bNode51", nil
	}, map[string]bool{"bNode51": true, "bNode61": true})

	node7 := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (string, error) {
		return "", nil
	})

	err = g.AddLambdaNode("node1", node1)
	if err != nil {
		return
	}
	err = g.AddLambdaNode("node2", node2, compose.WithInputKey("n1"))
	if err != nil {
		return
	}
	err = g.AddLambdaNode("node3", node3)
	if err != nil {
		return
	}
	err = g.AddLambdaNode("node31", node31)
	if err != nil {

		return
	}

	err = g.AddRetrieverNode("retriever", mockRetriever)
	if err != nil {

		return
	}
	err = g.AddLambdaNode("node4", node4)
	if err != nil {

		return
	}
	err = g.AddLambdaNode("bNode5", bNode5)
	if err != nil {

		return
	}
	err = g.AddLambdaNode("bNode6", bNode6)
	if err != nil {

		return
	}

	err = g.AddLambdaNode("bNode51", bNode5)
	if err != nil {

		return
	}
	err = g.AddLambdaNode("bNode61", bNode6)
	if err != nil {

		return
	}

	err = g.AddBranch("node4", branch1)
	if err != nil {
		return
	}
	err = g.AddBranch("node4", branch2)
	if err != nil {
		return
	}

	err = g.AddLambdaNode("node7", node7)
	if err != nil {

		return
	}

	err = g.AddEdge(compose.START, "node1")
	if err != nil {

		return
	}
	err = g.AddEdge("node1", "node2")
	if err != nil {

		return
	}
	err = g.AddEdge("node2", "node3")
	if err != nil {

		return
	}
	err = g.AddEdge("node3", "node31")
	if err != nil {

		return
	}

	err = g.AddEdge("node31", "retriever")
	if err != nil {

		return
	}
	err = g.AddEdge("retriever", "node4")
	if err != nil {

		return
	}

	err = g.AddEdge("bNode5", "node7")
	if err != nil {

		return
	}
	err = g.AddEdge("bNode6", "node7")
	if err != nil {

		return
	}

	err = g.AddEdge("node7", compose.END)
	if err != nil {

		return
	}

	return
}

type canvasCallBackInferStartNode struct {
	t *testing.T
}

func (c *canvasCallBackInferStartNode) OnFinish(ctx context.Context, info *compose.GraphInfo) {
	t := c.t
	g := GraphInfo{
		GraphInfo: info,
	}
	graphSchema, err := g.BuildGraphSchema()
	assert.NoError(t, err)
	for _, edge := range graphSchema.Edges {
		names := strings.Split(edge.Name, "_to_")
		assert.Equal(t, names[0], edge.SourceNodeKey)
		assert.Equal(t, names[1], edge.TargetNodeKey)
	}
	for _, node := range graphSchema.Nodes {
		if node.Type == devmodel.NodeTypeOfStart {
			assert.NotNil(t, node.InferInput)
			for k, n := range node.InferInput.Properties {
				assert.Contains(t, []string{"n1", "n2", "n3", "subGGG"}, k)
				if k == "subGGG" {
					for sk := range n.Properties {
						assert.Contains(t, []string{"subN1", "subN2"}, sk)

					}
				}
			}
		}
	}

	return
}

type SubN2 struct {
	Name string `json:"name " binding:"required"`
}

func TestGraphInfo_inferStartNodeImplMeta(t *testing.T) {
	g := compose.NewGraph[map[string]any, string]()

	n1 := compose.InvokableLambda[string, string](func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	})
	var err error
	err = g.AddLambdaNode("n1", n1, compose.WithInputKey("n1"))
	assert.Nil(t, err)
	n2 := compose.InvokableLambda[string, string](func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	})
	err = g.AddLambdaNode("n2", n2, compose.WithInputKey("n2"))
	assert.Nil(t, err)
	n3 := compose.InvokableLambda[string, string](func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	})
	err = g.AddLambdaNode("n3", n3, compose.WithInputKey("n3"))
	assert.Nil(t, err)

	subG := compose.NewGraph[map[string]any, string]()
	subN1 := compose.InvokableLambda[string, string](func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	})
	err = subG.AddLambdaNode("subN1", subN1, compose.WithInputKey("subN1"))
	assert.Nil(t, err)
	subN2 := compose.InvokableLambda[SubN2, string](func(ctx context.Context, input SubN2) (output string, err error) {
		return input.Name, nil
	})
	err = subG.AddLambdaNode("subN2", subN2, compose.WithInputKey("subN2"))
	assert.Nil(t, err)

	err = subG.AddEdge(compose.START, "subN1")
	assert.Nil(t, err)
	err = subG.AddEdge(compose.START, "subN2")
	assert.Nil(t, err)
	err = subG.AddEdge("subN1", compose.END)
	assert.Nil(t, err)
	err = subG.AddEdge("subN2", compose.END)
	assert.Nil(t, err)

	err = g.AddGraphNode("subG", subG, compose.WithInputKey("subGGG"))
	assert.Nil(t, err)

	err = g.AddEdge(compose.START, "n1")
	assert.Nil(t, err)
	err = g.AddEdge(compose.START, "n2")
	assert.Nil(t, err)
	err = g.AddEdge(compose.START, "n3")
	assert.Nil(t, err)
	err = g.AddEdge(compose.START, "subG")
	assert.Nil(t, err)
	err = g.AddEdge("n1", compose.END)
	assert.Nil(t, err)
	err = g.AddEdge("n2", compose.END)
	assert.Nil(t, err)
	err = g.AddEdge("n3", compose.END)
	assert.Nil(t, err)
	err = g.AddEdge("subG", compose.END)
	assert.Nil(t, err)
	_, err = g.Compile(context.Background(), compose.WithGraphCompileCallbacks(&canvasCallBackInferStartNode{t: t}))
	assert.Nil(t, err)

}

func Test_unmarshalJsonWithReflectType(t *testing.T) {
	t.Run("Test unmarshal string input", func(t *testing.T) {
		userInputStr := "hello"
		userInputJson, _ := json.Marshal(userInputStr)
		userInput := string(userInputJson)

		actual, err := unmarshalJsonWithReflectType(userInput, generic.TypeOf[string]())

		assert.NoError(t, err)
		assert.Equal(t, actual.Kind(), reflect.String)
		assert.Equal(t, actual.String(), userInputStr)
	})
}

func Test_unmarshalJsonWithGraphInferType(t *testing.T) {
	t.Run("Test map input type with unmarshalGraphInferType", func(t *testing.T) {
		git := GraphInferType{
			InputTypes:                map[string]reflect.Type{"1": reflect.TypeOf("")},
			ComplicatedGraphInferType: map[string]GraphInferType{"key": {}},
		}
		actual, err := unmarshalJsonWithGraphInferType("{\"1\":\"aa\"}", git)

		assert.NoError(t, err)
		assert.Equal(t, actual.Kind(), reflect.Map)
		assert.Equal(t, actual.MapIndex(actual.MapKeys()[0]).Elem().String(), "aa")
	})
}

type DemoV2 struct {
	Name string  `json:"name"`
	D    *DemoV2 `json:"d"`
}

type DemoV3 struct {
	Name string  `json:"name"`
	D    *DemoV2 `json:"d"`
}

type DemoV1 struct {
	Name   string             `json:"name " binding:"required"`
	Child2 []*DemoV1          `json:"child2" binding:"required"`
	Child3 map[string]*DemoV1 `json:"child3" binding:"required"`
	Child  *DemoV1            `json:"child" binding:"required"`

	Child4 *DemoV2 `json:"child4" binding:"required"`

	Child5 *DemoV2 `json:"child5" binding:"required"`

	Child6 *DemoV2 `json:"child6" binding:"required"`
	Child7 *DemoV3 `json:"child7" binding:"required"`
}

func Test_parseReflectTypeToTypeSchema(t *testing.T) {
	data := parseReflectTypeToJsonSchema(reflect.TypeOf(&DemoV1{}))

	assert.Len(t, data.Properties, 8)
	assert.Equal(t, data.Properties["child"].Type, devmodel.TypeOfObject)
	assert.Equal(t, data.Properties["child2"].Type, devmodel.TypeOfArray)
	assert.Equal(t, data.Properties["child4"].Title, "model.DemoV2")
	assert.Equal(t, data.Properties["child5"].Title, "model.DemoV2")

}
