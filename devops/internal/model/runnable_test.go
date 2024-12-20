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
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino-ext/devops/internal/utils/generic"
)

type mockRunnable interface {
	Name() string
}

type mockRunnableV2 interface {
	Name() string
}

type mockRunnableImpl struct {
	NN  string `json:"nn"`
	Age int    `json:"age"`
}

func (m mockRunnableImpl) Name() string {
	return m.NN
}

type mockRunnableImplV2 struct {
	NN  string `json:"nn"`
	Age int    `json:"age"`
}

func (m mockRunnableImplV2) Name() string {
	return m.NN
}

type mockRunnableCtxKey struct{}

type mockRunnableCallback struct {
	gi       *GraphInfo
	genState func(ctx context.Context) any
}

func (tt *mockRunnableCallback) OnFinish(ctx context.Context, graphInfo *compose.GraphInfo) {
	c, ok := ctx.Value(mockRunnableCtxKey{}).(*mockRunnableCallback)
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

func Test_GraphInfo_InferGraphInputType(t *testing.T) {
	t.Run("graph=string", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[string, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input string) (map[string]string, error) {
			return map[string]string{"A": input}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input string) (map[string]string, error) {
			return map[string]string{"B": input}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input string) (map[string]string, error) {
			return map[string]string{"C": input}, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[string]())

		userInput := `hello ABC`
		userInputMarshal, _ := json.Marshal(userInput)
		input, err := it.UnmarshalJson(string(userInputMarshal))
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.NoError(t, err)
		assert.Equal(t, resp, map[string]string{
			"A": userInput,
			"B": userInput,
			"C": userInput,
		})
	})

	t.Run("graph=int", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[int, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input int) (map[string]string, error) {
			return map[string]string{"A": strconv.Itoa(input)}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input int) (map[string]string, error) {
			return map[string]string{"B": strconv.Itoa(input)}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input int) (map[string]string, error) {
			return map[string]string{"C": strconv.Itoa(input)}, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[int]())

		userInput := `1`
		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.NoError(t, err)
		assert.Equal(t, resp, map[string]string{
			"A": userInput,
			"B": userInput,
			"C": userInput,
		})
	})

	t.Run("graph=struct", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[mockRunnableImpl, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"A": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"B": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"C": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[mockRunnableImpl]())

		userInput := `{"nn": "hello ABC"}`
		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A": "hello ABC",
			"B": "hello ABC",
			"C": "hello ABC",
		})
	})

	t.Run("graph=***struct", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[***mockRunnableImpl, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input ***mockRunnableImpl) (map[string]string, error) {
			i := ***input
			return map[string]string{"A": i.NN}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input ***mockRunnableImpl) (map[string]string, error) {
			i := ***input
			return map[string]string{"B": i.NN}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input ***mockRunnableImpl) (map[string]string, error) {
			i := ***input
			return map[string]string{"C": i.NN}, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[***mockRunnableImpl]())

		userInput := `{"nn": "hello ABC"}`
		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A": "hello ABC",
			"B": "hello ABC",
			"C": "hello ABC",
		})
	})

	t.Run("graph=struct, start nodes=(interface1, interface2)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[mockRunnableImpl, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnable) (map[string]string, error) {
			return map[string]string{"A": input.Name()}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnable) (map[string]string, error) {
			return map[string]string{"B": input.Name()}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input mockRunnableV2) (map[string]string, error) {
			return map[string]string{"C": input.Name()}, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[mockRunnableImpl]())

		userInput := `{"nn": "hello ABC"}`
		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A": "hello ABC",
			"B": "hello ABC",
			"C": "hello ABC",
		})
	})

	t.Run("graph=struct, start nodes=(interface)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[mockRunnableImpl, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnable) (map[string]string, error) {
			return map[string]string{"A": input.Name()}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnable) (map[string]string, error) {
			return map[string]string{"B": input.Name()}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input mockRunnable) (map[string]string, error) {
			return map[string]string{"C": input.Name()}, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[mockRunnableImpl]())

		userInput := `{"nn": "hello ABC"}`
		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A": "hello ABC",
			"B": "hello ABC",
			"C": "hello ABC",
		})
	})

	t.Run("graph=interface, start nodes=(struct)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[mockRunnable, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"A": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"B": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"C": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[mockRunnableImpl]())

		userInput := `{"nn": "hello ABC"}`
		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A": "hello ABC",
			"B": "hello ABC",
			"C": "hello ABC",
		})
	})

	t.Run("graph=interface, start nodes=(struct1, struct2)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[mockRunnable, string]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[mockRunnable]())
	})

	t.Run("graph=interface, start nodes=(struct, interface)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[mockRunnable, string]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		assert.NoError(t, err)
		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input mockRunnable) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[mockRunnable]())
	})

	t.Run("graph=map[string]any, start nodes=(map[string]any)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[map[string]any, string]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input map[string]any) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input map[string]any) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input map[string]any) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[map[string]any]())
	})

	t.Run("graph=map[string]any, start nodes=(interface), withInputKey", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[map[string]any, string]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnable) (string, error) {
			return "", nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnable) (string, error) {
			return "", nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input mockRunnable) (string, error) {
			return "", nil
		}), compose.WithInputKey("C"))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[map[string]any]())
	})

	t.Run("graph=map[string]any, start nodes=(***struct), withInputKey", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[map[string]any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input ***mockRunnableImpl) (map[string]string, error) {
			i := ***input
			return map[string]string{"A": i.NN}, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input ***mockRunnableImpl) (map[string]string, error) {
			i := ***input
			return map[string]string{"B": i.NN}, nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input ***mockRunnableImpl) (map[string]string, error) {
			i := ***input
			return map[string]string{"C": i.NN}, nil
		}), compose.WithInputKey("C"))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputTypes, map[string]reflect.Type{
			"A": generic.TypeOf[***mockRunnableImpl](),
			"B": generic.TypeOf[***mockRunnableImpl](),
			"C": generic.TypeOf[***mockRunnableImpl](),
		})

		userInput := `
{
    "A": {
        "nn": "hello A"
    },
    "B": {
        "nn": "hello B"
    },
    "C": {
        "nn": "hello C"
    }
}
`
		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A": "hello A",
			"B": "hello B",
			"C": "hello C",
		})
	})

	t.Run("graph=map[string]any, start nodes=([]***struct), withInputKey", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[map[string]any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input []***mockRunnableImpl) (map[string]string, error) {
			i := ***input[0]
			return map[string]string{"A": i.NN}, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input []***mockRunnableImpl) (map[string]string, error) {
			i := ***input[0]
			return map[string]string{"B": i.NN}, nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input []***mockRunnableImpl) (map[string]string, error) {
			i := ***input[0]
			return map[string]string{"C": i.NN}, nil
		}), compose.WithInputKey("C"))
		assert.NoError(t, err)

		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputTypes, map[string]reflect.Type{
			"A": generic.TypeOf[[]***mockRunnableImpl](),
			"B": generic.TypeOf[[]***mockRunnableImpl](),
			"C": generic.TypeOf[[]***mockRunnableImpl](),
		})

		userInput := `
{
    "A": [
        {
            "nn": "hello A"
        }
    ],
    "B": [
        {
            "nn": "hello B"
        }
    ],
    "C": [
        {
            "nn": "hello C"
        }
    ]
}
`
		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A": "hello A",
			"B": "hello B",
			"C": "hello C",
		})
	})

	t.Run("graph=map[string]any, start nodes=([]***struct1, []***struct2), withInputKey", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[map[string]any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input []***mockRunnableImpl) (map[string]string, error) {
			i := ***input[0]
			return map[string]string{"A": i.NN}, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input []***mockRunnableImpl) (map[string]string, error) {
			i := ***input[0]
			return map[string]string{"B": i.NN}, nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("C", compose.InvokableLambda(func(ctx context.Context, input []***mockRunnableImplV2) (map[string]string, error) {
			i := ***input[0]
			return map[string]string{"C": i.NN}, nil
		}), compose.WithInputKey("C"))
		assert.NoError(t, err)

		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputTypes, map[string]reflect.Type{
			"A": generic.TypeOf[[]***mockRunnableImpl](),
			"B": generic.TypeOf[[]***mockRunnableImpl](),
			"C": generic.TypeOf[[]***mockRunnableImplV2](),
		})

		userInput := `
{
    "A": [
        {
            "nn": "hello A"
        }
    ],
    "B": [
        {
            "nn": "hello B"
        }
    ],
    "C": [
        {
            "nn": "hello C"
        }
    ]
}
`
		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A": "hello A",
			"B": "hello B",
			"C": "hello C",
		})
	})

	t.Run("graph=any, start nodes=(string, subgraph(graph=any, start nodes=(string)))", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input string) (map[string]string, error) {
			return map[string]string{"A": input}, nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input string) (map[string]string, error) {
			return map[string]string{"B": input}, nil
		}))
		assert.NoError(t, err)

		sg := compose.NewGraph[any, any]()
		err = sg.AddLambdaNode("sub_A", compose.InvokableLambda(func(ctx context.Context, input string) (map[string]string, error) {
			return map[string]string{"sub_A": input}, nil
		}))
		assert.NoError(t, err)

		err = sg.AddLambdaNode("sub_B", compose.InvokableLambda(func(ctx context.Context, input string) (map[string]string, error) {
			return map[string]string{"sub_B": input}, nil
		}))
		assert.NoError(t, err)

		err = sg.AddEdge(compose.START, "sub_A")
		assert.NoError(t, err)
		err = sg.AddEdge(compose.START, "sub_B")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_A", compose.END)
		assert.NoError(t, err)
		err = sg.AddEdge("sub_B", compose.END)
		assert.NoError(t, err)

		err = g.AddGraphNode("C", sg)
		assert.NoError(t, err)

		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[string]())
		_, ok = it.ComplicatedGraphInferType["C"]
		assert.False(t, ok)

		userInput := `hello world`

		userInputMarshal, _ := json.Marshal(userInput)
		input, err := it.UnmarshalJson(string(userInputMarshal))
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A":     userInput,
			"B":     userInput,
			"sub_A": userInput,
			"sub_B": userInput,
		})
	})

	t.Run("graph=any, start nodes=(struct1, struct2, subgraph(graph=any, start nodes=(struct1, struct2), withInputKey), withInputKey)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"A": input.NN}, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (map[string]string, error) {
			return map[string]string{"B": input.NN}, nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		sg := compose.NewGraph[any, any]()
		err = sg.AddLambdaNode("sub_A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"sub_A": input.NN}, nil
		}), compose.WithInputKey("sub_A"))
		assert.NoError(t, err)

		err = sg.AddLambdaNode("sub_B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (map[string]string, error) {
			return map[string]string{"sub_B": input.NN}, nil
		}), compose.WithInputKey("sub_B"))
		assert.NoError(t, err)

		err = sg.AddEdge(compose.START, "sub_A")
		assert.NoError(t, err)
		err = sg.AddEdge(compose.START, "sub_B")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_A", compose.END)
		assert.NoError(t, err)
		err = sg.AddEdge("sub_B", compose.END)
		assert.NoError(t, err)

		err = g.AddGraphNode("C", sg)
		assert.NoError(t, err)

		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputTypes, map[string]reflect.Type{
			"A":     generic.TypeOf[mockRunnableImpl](),
			"B":     generic.TypeOf[mockRunnableImplV2](),
			"sub_A": generic.TypeOf[mockRunnableImpl](),
			"sub_B": generic.TypeOf[mockRunnableImplV2](),
		})
		_, ok = it.ComplicatedGraphInferType["C"]
		assert.False(t, ok)

		userInput := `
{
    "A": {
        "nn": "A"
    },
    "B": {
        "nn": "B"
    },
	 "sub_A": {
		"nn": "sub_A"
	},
	"sub_B": {
		"nn": "sub_B"
	}
}
		`

		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A":     "A",
			"B":     "B",
			"sub_A": "sub_A",
			"sub_B": "sub_B",
		})
	})

	t.Run("graph=any, start nodes=(struct1, struct2, subgraph(graph=any, start nodes=(struct1, struct2), withInputKey), withInputKey)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"A": input.NN}, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (map[string]string, error) {
			return map[string]string{"B": input.NN}, nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		sg := compose.NewGraph[any, any]()
		err = sg.AddLambdaNode("sub_A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"sub_A": input.NN}, nil
		}), compose.WithInputKey("sub_A"))
		assert.NoError(t, err)

		err = sg.AddLambdaNode("sub_B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (map[string]string, error) {
			return map[string]string{"sub_B": input.NN}, nil
		}), compose.WithInputKey("sub_B"))
		assert.NoError(t, err)

		err = sg.AddEdge(compose.START, "sub_A")
		assert.NoError(t, err)
		err = sg.AddEdge(compose.START, "sub_B")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_A", compose.END)
		assert.NoError(t, err)
		err = sg.AddEdge("sub_B", compose.END)
		assert.NoError(t, err)

		err = g.AddGraphNode("C", sg, compose.WithInputKey("C"))
		assert.NoError(t, err)

		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputTypes, map[string]reflect.Type{
			"A": generic.TypeOf[mockRunnableImpl](),
			"B": generic.TypeOf[mockRunnableImplV2](),
			"C": generic.TypeOf[map[string]any](),
		})
		assert.Equal(t, it.ComplicatedGraphInferType["C"].InputTypes, map[string]reflect.Type{
			"sub_A": generic.TypeOf[mockRunnableImpl](),
			"sub_B": generic.TypeOf[mockRunnableImplV2](),
		})

		userInput := `
{
    "A": {
        "nn": "A"
    },
    "B": {
        "nn": "B"
    },
    "C": {
		 "sub_A": {
            "nn": "sub_A"
        },
        "sub_B": {
            "nn": "sub_B"
        }
    }
}
		`

		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A":     "A",
			"B":     "B",
			"sub_A": "sub_A",
			"sub_B": "sub_B",
		})
	})

	t.Run("graph=any, start nodes=(struct1, struct2, subgraph(graph=any, start nodes=(struct1, struct2)), withInputKey)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[any, string]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (string, error) {
			return "", nil
		}))
		assert.NoError(t, err)

		sg := compose.NewGraph[any, string]()
		err = sg.AddLambdaNode("sub_A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (string, error) {
			return "", nil
		}), compose.WithInputKey("sub_A"))
		assert.NoError(t, err)

		err = sg.AddLambdaNode("sub_B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (string, error) {
			return "", nil
		}), compose.WithInputKey("sub_B"))
		assert.NoError(t, err)

		err = sg.AddEdge(compose.START, "sub_A")
		assert.NoError(t, err)
		err = sg.AddEdge(compose.START, "sub_B")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_A", compose.END)
		assert.NoError(t, err)
		err = sg.AddEdge("sub_B", compose.END)
		assert.NoError(t, err)

		err = g.AddGraphNode("C", sg)
		assert.NoError(t, err)

		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, it.InputType, generic.TypeOf[any]())
	})

	t.Run("graph=any, start nodes=(struct1, struct2, subgraph(graph=any, start nodes=(struct1, struct2)), withInputKey)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"A": input.NN}, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (map[string]string, error) {
			return map[string]string{"B": input.NN}, nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		sg := compose.NewGraph[any, any]()
		err = sg.AddLambdaNode("sub_A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"sub_A": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = sg.AddLambdaNode("sub_B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"sub_B": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = sg.AddEdge(compose.START, "sub_A")
		assert.NoError(t, err)
		err = sg.AddEdge(compose.START, "sub_B")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_A", compose.END)
		assert.NoError(t, err)
		err = sg.AddEdge("sub_B", compose.END)
		assert.NoError(t, err)

		err = g.AddGraphNode("C", sg, compose.WithInputKey("C"))
		assert.NoError(t, err)

		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph(compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType(compose.START)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputTypes, map[string]reflect.Type{
			"A": generic.TypeOf[mockRunnableImpl](),
			"B": generic.TypeOf[mockRunnableImplV2](),
			"C": generic.TypeOf[mockRunnableImpl](),
		})
		_, ok = it.ComplicatedGraphInferType["C"]
		assert.False(t, ok)

		userInput := `
{
    "A": {
        "nn": "A"
    },
    "B": {
        "nn": "B"
    },
    "C": {
		"nn": "sub_AB"
    }
}
		`

		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A":     "A",
			"B":     "B",
			"sub_A": "sub_AB",
			"sub_B": "sub_AB",
		})
	})

	t.Run("start from subgraph, graph=any, start nodes=(struct1, struct2, subgraph(graph=any, start nodes=(struct1, struct2)), withInputKey)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"A": input.NN}, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (map[string]string, error) {
			return map[string]string{"B": input.NN}, nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		sg := compose.NewGraph[any, any]()
		err = sg.AddLambdaNode("sub_A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"sub_A": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = sg.AddLambdaNode("sub_B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"sub_B": input.NN}, nil
		}))
		assert.NoError(t, err)

		err = sg.AddEdge(compose.START, "sub_A")
		assert.NoError(t, err)
		err = sg.AddEdge(compose.START, "sub_B")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_A", compose.END)
		assert.NoError(t, err)
		err = sg.AddEdge("sub_B", compose.END)
		assert.NoError(t, err)

		err = g.AddGraphNode("C", sg, compose.WithInputKey("C"))
		assert.NoError(t, err)

		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph("C")
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType("C")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputTypes, map[string]reflect.Type{
			"C": generic.TypeOf[mockRunnableImpl](),
		})
		_, ok = it.ComplicatedGraphInferType["C"]
		assert.False(t, ok)

		userInput := `
{
    "C": {
		"nn": "sub_AB"
    }
}
		`

		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"sub_A": "sub_AB",
			"sub_B": "sub_AB",
		})
	})

	t.Run("start from subgraph, graph=any, start nodes=(struct1, struct2, subgraph(graph=any, start nodes=(struct1, struct2), withInputKey), withInputKey)", func(t *testing.T) {
		tc := &mockRunnableCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, mockRunnableCtxKey{}, tc)
		g := compose.NewGraph[any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"A": input.NN}, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (map[string]string, error) {
			return map[string]string{"B": input.NN}, nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		sg := compose.NewGraph[any, any]()
		err = sg.AddLambdaNode("sub_A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"sub_A": input.NN}, nil
		}), compose.WithInputKey("sub_A"))
		assert.NoError(t, err)

		err = sg.AddLambdaNode("sub_B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (map[string]string, error) {
			return map[string]string{"sub_B": input.NN}, nil
		}), compose.WithInputKey("sub_B"))
		assert.NoError(t, err)

		err = sg.AddEdge(compose.START, "sub_A")
		assert.NoError(t, err)
		err = sg.AddEdge(compose.START, "sub_B")
		assert.NoError(t, err)
		err = sg.AddEdge("sub_A", compose.END)
		assert.NoError(t, err)
		err = sg.AddEdge("sub_B", compose.END)
		assert.NoError(t, err)

		err = g.AddGraphNode("C", sg, compose.WithInputKey("C"))
		assert.NoError(t, err)

		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "A")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "B")
		assert.NoError(t, err)
		err = g.AddEdge(compose.START, "C")
		assert.NoError(t, err)
		err = g.AddEdge("A", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("B", compose.END)
		assert.NoError(t, err)
		err = g.AddEdge("C", compose.END)
		assert.NoError(t, err)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.NoError(t, err)

		dg, err := tc.gi.BuildDevGraph("C")
		assert.NoError(t, err)
		r, err := dg.Compile(tc.gi.CompileOptions...)
		assert.NoError(t, err)

		it, ok, err := tc.gi.InferGraphInputType("C")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, it.InputTypes, map[string]reflect.Type{
			"C": generic.TypeOf[map[string]any](),
		})
		assert.Equal(t, it.ComplicatedGraphInferType["C"].InputTypes, map[string]reflect.Type{
			"sub_A": generic.TypeOf[mockRunnableImpl](),
			"sub_B": generic.TypeOf[mockRunnableImplV2](),
		})

		userInput := `
{
    "C": {
		 "sub_A": {
            "nn": "sub_A"
        },
        "sub_B": {
            "nn": "sub_B"
        }
    }
}
		`

		input, err := it.UnmarshalJson(userInput)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"sub_A": "sub_A",
			"sub_B": "sub_B",
		})
	})
}
