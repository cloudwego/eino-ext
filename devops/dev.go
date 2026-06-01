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

package devops

import (
	"context"
	"time"

	"github.com/cloudwego/eino-ext/devops/internal/apihandler"
	"github.com/cloudwego/eino-ext/devops/internal/model"
	"github.com/cloudwego/eino-ext/devops/internal/utils/safego"
)

// Init start eino devops server.
// By default, the server listens on 127.0.0.1:52538.
// Use WithDevServerIP and WithDevServerPort to customize the listen address.
// If the IP is unknown, you can set it to "0.0.0.0" to listen on all interfaces,
// but be aware this may expose the server to external access and pose security risks.
func Init(ctx context.Context, opts ...model.DevOption) error {
	opt := model.NewDevOpt(opts)
	apihandler.InitDebug(opt)

	errCh := make(chan error)
	safego.Go(ctx, func() {
		errCh <- apihandler.StartHTTPServer(ctx, opt.DevServerIP, opt.DevServerPort)
	})

	select {
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Second):
		return nil
	}
}
