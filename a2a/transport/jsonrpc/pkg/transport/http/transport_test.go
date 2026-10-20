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

package http

import (
	"context"
	"sync"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/pkg/conninfo"
)

type mockRequest struct {
	Msg string `json:"msg"`
}

type mockResponse struct {
	Msg string `json:"msg"`
}

type mockServerTransportHandler struct {
	mu            sync.Mutex
	t             *testing.T
	testRoundFunc func(t *testing.T, ctx context.Context, msg core.Message, msgWriter core.MessageWriter)
}

func (hdl *mockServerTransportHandler) OnTransport(ctx context.Context, trans core.Transport) error {
	hdl.mu.Lock()
	defer hdl.mu.Unlock()
	t := hdl.t
	rounder, ok := trans.ServerCapability()
	assert.True(t, ok)
	go func() {
		for {
			rCtx, msg, msgWriter, err := rounder.OnRound()
			assert.Nil(t, err)
			hdl.testRoundFunc(t, rCtx, msg, msgWriter)
		}
	}()

	return nil
}

func (hdl *mockServerTransportHandler) SetT(t *testing.T) {
	hdl.mu.Lock()
	defer hdl.mu.Unlock()
	hdl.t = t
}

func (hdl *mockServerTransportHandler) SetTestRoundFunc(f func(t *testing.T, ctx context.Context, msg core.Message, msgWriter core.MessageWriter)) {
	hdl.mu.Lock()
	defer hdl.mu.Unlock()
	hdl.testRoundFunc = f
}

func Test_SSE(t *testing.T) {
	srvHdl := &mockServerTransportHandler{}
	builder := NewServerTransportBuilder("/sse")
	ctx := context.Background()
	srvTrans, err := builder.Build(ctx, srvHdl)
	assert.Nil(t, err)
	go func() {
		srvTrans.ListenAndServe(ctx)
	}()
	defer srvTrans.Shutdown(ctx)
	t.Run("upstream Close", func(t *testing.T) {
		finished := make(chan struct{})
		srvHdl.SetT(t)
		srvHdl.SetTestRoundFunc(func(t *testing.T, ctx context.Context, msg core.Message, msgWriter core.MessageWriter) {
			defer func() {
				close(finished)
			}()
			jReq, ok := msg.(*core.Request)
			assert.True(t, ok)
			assert.Equal(t, "test_sse", jReq.Method)
			assert.Equal(t, "1", jReq.ID.String())
			mockReq := mockRequest{}
			assert.Nil(t, sonic.Unmarshal(jReq.Params, &mockReq))
			assert.Equal(t, "test_sse", mockReq.Msg)
			jResp, err := core.NewResponse(jReq.ID, &mockResponse{mockReq.Msg})
			assert.Nil(t, err)
			err = msgWriter.WriteStreaming(ctx, jResp)
			assert.Nil(t, err)
			// wait for connection closed
			<-ctx.Done()
			err = msgWriter.WriteStreaming(ctx, jResp)
			assert.NotNil(t, err)
			t.Log(err)
		})
		cliHdl := NewClientTransportHandler()
		trans, err := cliHdl.NewTransport(context.Background(), conninfo.NewPeer(conninfo.PeerTypeURL, "http://127.0.0.1:8888/sse"))
		assert.Nil(t, err)
		rounder, ok := trans.ClientCapability()
		assert.True(t, ok)
		req, err := core.NewRequest("test_sse", core.NewIDFromString("1"), &mockRequest{Msg: "test_sse"})
		assert.Nil(t, err)
		reader, err := rounder.Round(context.Background(), req)
		assert.Nil(t, err)
		msg, err := reader.Read(ctx)
		assert.Nil(t, err)
		jResp, ok := msg.(*core.Response)
		assert.True(t, ok)
		mockResp := mockResponse{}
		err = sonic.Unmarshal(jResp.Result, &mockResp)
		assert.Nil(t, err)
		assert.Equal(t, "test_sse", mockResp.Msg)
		reader.Close()
		_, err = reader.Read(ctx)
		assert.NotNil(t, err)
		t.Log(err)

		<-finished
	})
}
