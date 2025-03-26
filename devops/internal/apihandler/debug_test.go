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
	"strings"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/cloudwego/eino-ext/devops/internal/apihandler/types"
	"github.com/cloudwego/eino-ext/devops/internal/mock"
	"github.com/cloudwego/eino-ext/devops/internal/model"
	"github.com/cloudwego/eino-ext/devops/internal/service"
	devmodel "github.com/cloudwego/eino-ext/devops/model"
)

type debugTestSuite struct {
	suite.Suite

	t                *testing.T
	mockContainerSVC *mock.MockContainerService
	mockDebugSVC     *mock.MockDebugService
}

func Test_debug_run(t *testing.T) {
	suite.Run(t, new(debugTestSuite))
}

type mockResponseWriter struct {
	body []byte
}

func (m *mockResponseWriter) Header() http.Header {
	return map[string][]string{}
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	m.body = b
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
}

func (d *debugTestSuite) SetupSuite() {
	d.t = d.T()
	ctrl := gomock.NewController(d.T())

	d.mockContainerSVC = mock.NewMockContainerService(ctrl)
	d.mockDebugSVC = mock.NewMockDebugService(ctrl)

	service.DebugSVC = d.mockDebugSVC
	service.ContainerSVC = d.mockContainerSVC
}

func (d *debugTestSuite) Test_GetCanvasInfo() {
	mockey.PatchConvey("", d.t, func() {
		mockGraphID := "mock_graph"
		mockey.Mock(getPathParam).Return(mockGraphID).Build()

		d.mockContainerSVC.EXPECT().GetCanvas(mockGraphID).Return(devmodel.CanvasInfo{
			GraphSchema: &devmodel.GraphSchema{},
		}, false).Times(1)
		d.mockContainerSVC.EXPECT().CreateCanvas(mockGraphID).Return(devmodel.CanvasInfo{
			GraphSchema: &devmodel.GraphSchema{
				Name: "mock_canvas",
			},
		}, nil).Times(1)

		req, err := http.NewRequest(http.MethodGet, "", nil)
		assert.Nil(d.t, err)
		res := &mockResponseWriter{}
		GetCanvasInfo(res, req)

		resp := &HTTPResp{}
		err = json.Unmarshal(res.body, &resp)
		assert.Nil(d.t, err)
		b, err := json.Marshal(resp.Data)
		assert.Nil(d.t, err)
		var data *types.GetCanvasInfoResponse
		err = json.Unmarshal(b, &data)
		assert.Nil(d.t, err)
		assert.Equal(d.t, "mock_canvas", data.CanvasInfo.Name)
	})
}

func (d *debugTestSuite) Test_CreateDebugThread() {
	mockey.PatchConvey("", d.t, func() {
		mockGraphID := "mock_graph"
		mockey.Mock(validateCreateDebugThreadRequest).Return(nil).Build()
		mockey.Mock(getPathParam).Return(mockGraphID).Build()
		d.mockDebugSVC.EXPECT().CreateDebugThread(gomock.Any(), mockGraphID).Return("mock_thread_id", nil).Times(1)

		req, err := http.NewRequest(http.MethodGet, "", nil)
		assert.Nil(d.t, err)
		res := &mockResponseWriter{}
		CreateDebugThread(res, req)

		resp := &HTTPResp{}
		err = json.Unmarshal(res.body, &resp)
		assert.Nil(d.t, err)
		b, err := json.Marshal(resp.Data)
		assert.Nil(d.t, err)
		var data *types.CreateDebugThreadResponse
		err = json.Unmarshal(b, &data)
		assert.Nil(d.t, err)
		assert.Equal(d.t, "mock_thread_id", data.ThreadID)
	})
}

func (d *debugTestSuite) Test_validateCreateDebugThreadRequest() {
	mockey.PatchConvey("graph_id is nil", d.t, func() {
		mockey.Mock(getPathParam).Return("").Build()
		req, err := http.NewRequest(http.MethodGet, "", nil)
		assert.Nil(d.t, err)
		err = validateCreateDebugThreadRequest(req)
		assert.NotNil(d.t, err)
	})
}

func (d *debugTestSuite) Test_DebugRun() {
	mockey.PatchConvey("", d.t, func() {
		mockey.Mock(validateDebugRunRequest).Return(&types.DebugRunRequest{}, nil).Build()

		mockGraphID := "mock_graph"
		threadID := "mock_thread_id"
		mockey.Mock(getPathParam).To(func(req *http.Request, key string) string {
			if key == "graph_id" {
				return mockGraphID
			}
			if key == "thread_id" {
				return threadID
			}
			return ""
		}).Build()

		stateCh := make(chan *model.NodeDebugState, 100)
		errCh := make(chan error, 1)
		d.mockDebugSVC.EXPECT().DebugRun(gomock.Any(), gomock.Any(), gomock.Any()).
			Return("mock_debug_id", stateCh, errCh, nil).Times(1)

		stateCh <- &model.NodeDebugState{
			NodeKey: "mock_node_key",
		}

		close(stateCh)
		close(errCh)

		mockey.Mock(doSSEResp).To(func(ctx context.Context, res http.ResponseWriter, sseResponseChan <-chan SSEResponse) {
			response, ok := <-sseResponseChan
			assert.True(d.t, ok)
			var evt *types.DebugRunEventMsg
			err := json.Unmarshal([]byte(response.data), &evt)
			assert.Nil(d.t, err)
			assert.Equal(d.t, evt.Type, types.DebugRunEventType("data"))
			assert.Equal(d.t, evt.Content.NodeKey, "mock_node_key")

			response, ok = <-sseResponseChan
			assert.True(d.t, ok)
			err = json.Unmarshal([]byte(response.data), &evt)
			assert.Nil(d.t, err)
			assert.Equal(d.t, evt.Type, types.DebugRunEventType("finish"))

			response, ok = <-sseResponseChan
			assert.False(d.t, ok)
		}).Build()

		req, err := http.NewRequest(http.MethodGet, "", nil)
		assert.Nil(d.t, err)
		res := &mockResponseWriter{}
		StreamDebugRun(res, req)
	})
}

func (d *debugTestSuite) Test_validateDebugRunRequest() {
	reader := strings.NewReader(`{"from_node":"start"}`)
	req, err := http.NewRequest(http.MethodPost, "", reader)
	req = mux.SetURLVars(req, map[string]string{
		"graph_id":  "mock_graph_id",
		"thread_id": "mock_thread_id",
	})
	assert.Nil(d.t, err)
	r, err := validateDebugRunRequest(req)
	assert.Nil(d.t, err)
	assert.Equal(d.t, "start", r.FromNode)
}

func (d *debugTestSuite) Test_ListInputTypes() {
	req, err := http.NewRequest(http.MethodGet, "", nil)
	assert.Nil(d.t, err)
	res := &mockResponseWriter{}
	ListInputTypes(res, req)

	resp := &HTTPResp{}
	err = json.Unmarshal(res.body, &resp)
	assert.Nil(d.t, err)
	b, err := json.Marshal(resp.Data)
	assert.Nil(d.t, err)
	var data *types.ListInputTypesResponse
	err = json.Unmarshal(b, &data)
	assert.Nil(d.t, err)
	assert.Greater(d.t, len(data.Types), 0)
}
