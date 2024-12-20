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

package apihandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/devops/internal/utils/safego"
)

func Test_doResp(t *testing.T) {
	res := httptest.NewRecorder()
	PatchConvey("Test normal case", t, func() {
		msg := "success"
		code := 0
		data := "data"
		newHTTPResp(data).doResp(res)
		assert.Equal(t, res.Code, 200)
		var actualRes HTTPResp
		err := json.Unmarshal(res.Body.Bytes(), &actualRes)
		assert.Nil(t, err)
		assert.Equal(t, actualRes.Msg, msg)
		assert.Equal(t, actualRes.Code, code)
		assert.Equal(t, actualRes.Data, data)
	})
}

func TestNewStreamResponse(t *testing.T) {
	r := NewStreamResponse("error", "error")
	assert.Contains(t, string(r.ToEventBytes()), "error")
}

type mockWriter struct {
	t *testing.T
}

func (m *mockWriter) Header() http.Header {
	return http.Header{}
}

func (m *mockWriter) Write(bytes []byte) (int, error) {
	assert.Contains(m.t, string(bytes), "1")
	return len(bytes), nil
}

func (m *mockWriter) WriteHeader(statusCode int) {
	return
}

func Test_doSSEResp(t *testing.T) {
	ctx := context.Background()
	sseStreamRespChan := make(chan SSEResponse, 100)

	safego.Go(ctx, func() {
		for i := 0; i < 10; i++ {
			sseStreamRespChan <- NewStreamResponse("1", "")
		}
		close(sseStreamRespChan)
	})

	doSSEResp(context.Background(), &mockWriter{t: t}, sseStreamRespChan)

}
