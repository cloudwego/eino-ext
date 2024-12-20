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
	"time"

	"github.com/cloudwego/eino/compose"

	"github.com/cloudwego/eino-ext/devops/internal/apihandler"
	"github.com/cloudwego/eino-ext/devops/internal/utils/safego"
)

// Deprecated: use Init instead.
func Run(ctx context.Context, opts ...ServerOption) error {
	o := newServerOption(opts)

	errCh := make(chan error)
	safego.Go(ctx, func() {
		errCh <- apihandler.StartHTTPServer(ctx, o.port)
	})

	select {
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Second):
		return nil
	}
}

// Init start einodev.
func Init(ctx context.Context, opts ...ServerOption) error {
	compose.InitGraphCompileCallbacks([]compose.GraphCompileCallback{newGlobalDevGraphCompileCallback()})

	o := newServerOption(opts)

	errCh := make(chan error)
	safego.Go(ctx, func() {
		errCh <- apihandler.StartHTTPServer(ctx, o.port)
	})

	select {
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Second):
		return nil
	}
}
