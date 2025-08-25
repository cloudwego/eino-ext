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
	"context"
	"errors"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/conninfo"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/transport"
)

var (
	errNoURL = errors.New("no url")
)

// Client is a jsonrpc2 client that corresponding to a specific URL.
// You can create a Connection implementing jsonrpc2 semantics.
type Client struct {
	remotePeer conninfo.Peer
	hdl        transport.ClientTransportHandler
}

func NewClient(opts ...Option) (*Client, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}
	// parse url
	if options.url == "" {
		return nil, errNoURL
	}
	cli := &Client{}
	cli.remotePeer = conninfo.NewPeer(conninfo.PeerTypeURL, options.url)
	cli.hdl = options.hdl
	return cli, nil
}

func (cli *Client) NewConnection(ctx context.Context) (core.Connection, error) {
	st, err := cli.hdl.NewTransport(ctx, cli.remotePeer)
	if err != nil {
		return nil, err
	}
	var opts []core.Option
	// client calling
	if cliTrans, ok := st.ClientCapability(); ok {
		opts = append(opts, core.WithClientRounder(cliTrans))
	}
	// server handling
	if srvTrans, ok := st.ServerCapability(); ok {
		opts = append(opts, core.WithServerRounder(srvTrans))
	}
	ctx, conn, err := core.NewConnection(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
