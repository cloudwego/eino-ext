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
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/devops/internal/utils/generic"
	"github.com/cloudwego/eino/compose"
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `hello ABC`
		input, err := UnmarshalJson([]byte(fmt.Sprintf(`"%s"`, userInput)), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `1`
		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `{"nn": "hello ABC"}`
		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `{"nn": "hello ABC"}`
		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `{"nn": "hello ABC"}`
		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `{"nn": "hello ABC"}`
		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `{
    "_eino_go_type": "model.mockRunnableImpl",
    "_value": {
        "nn": "hello ABC"
    }
}`
		RegisterType(generic.TypeOf[mockRunnableImpl]())
		a := registeredTypes
		_ = a
		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A": "hello ABC",
			"B": "hello ABC",
			"C": "hello ABC",
		})
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `
{
	"A": {
		"_value": {
			"nn": "hello A"
		},
		"_eino_go_type": "***model.mockRunnableImpl"
	},
	"B": {
		"_value": {
			"nn": "hello B"
		},
		"_eino_go_type": "***model.mockRunnableImpl"
	},
	"C": {
		"_value": {
			"nn": "hello C"
		},
		"_eino_go_type": "***model.mockRunnableImpl"
	}
}
`
		RegisterType(generic.TypeOf[***mockRunnableImpl]())

		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `
{
    "A": {
        "_value": [
            {
                "nn": "hello A"
            }
        ],
        "_eino_go_type": "[]***model.mockRunnableImpl"
    },
    "B": {
        "_value": [
            {
                "nn": "hello B"
            }
        ],
        "_eino_go_type": "[]***model.mockRunnableImpl"
    },
    "C": {
        "_value": [
            {
                "nn": "hello C"
            }
        ],
        "_eino_go_type": "[]***model.mockRunnableImpl"
    }
}
`
		RegisterType(generic.TypeOf[[]***mockRunnableImpl]())

		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `
{
    "A": {
        "_value": [
            {
                "nn": "hello A"
            }
        ],
        "_eino_go_type": "[]***model.mockRunnableImpl"
    },
    "B": {
        "_value": [
            {
                "nn": "hello B"
            }
        ],
        "_eino_go_type": "[]***model.mockRunnableImpl"
    },
    "C": {
        "_value": [
            {
                "nn": "hello C"
            }
        ],
        "_eino_go_type": "[]***model.mockRunnableImplV2"
    }
}
`

		RegisterType(generic.TypeOf[[]***mockRunnableImpl]())
		RegisterType(generic.TypeOf[[]***mockRunnableImplV2]())

		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `{
    "_value": "hello world",
    "_eino_go_type": "string"
}`
		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"A":     "hello world",
			"B":     "hello world",
			"sub_A": "hello world",
			"sub_B": "hello world",
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `
{
  "_eino_go_type": "map[string]interface {}",
  "_value": {
    "A": {
     "_value": {
       "nn": "A"
     },
      "_eino_go_type": "model.mockRunnableImpl"
    },
    "B": {
      "_value": {
        "nn": "B"
      },
      "_eino_go_type": "model.mockRunnableImplV2"
    },
    "sub_A": {
      "_value": {
        "nn": "sub_A"
      },
      "_eino_go_type": "model.mockRunnableImpl"
    },
    "sub_B": {
      "_value": {
        "nn": "sub_B"
      },
      "_eino_go_type": "model.mockRunnableImplV2"
    }
  }
}
		`

		RegisterType(generic.TypeOf[mockRunnableImpl]())
		RegisterType(generic.TypeOf[mockRunnableImplV2]())
		RegisterType(generic.TypeOf[map[string]any]())

		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `
{
  "_eino_go_type": "map[string]interface {}",
  "_value": {
    "A": {
      "_value": {
        "nn": "A"
      },
      "_eino_go_type": "model.mockRunnableImpl"
    },
    "B": {
      "_value": {
        "nn": "B"
      },
      "_eino_go_type": "model.mockRunnableImplV2"
    },
    "C": {
      "_value": {
        "sub_A": {
          "_value": {
            "nn": "sub_A"
          },
          "_eino_go_type": "model.mockRunnableImpl"
        },
        "sub_B": {
          "_value": {
            "nn": "sub_B"
          },
          "_eino_go_type": "model.mockRunnableImplV2"
        }
      },
	  "_eino_go_type": "map[string]interface {}"
    }
  }
}
		`

		RegisterType(generic.TypeOf[map[string]any]())
		RegisterType(generic.TypeOf[mockRunnableImpl]())
		RegisterType(generic.TypeOf[mockRunnableImplV2]())

		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, compose.START)
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `
{
  "_eino_go_type": "map[string]interface {}",
  "_value": {
    "A": {
      "_value": {
        "nn": "A"
      },
      "_eino_go_type": "model.mockRunnableImpl"
    },
    "B": {
      "_value": {
        "nn": "B"
      },
      "_eino_go_type": "model.mockRunnableImplV2"
    },
    "C": {
      "_value": {
		"nn": "sub_AB"
	  },
	  "_eino_go_type": "model.mockRunnableImpl"
    }
  }
}
		`

		RegisterType(generic.TypeOf[map[string]any]())
		RegisterType(generic.TypeOf[mockRunnableImpl]())
		RegisterType(generic.TypeOf[mockRunnableImplV2]())

		input, err := UnmarshalJson([]byte(userInput), tc.gi.InputType)
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

		dg, err := BuildDevGraph(tc.gi, "C")
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)

		userInput := `
{
  "_eino_go_type": "map[string]interface {}",
  "_value": {
    "C": {
      "_value": {
        "nn": "sub_AB"
      },
      "_eino_go_type": "model.mockRunnableImpl"
    }
  }
}
		`

		RegisterType(generic.TypeOf[map[string]any]())
		RegisterType(generic.TypeOf[mockRunnableImpl]())
		RegisterType(generic.TypeOf[mockRunnableImplV2]())

		input, err := UnmarshalJson([]byte(userInput), tc.gi.Nodes["C"].InputType)
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
		g := compose.NewGraph[map[string]any, any]()

		err := g.AddLambdaNode("A", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImpl) (map[string]string, error) {
			return map[string]string{"A": input.NN}, nil
		}), compose.WithInputKey("A"))
		assert.NoError(t, err)

		err = g.AddLambdaNode("B", compose.InvokableLambda(func(ctx context.Context, input mockRunnableImplV2) (map[string]string, error) {
			return map[string]string{"B": input.NN}, nil
		}), compose.WithInputKey("B"))
		assert.NoError(t, err)

		sg := compose.NewGraph[map[string]any, any]()
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

		dg, err := BuildDevGraph(tc.gi, "C")
		assert.NoError(t, err)
		r, err := dg.Compile()
		assert.NoError(t, err)
		assert.Equal(t, len(dg.GraphInfo.Nodes), 1)
		assert.Equal(t, len(dg.GraphInfo.Edges), 2)

		userInput := `
{
  "C": {
    "_value": {
      "sub_A": {
        "_value": {
          "nn": "sub_A"
        },
        "_eino_go_type": "model.mockRunnableImpl"
      },
      "sub_B": {
        "_value": {
          "nn": "sub_B"
        },
        "_eino_go_type": "model.mockRunnableImplV2"
      }
    },
    "_eino_go_type": "map[string]interface {}"
  }
}
		`

		RegisterType(generic.TypeOf[map[string]any]())
		RegisterType(generic.TypeOf[mockRunnableImpl]())
		RegisterType(generic.TypeOf[mockRunnableImplV2]())

		input, err := UnmarshalJson([]byte(userInput), tc.gi.Nodes["C"].InputType)
		assert.NoError(t, err)
		resp, err := r.Invoke(ctx, input)
		assert.Equal(t, resp, map[string]string{
			"sub_A": "sub_A",
			"sub_B": "sub_B",
		})
	})
}
