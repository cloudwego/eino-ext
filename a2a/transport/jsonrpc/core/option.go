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

package core

import (
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/tracer"
)

type CallOptions struct {
	relatedId ID
}

type CallOption func(options *CallOptions)

func WithCallRelatedID(id ID) CallOption {
	return func(options *CallOptions) {
		options.relatedId = id
	}
}

type Options struct {
	ppHdls    map[string]HandleEndpoint
	notifHdls map[string]NotificationHandleEndpoint
	reqHdls   map[string]RequestHandleEndpoint

	callMWs []CallMiddleware
	hdlMWs  []HandleMiddleware

	tracer   tracer.Tracer
	cliTrans ClientRounder
	srvTrans ServerRounder
}

func defaultOptions() Options {
	return Options{
		tracer: tracer.NewNoopTracer(),
	}
}

type Option func(options *Options)

type CloseCallback func(err error)

func WithClientRounder(trans ClientRounder) Option {
	return func(options *Options) {
		options.cliTrans = trans
	}
}

func WithServerRounder(trans ServerRounder) Option {
	return func(options *Options) {
		options.srvTrans = trans
	}
}

func WithHandler(method string, hdl HandleEndpoint) Option {
	return func(options *Options) {
		if options.ppHdls == nil {
			options.ppHdls = make(map[string]HandleEndpoint)
		}
		options.ppHdls[method] = hdl
	}
}

func WithNotificationHandler(method string, hdl NotificationHandleEndpoint) Option {
	return func(options *Options) {
		if options.notifHdls == nil {
			options.notifHdls = make(map[string]NotificationHandleEndpoint)
		}
		options.notifHdls[method] = hdl
	}
}

func WithRequestHandler(method string, hdl RequestHandleEndpoint) Option {
	return func(options *Options) {
		if options.reqHdls == nil {
			options.reqHdls = make(map[string]RequestHandleEndpoint)
		}
		options.reqHdls[method] = hdl
	}
}

func WithTracer(tracer tracer.Tracer) Option {
	return func(options *Options) {
		options.tracer = tracer
	}
}

func WithCallMiddleware(mw CallMiddleware) Option {
	return func(options *Options) {
		options.callMWs = append(options.callMWs, mw)
	}
}

func WithHandleMiddleware(mw HandleMiddleware) Option {
	return func(options *Options) {
		options.hdlMWs = append(options.hdlMWs, mw)
	}
}
