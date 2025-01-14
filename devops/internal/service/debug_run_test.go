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
	"testing"

	"github.com/cloudwego/eino-ext/devops/internal/model"
	"github.com/cloudwego/eino/compose"

	"github.com/stretchr/testify/assert"
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
