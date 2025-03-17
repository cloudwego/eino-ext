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
	"net/http"

	"github.com/cloudwego/eino-ext/devops/internal/apihandler/types"
	"github.com/cloudwego/eino-ext/devops/internal/service"
	"github.com/cloudwego/eino-ext/devops/internal/utils/log"
	"github.com/cloudwego/eino-ext/devops/internal/utils/safego"
)

// Ping test ping.
func Ping(res http.ResponseWriter, _ *http.Request) {
	newHTTPResp("pong").doResp(res)
}

// Version return devops current version
func Version(res http.ResponseWriter, _ *http.Request) {
	newHTTPResp(types.Version).doResp(res)
}

// ListGraphs get all graphs.
func ListGraphs(res http.ResponseWriter, _ *http.Request) {
	graphNameToID := service.ContainerSVC.ListGraphs()
	graphs := make([]*types.GraphMeta, 0, len(graphNameToID))
	for name, id := range graphNameToID {
		graphs = append(graphs, &types.GraphMeta{
			ID:   id,
			Name: name,
		})
	}

	resp := &types.ListGraphsResponse{
		Graphs: graphs,
	}

	newHTTPResp(resp).doResp(res)
}

func StreamLog(res http.ResponseWriter, req *http.Request) {
	sseResponseChan := make(chan SSEResponse, 1000)
	ctx := req.Context()
	safego.Go(ctx, func() {
		for {
			select {
			case <-ctx.Done():
				log.Errorf("client disconnect")
				return
			case message, ok := <-logCh:
				if !ok {
					return
				}
				sseResponseChan <- NewStreamResponse(message.Level, message.Msg)
			}
		}

	})

	doSSEResp(ctx, res, sseResponseChan)
}
