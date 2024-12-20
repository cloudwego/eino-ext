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

package einodev

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino-ext/devops/internal/service"
)

func Test_newGlobalDevGraphCompileCallback(t *testing.T) {
	t.Run("graph", func(t *testing.T) {
		graphName := "mock_graph"
		cb := newGlobalDevGraphCompileCallback()
		cb.OnFinish(context.Background(), &compose.GraphInfo{
			Key: graphName,
		})
		m := service.ContainerSVC.ListGraphs()
		assert.NotEqual(t, m[graphName], "")
	})
}
