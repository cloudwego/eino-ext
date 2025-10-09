/*
 * Copyright 2025 CloudWeGo Authors
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

package client

import (
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport/http"
)

type Options struct {
	url string
	hdl transport.ClientTransportHandler
}

func defaultOptions() Options {
	return Options{
		hdl: http.NewClientTransportHandler(),
	}
}

type Option func(options *Options)

func WithURL(url string) Option {
	return func(options *Options) {
		options.url = url
	}
}

func WithTransportHandler(hdl transport.ClientTransportHandler) Option {
	return func(options *Options) {
		options.hdl = hdl
	}
}
