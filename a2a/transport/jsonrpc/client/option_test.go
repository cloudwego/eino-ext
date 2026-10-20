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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/conninfo"
)

type testClientTransportHandler struct{}

func (t *testClientTransportHandler) NewTransport(ctx context.Context, peer conninfo.Peer) (core.Transport, error) {
	panic("implement me")
}

func TestWithOptions(t *testing.T) {
	// default Options
	defOptions := defaultOptions()
	testURL := "http://127.0.0.1:8888/testing"
	testHdl := &testClientTransportHandler{}
	opts := []Option{
		WithURL(testURL),
		WithTransportHandler(testHdl),
	}
	for _, opt := range opts {
		opt(&defOptions)
	}
	assert.Equal(t, testURL, defOptions.url)
	assert.Equal(t, testHdl, defOptions.hdl)
}

func TestWithJSONRPCIDGenerator(t *testing.T) {
	ctx := context.Background()
	// default
	cth := &mockClientTransportHandler{func(_ context.Context, _ conninfo.Peer) (core.Transport, error) {
		return &mockTransport{
			cr: &mockClientRounder{func(_ context.Context, m core.Message) (core.MessageReader, error) {
				req := m.(*core.Request)
				assert.NotNil(t, req.ID.Str)
				return &mockMessageReader{func(ctx context.Context) (core.Message, error) {
					return &core.Response{
						Version: req.Version,
						ID:      req.ID,
						Result:  []byte("{}"),
					}, nil
				}}, nil
			}},
		}, nil
	}}
	cli, err := NewClient(WithTransportHandler(cth), WithURL("123"))
	assert.NoError(t, err)
	conn, err := cli.NewConnection(ctx)
	assert.NoError(t, err)
	var resp json.RawMessage
	err = conn.Call(ctx, "", "", &resp)
	assert.NoError(t, err)

	// with id gen
	cth = &mockClientTransportHandler{func(_ context.Context, _ conninfo.Peer) (core.Transport, error) {
		return &mockTransport{
			cr: &mockClientRounder{func(_ context.Context, m core.Message) (core.MessageReader, error) {
				req := m.(*core.Request)
				assert.NotNil(t, req.ID.Num)
				return &mockMessageReader{func(ctx context.Context) (core.Message, error) {
					return &core.Response{
						Version: req.Version,
						ID:      req.ID,
						Result:  []byte("{}"),
					}, nil
				}}, nil
			}},
		}, nil
	}}
	cli, err = NewClient(WithTransportHandler(cth), WithURL("123"), WithJSONRPCIDGenerator(func(ctx context.Context) core.ID {
		var num = 1.0
		return core.ID{Num: &num}
	}))
	assert.NoError(t, err)
	conn, err = cli.NewConnection(ctx)
	assert.NoError(t, err)
	err = conn.Call(ctx, "", "", &resp)
	assert.NoError(t, err)
}

type mockClientTransportHandler struct {
	f func(ctx context.Context, peer conninfo.Peer) (core.Transport, error)
}

func (m *mockClientTransportHandler) NewTransport(ctx context.Context, peer conninfo.Peer) (core.Transport, error) {
	return m.f(ctx, peer)
}

type mockTransport struct {
	cr core.ClientRounder
	sr core.ServerRounder
}

func (m *mockTransport) ClientCapability() (core.ClientRounder, bool) {
	if m.cr == nil {
		return nil, false
	}
	return m.cr, true
}

func (m *mockTransport) ServerCapability() (core.ServerRounder, bool) {
	if m.sr == nil {
		return nil, false
	}
	return m.sr, true
}

type mockClientRounder struct {
	f func(ctx context.Context, msg core.Message) (core.MessageReader, error)
}

func (m *mockClientRounder) Round(ctx context.Context, msg core.Message) (core.MessageReader, error) {
	return m.f(ctx, msg)
}

type mockMessageReader struct {
	f func(ctx context.Context) (core.Message, error)
}

func (m *mockMessageReader) Read(ctx context.Context) (core.Message, error) {
	return m.f(ctx)
}

func (m *mockMessageReader) Close() error {
	return nil
}
