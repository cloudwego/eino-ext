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
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino-ext/devops/internal/model"
)

func Test_containerServiceImpl_AddGraphInfo(t *testing.T) {
	t.Run("add graph info", func(t *testing.T) {
		mockGraphName := "mock_graph"
		s := &containerServiceImpl{}
		graphID, err := s.AddCustomGraphInfo(mockGraphName, &compose.GraphInfo{}, model.GraphOption{})
		assert.Nil(t, err)
		g, ok := s.container[graphID]
		assert.True(t, ok)
		assert.NotNil(t, g)
	})

	t.Run("add graph info", func(t *testing.T) {
		mockGraphName := "mock_graph"
		s := &containerServiceImpl{}
		graphID, err := s.AddCustomGraphInfo(mockGraphName, &compose.GraphInfo{}, model.GraphOption{})
		assert.Nil(t, err)
		g, ok := s.container[graphID]
		assert.True(t, ok)
		assert.NotNil(t, g)
	})
}

func Test_containerServiceImpl_CreateRunnable(t *testing.T) {
	t.Run("graphInfo not exist", func(t *testing.T) {
		s := &containerServiceImpl{}
		_, err := s.CreateRunnable("test_graph", compose.START)
		assert.NotNil(t, err)
	})

	t.Run("graphInfo exist", func(t *testing.T) {
		g := compose.NewGraph[int, []string]()
		err := g.AddLambdaNode("node_1", compose.InvokableLambda(func(ctx context.Context, input int) (output []string, err error) {
			return []string{strconv.Itoa(input), fmt.Sprintf("out_lambda_1")}, nil
		}))
		assert.Nil(t, err)
		err = g.AddLambdaNode("node_2", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_2"))
			return output, nil
		}))
		assert.Nil(t, err)
		err = g.AddLambdaNode("node_3", compose.InvokableLambda(func(ctx context.Context, input []string) (output []string, err error) {
			output = append(input, fmt.Sprintf("out_lambda_3"))
			return output, nil
		}))
		assert.Nil(t, err)

		err = g.AddEdge(compose.START, "node_1")
		assert.Nil(t, err)
		err = g.AddEdge("node_1", "node_2")
		assert.Nil(t, err)
		err = g.AddEdge("node_2", "node_3")
		assert.Nil(t, err)
		err = g.AddEdge("node_3", compose.END)
		assert.Nil(t, err)

		tc := &testCallback{}
		ctx := context.Background()
		ctx = context.WithValue(ctx, testCtxKey{}, tc)

		_, err = g.Compile(ctx, compose.WithGraphCompileCallbacks(tc))
		assert.Nil(t, err)

		mockey.PatchConvey("from start node", t, func() {
			mockey.Mock((*model.GraphInfo).BuildDevGraph).Return(g, nil).Build()
			mockey.Mock((*model.Graph).Compile).Return(model.Runnable{}, nil).Build()

			mockGraphID := "mock_graph"
			s := &containerServiceImpl{
				container: map[string]*model.GraphContainer{
					mockGraphID: {
						GraphInfo: tc.gi,
					},
				},
			}
			_, err = s.CreateRunnable(mockGraphID, compose.START)
			assert.Nil(t, err)

			_, exist := s.container[mockGraphID].NodesRunnable[compose.START]
			assert.True(t, exist)
		})

		mockey.PatchConvey("from node_2", t, func() {
			mockey.Mock((*model.GraphInfo).BuildDevGraph).Return(g, nil).Build()
			mockey.Mock((*model.Graph).Compile).Return(model.Runnable{}, nil).Build()

			mockGraphID := "mock_graph"
			s := &containerServiceImpl{
				container: map[string]*model.GraphContainer{
					mockGraphID: {
						GraphInfo: tc.gi,
					},
				},
			}
			_, err = s.CreateRunnable(mockGraphID, "node_2")
			assert.Nil(t, err)

			_, exist := s.container[mockGraphID].NodesRunnable["node_2"]
			assert.True(t, exist)
		})
	})
}

func Test_containerServiceImpl_GetRunnable(t *testing.T) {
	t.Run("graph not exist", func(t *testing.T) {
		mockGraphID := "mock_graph"
		mockNode := "mock_node"

		s := &containerServiceImpl{}
		_, exist := s.GetRunnable(mockGraphID, mockNode)
		assert.False(t, exist)
	})

	t.Run("node not exist", func(t *testing.T) {
		mockGraphID := "mock_graph"
		mockNode := "mock_node"

		s := &containerServiceImpl{
			container: map[string]*model.GraphContainer{
				mockGraphID: {},
			},
		}
		_, exist := s.GetRunnable(mockGraphID, mockNode)
		assert.False(t, exist)
	})

	t.Run("node exist", func(t *testing.T) {
		mockGraphID := "mock_graph"
		mockNode := "mock_node"

		s := &containerServiceImpl{
			container: map[string]*model.GraphContainer{
				mockGraphID: {
					NodesRunnable: map[string]*model.Runnable{
						mockNode: {},
					},
				},
			},
		}
		_, exist := s.GetRunnable(mockGraphID, mockNode)
		assert.True(t, exist)
	})
}
func Test_containerServiceImpl_CreateCanvas(t *testing.T) {

	t.Run("not get canvas", func(t *testing.T) {
		s := newContainerService()
		_, err := s.CreateCanvas("graph_id")
		assert.NotNil(t, err)
		c, ok := s.GetCanvas("graph_id")
		assert.False(t, ok)
		assert.NotNil(t, c)

	})

	t.Run("create canvas and get this", func(t *testing.T) {
		s := newContainerService()
		g := &compose.GraphInfo{
			InputType: reflect.TypeOf(map[string]any{}),
		}
		id, err := s.AddCustomGraphInfo("graph", g, model.GraphOption{})
		assert.Nil(t, err)
		c, err := s.CreateCanvas(id)
		assert.Nil(t, err)
		assert.Equal(t, "graph", c.Name)

		c, ok := s.GetCanvas(id)
		assert.True(t, ok)
		assert.Equal(t, "graph", c.Name)
	})

}

func Test_containerServiceImpl_ListGraphs(t *testing.T) {
	s := &containerServiceImpl{
		container: map[string]*model.GraphContainer{
			"g1": {GraphID: "1", GraphName: "g1"},
			"g2": {GraphID: "2", GraphName: "g2"},
		},
	}
	assert.True(t, reflect.DeepEqual(s.ListGraphs(), map[string]string{
		"g1": "1",
		"g2": "2",
	}))
}

type testCtxKey struct{}

type testCallback struct {
	gi *model.GraphInfo
}

func (*testCallback) OnFinish(ctx context.Context, graphInfo *compose.GraphInfo) {
	c, ok := ctx.Value(testCtxKey{}).(*testCallback)
	if !ok {
		return
	}
	c.gi = &model.GraphInfo{
		GraphInfo: graphInfo,
	}
}