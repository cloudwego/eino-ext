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
	"testing"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/cloudwego/eino/compose"

	"github.com/cloudwego/eino-ext/devops/internal/mock"
	"github.com/cloudwego/eino-ext/devops/internal/model"
)

type callbackTestSuite struct {
	suite.Suite
	ctrl *gomock.Controller
	ctx  context.Context

	mockContainer *mock.MockContainerService
}

func Test_callbackTestSuite(t *testing.T) {
	suite.Run(t, new(callbackTestSuite))
}

func (c *callbackTestSuite) SetupSuite() {
	c.ctrl = gomock.NewController(c.T())

	c.mockContainer = mock.NewMockContainerService(c.ctrl)
	ContainerSVC = c.mockContainer
}

func (c *callbackTestSuite) SetupTest() {
	c.ctx = context.Background()
}

func (c *callbackTestSuite) buildCallbackGraph() {
	g := compose.NewGraph[string, string]()
	_ = g.AddLambdaNode("node", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	}))
	_ = g.AddEdge(compose.START, "node")
	_ = g.AddEdge("node", compose.END)
	_, err := g.Compile(context.Background(), compose.WithGraphCompileCallbacks(NewGlobalDevGraphCompileCallback()))
	assert.NoError(c.T(), err)
}

func (c *callbackTestSuite) buildCallbackChain() {
	cn := compose.NewChain[string, string]()
	_ = cn.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	}))
	_, err := cn.Compile(context.Background(), compose.WithGraphCompileCallbacks(NewGlobalDevGraphCompileCallback()))
	assert.NoError(c.T(), err)
}

func (c *callbackTestSuite) Test_NewGlobalDevGraphCompileCallback() {
	mockey.PatchConvey("add graph with no graph name", c.T(), func() {
		c.mockContainer.EXPECT().AddGraphInfo(gomock.Any(), gomock.Any()).DoAndReturn(
			func(graphName string, graphInfo *compose.GraphInfo) (graphID string, err error) {
				assert.Equal(c.T(), "callback_test.buildCallbackGraph:64", graphName)
				return "", nil
			}).Times(1)

		c.buildCallbackGraph()
	})

	mockey.PatchConvey("add chain with no chian name", c.T(), func() {
		c.mockContainer.EXPECT().AddGraphInfo(gomock.Any(), gomock.Any()).DoAndReturn(
			func(graphName string, graphInfo *compose.GraphInfo) (graphID string, err error) {
				assert.Equal(c.T(), "callback_test.buildCallbackChain:73", graphName)
				return "", nil
			}).Times(1)

		c.buildCallbackChain()
	})

	mockey.PatchConvey("skip eino devops compile graph", c.T(), func() {
		var gi model.GraphInfo
		c.mockContainer.EXPECT().AddGraphInfo(gomock.Any(), gomock.Any()).DoAndReturn(
			func(graphName string, graphInfo *compose.GraphInfo) (graphID string, err error) {
				gi = model.GraphInfo{
					GraphInfo: graphInfo,
				}

				assert.Equal(c.T(), "callback_test.buildCallbackGraph:64", graphName)
				return "", nil
			}).Times(1)
		c.buildCallbackGraph()

		mockID := "mock_graph_id"
		svcImpl := containerServiceImpl{
			container: map[string]*model.GraphContainer{mockID: {GraphInfo: &gi}},
		}
		_, err := svcImpl.CreateDevGraph(mockID, compose.START)
		assert.NoError(c.T(), err)
	})
}
