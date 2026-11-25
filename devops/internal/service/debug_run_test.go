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

	"github.com/cloudwego/eino-ext/devops/internal/model"
	devmodel "github.com/cloudwego/eino-ext/devops/model"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewDebugService(t *testing.T) {
	svc := newDebugService()
	impl, ok := svc.(*debugServiceImpl)
	assert.True(t, ok)
	assert.NotNil(t, impl.debugGraphs)
}

func Test_debugServiceImpl_getInvokeOptions(t *testing.T) {
	gi := &model.GraphInfo{
		GraphInfo: &compose.GraphInfo{
			Nodes: map[string]compose.GraphNodeInfo{
				"node1": {},
				"node2": {},
			},
		},
	}

	svc := newDebugService()
	impl, ok := svc.(*debugServiceImpl)
	assert.True(t, ok)
	opts, err := impl.getInvokeOptions(gi, "t1", nil)
	assert.Nil(t, err)
	assert.NotNil(t, opts)
}

func Test_debugServiceImpl_DebugRunTextUUID(t *testing.T) {
	type uuidInput struct {
		ID uuid.UUID `json:"customer_id"`
	}

	const id = "550e8400-e29b-41d4-a716-446655440000"
	expected := uuid.MustParse(id)
	ctx := context.Background()

	oldContainer := ContainerSVC
	ContainerSVC = newContainerService()
	t.Cleanup(func() {
		ContainerSVC = oldContainer
	})

	invoked := make(chan uuidInput, 2)
	graph := compose.NewGraph[uuidInput, uuidInput]()
	require.NoError(t, graph.AddLambdaNode("echo", compose.InvokableLambda(
		func(_ context.Context, input uuidInput) (uuidInput, error) {
			invoked <- input
			return input, nil
		},
	)))
	require.NoError(t, graph.AddEdge(compose.START, "echo"))
	require.NoError(t, graph.AddEdge("echo", compose.END))
	_, err := graph.Compile(ctx, compose.WithGraphCompileCallbacks(NewGlobalDevGraphCompileCallback()))
	require.NoError(t, err)

	graphs := ContainerSVC.ListGraphs()
	require.Len(t, graphs, 1)
	var graphID string
	for _, id := range graphs {
		graphID = id
	}

	canvas, err := ContainerSVC.CreateCanvas(graphID)
	require.NoError(t, err)
	for _, node := range canvas.Nodes {
		if node.Key != compose.START && node.Key != "echo" {
			continue
		}
		for _, schemaRoot := range []*devmodel.JsonSchema{
			node.ComponentSchema.InputType,
			node.ComponentSchema.OutputType,
		} {
			schema := schemaRoot.Properties["customer_id"]
			require.NotNil(t, schema)
			assert.Equal(t, devmodel.JsonTypeOfString, schema.Type)
		}
	}

	debug := newDebugService()
	threadID, err := debug.CreateDebugThread(ctx, graphID)
	require.NoError(t, err)

	for _, fromNode := range []string{compose.START, "echo"} {
		debugID, stateCh, errCh, err := debug.DebugRun(ctx, &model.DebugRunMeta{
			GraphID:  graphID,
			ThreadID: threadID,
			FromNode: fromNode,
		}, `{"customer_id":"`+id+`"}`)
		require.NoError(t, err)
		assert.NotEmpty(t, debugID)

		for range stateCh {
		}
		for runErr := range errCh {
			require.NoError(t, runErr)
		}

		require.Len(t, invoked, 1)
		input := <-invoked
		assert.Equal(t, expected, input.ID)
	}
}
