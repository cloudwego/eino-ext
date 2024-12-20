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

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino-ext/devops/internal/model"
)

const (
	defaultHttpPort = "52538"
)

type serverOption struct {
	port string
}

// newServerOption create ServerOption.
func newServerOption(options []ServerOption) *serverOption {
	o := &serverOption{
		port: defaultHttpPort,
	}
	for _, opt := range options {
		opt(o)
	}
	return o
}

type ServerOption func(*serverOption)

// WithDevServerPort dev server port, default to 52538
func WithDevServerPort(port string) ServerOption {
	return func(o *serverOption) {
		o.port = port
	}
}

type devOption struct {
	genLocalState     func(ctx context.Context) any
	inputUnmarshalFns []model.NodeUnmarshalInput
}

type DevOption func(*devOption)

// Deprecated: WithGenLocalState is useless, and will no longer be provided in the future.
// WithGenLocalState generate local state function for state graph, need to be same as compose.GenLocalState definition.
func WithGenLocalState[S any](genLocalState compose.GenLocalState[S]) DevOption {
	return func(o *devOption) {
		o.genLocalState = func(ctx context.Context) any {
			return genLocalState(ctx)
		}
	}
}

// WithUnmarshalInput register unmarshal method for node debug input
func WithUnmarshalInput(nodeKey string, f model.UnmarshalInput) DevOption {
	return func(o *devOption) {
		o.inputUnmarshalFns = append(o.inputUnmarshalFns, model.NodeUnmarshalInput{
			NodeKey:        nodeKey,
			UnmarshalInput: f,
		})
	}
}
