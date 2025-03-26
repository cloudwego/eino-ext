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
	"fmt"
	"net/http"
	"sync"

	"github.com/cloudwego/eino-ext/devops/internal/apihandler/types"
	"github.com/cloudwego/eino-ext/devops/internal/model"
	"github.com/cloudwego/eino-ext/devops/internal/service"
	"github.com/cloudwego/eino-ext/devops/internal/utils/log"
	"github.com/cloudwego/eino-ext/devops/internal/utils/safego"
	devmodel "github.com/cloudwego/eino-ext/devops/model"
	"github.com/cloudwego/eino/compose"
)

func InitDebug(opt *model.DevOpt) {
	compose.InitGraphCompileCallbacks([]compose.GraphCompileCallback{
		service.NewGlobalDevGraphCompileCallback(),
	})
	for _, rt := range opt.GoTypes {
		model.RegisterType(rt.Type)
	}
}

// GetCanvasInfo use graph name to  get canvas info
func GetCanvasInfo(res http.ResponseWriter, req *http.Request) {
	var (
		graphID    string
		ok         bool
		err        error
		canvasInfo devmodel.CanvasInfo
	)

	graphID = getPathParam(req, "graph_id")
	if len(graphID) == 0 {
		newHTTPResp(newBizError(http.StatusBadRequest, fmt.Errorf("graph_name is empty")), newBaseResp(http.StatusBadRequest, "")).doResp(res)
		return
	}

	canvasInfo, ok = service.ContainerSVC.GetCanvas(graphID)
	if !ok {
		canvasInfo, err = service.ContainerSVC.CreateCanvas(graphID)
		if err != nil {
			newHTTPResp(newBizError(http.StatusBadRequest, err), newBaseResp(http.StatusBadRequest, "")).doResp(res)
			return
		}
	}

	resp := &types.GetCanvasInfoResponse{
		CanvasInfo: canvasInfo,
	}

	newHTTPResp(resp).doResp(res)
}

// CreateDebugThread create thread_id.
func CreateDebugThread(res http.ResponseWriter, req *http.Request) {
	err := validateCreateDebugThreadRequest(req)
	if err != nil {
		newHTTPResp(newBizError(http.StatusBadRequest, err), newBaseResp(http.StatusBadRequest, "")).doResp(res)
		return
	}

	var graphID = getPathParam(req, "graph_id")

	threadID, err := service.DebugSVC.CreateDebugThread(req.Context(), graphID)
	if err != nil {
		newHTTPResp(newBizError(http.StatusInternalServerError, err)).doResp(res)
		return
	}

	resp := &types.CreateDebugThreadResponse{
		ThreadID: threadID,
	}

	newHTTPResp(resp).doResp(res)
}

func validateCreateDebugThreadRequest(req *http.Request) error {
	graphID := getPathParam(req, "graph_id")
	if graphID == "" {
		return fmt.Errorf("graph_id is empty")
	}

	return nil
}

// StreamDebugRun run using mock input data.
func StreamDebugRun(res http.ResponseWriter, req *http.Request) {
	var err error
	defer func() {
		if err != nil {
			sseResp := make(chan SSEResponse, 1)
			evt := types.DebugRunErrEVT("", err.Error())
			sseResp <- NewStreamResponse(string(evt.Type), string(evt.JsonBytes()))
			close(sseResp)
			doSSEResp(req.Context(), res, sseResp)
		}
	}()

	rs, err := validateDebugRunRequest(req)
	if err != nil {
		log.Errorf(err.Error())
		return
	}

	var (
		ctx      = req.Context()
		graphID  = getPathParam(req, "graph_id")
		threadID = getPathParam(req, "thread_id")
	)

	ctx = context.WithValue(ctx, "K_LOGID", rs.LogID)

	m := &model.DebugRunMeta{
		GraphID:  graphID,
		ThreadID: threadID,
		FromNode: rs.FromNode,
	}

	debugID, stateCh, errCh, err := service.DebugSVC.DebugRun(ctx, m, rs.Input)
	if err != nil {
		log.Errorf(err.Error())
		return
	}

	sseStreamResponseChan := make(chan SSEResponse, 50)
	wg := sync.WaitGroup{}
	wg.Add(1)
	safego.Go(ctx, func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case state, ok := <-stateCh:
				if !ok {
					evt := types.DebugRunFinishEVT(debugID)
					sseStreamResponseChan <- NewStreamResponse(string(evt.Type), string(evt.JsonBytes()))
					return
				}

				evt := types.DebugRunDataEVT(debugID, state)
				if err != nil {
					errEvt := types.DebugRunErrEVT(debugID, err.Error())
					sseStreamResponseChan <- NewStreamResponse(string(errEvt.Type), string(evt.JsonBytes()))
					return
				}
				sseStreamResponseChan <- NewStreamResponse(string(evt.Type), string(evt.JsonBytes()))
			}
		}
	})

	safego.Go(ctx, func() {
		defer close(sseStreamResponseChan)
		wg.Wait()
		for e := range errCh {
			evt := types.DebugRunErrEVT(debugID, e.Error())
			sseStreamResponseChan <- NewStreamResponse(string(evt.Type), string(evt.JsonBytes()))
		}
	})

	doSSEResp(req.Context(), res, sseStreamResponseChan)
}

func validateDebugRunRequest(req *http.Request) (*types.DebugRunRequest, error) {
	graphID := getPathParam(req, "graph_id")
	if graphID == "" {
		return nil, fmt.Errorf("graph_id is empty")
	}

	threadID := getPathParam(req, "thread_id")
	if threadID == "" {
		return nil, fmt.Errorf("thread_id is empty")
	}

	r, err := getReqFromBody[types.DebugRunRequest](req)
	if err != nil {
		return nil, err
	}

	if r.FromNode == "" {
		return nil, fmt.Errorf("from_node is empty")
	}

	return r, nil
}

func ListInputTypes(res http.ResponseWriter, req *http.Request) {
	resp := &types.ListInputTypesResponse{
		Types: model.GetRegisteredTypeJsonSchema(),
	}
	newHTTPResp(resp).doResp(res)
}
