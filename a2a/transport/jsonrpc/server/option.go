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

package server

import (
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/streaming"
)

type Options struct {
	transBuilder transport.ServerTransportBuilder
	ppHdls       map[string]core.HandleEndpoint
	ssHdls       map[string]streaming.ServerStreamingHandleEndpoint
	notifHdls    map[string]core.NotificationHandleEndpoint
}

func buildOptions(opts ...Option) *Options {
	options := new(Options)
	for _, opt := range opts {
		opt(options)
	}
	return options
}

type Option func(options *Options)

func WithTransportBuilder(transBuilder transport.ServerTransportBuilder) Option {
	return func(options *Options) {
		options.transBuilder = transBuilder
	}
}

func WithPingPongHandler(method string, hdl core.HandleEndpoint) Option {
	return func(options *Options) {
		if options.ppHdls == nil {
			options.ppHdls = make(map[string]core.HandleEndpoint)
		}
		options.ppHdls[method] = hdl
	}
}

func WithServerStreamingHandler(method string, hdl streaming.ServerStreamingHandleEndpoint) Option {
	return func(options *Options) {
		if options.ssHdls == nil {
			options.ssHdls = make(map[string]streaming.ServerStreamingHandleEndpoint)
		}
		options.ssHdls[method] = hdl
	}
}

func WithNotificationHandler(method string, hdl core.NotificationHandleEndpoint) Option {
	return func(options *Options) {
		if options.notifHdls == nil {
			options.notifHdls = make(map[string]core.NotificationHandleEndpoint)
		}
		options.notifHdls[method] = hdl
	}
}
