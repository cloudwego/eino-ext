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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"github.com/cloudwego/eino-ext/devops/internal/utils/log"
)

var (
	logCh     = log.InitLogger()
	startOnce sync.Once
)

// StartHTTPServer init http sever use the specified port.
func StartHTTPServer(_ context.Context, port string) error {
	log.Infof("start debug http server at port=%s", port)
	var err error
	startOnce.Do(func() {
		r := mux.NewRouter()
		registerRoutes(r)
		err = http.ListenAndServe(":"+port, r)
		if err != nil {
			log.Errorf("start debug http server failed, err=%v", err)
		}
	})
	return err
}

func registerRoutes(r *mux.Router) {
	const (
		root     = "/eino/devops"
		debugBiz = "/debug/v1"
	)

	r.Use(recoverMiddleware, corsMiddleware)

	rootR := r.PathPrefix(root).Subrouter()
	rootR.Path("/ping").HandlerFunc(Ping).Methods(http.MethodGet)
	rootR.Path("/stream_log").HandlerFunc(StreamLog).Methods(http.MethodGet)
	rootR.Path("/version").HandlerFunc(Version).Methods(http.MethodGet)

	// debug routes
	debugR := rootR.PathPrefix(debugBiz).Subrouter()
	debugR.Path("/input_types").HandlerFunc(ListInputTypes).Methods(http.MethodGet)
	debugR.Path("/graphs").HandlerFunc(ListGraphs).Methods(http.MethodGet)
	debugR.Path("/graphs/{graph_id}/canvas").HandlerFunc(GetCanvasInfo).Methods(http.MethodGet)
	debugR.Path("/graphs/{graph_id}/threads").HandlerFunc(CreateDebugThread).Methods(http.MethodPost)
	debugR.Path("/graphs/{graph_id}/threads/{thread_id}/stream").HandlerFunc(StreamDebugRun).Methods(http.MethodPost)
}

type HTTPResp struct {
	BaseResp `json:",omitempty,inline"`
	Data     any `json:"data"`
}

// BaseResp http response.
type BaseResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

/*
newHTTPResp create a new HTTPResp.
data: the data to be returned.
baseResp: the base resp to be returned.
 1. if baseResp set, the base resp will be set to the base resp
 2. if baseResp not set and data is a BizError, the base resp will be set to http.StatusInternalServerError
 3. if baseResp not set and data is not a BizError, the base resp will be set to 0
*/
func newHTTPResp(data any, baseResp ...BaseResp) *HTTPResp {
	resp := &HTTPResp{
		Data:     data,
		BaseResp: BaseResp{Code: 0, Msg: "success"},
	}

	if len(baseResp) > 0 {
		resp.BaseResp = baseResp[0]
	} else if _, ok := data.(BizError); ok {
		resp.BaseResp = BaseResp{
			Code: http.StatusInternalServerError,
			Msg:  http.StatusText(http.StatusInternalServerError),
		}
	}

	if resp.Code != 0 {
		b, err := json.Marshal(resp)
		if err == nil {
			log.Errorf("request failed, resp=%s", string(b))
		}
	}

	return resp
}

func newBaseResp(code int, msg string) BaseResp {
	return BaseResp{
		Code: code,
		Msg:  msg,
	}
}

type BizError struct {
	BizCode int    `json:"biz_code"` // TODO: define biz code
	BizMsg  string `json:"biz_msg"`
}

func newBizError(bizCode int, err error) *BizError {
	return &BizError{
		BizCode: bizCode,
		BizMsg:  err.Error(),
	}
}

func (h HTTPResp) doResp(res http.ResponseWriter) {
	res.Header().Set("content-type", "application/json")
	out, err := json.Marshal(h)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = res.Write(out)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}

var (
	sseMutex      sync.Mutex
	sseRoutineNum = 0
)

const (
	sseMaxRoutineNum = 10
)

type StreamResponseType string

func NewStreamResponse(eventType string, data string) SSEResponse {
	return SSEResponse{
		eventType: eventType,
		data:      data,
	}
}

type SSEResponse struct {
	eventType string
	data      string
}

func (r SSEResponse) ToEventBytes() []byte {
	byWriter := bytes.NewBuffer(nil)
	byWriter.WriteString(fmt.Sprintf("event: %v\n", r.eventType))
	byWriter.WriteString(fmt.Sprintf("data: %v\n\n", r.data))
	return byWriter.Bytes()
}

func doSSEResp(ctx context.Context, res http.ResponseWriter, sseStreamRespChan <-chan SSEResponse) {
	sseMutex.Lock()
	if sseRoutineNum >= sseMaxRoutineNum {
		newHTTPResp(newBizError(http.StatusBadRequest, fmt.Errorf("too many connections")),
			newBaseResp(http.StatusBadRequest, "")).doResp(res)
		sseMutex.Unlock()
		return
	}
	sseRoutineNum += 1
	sseMutex.Unlock()

	defer func() {
		sseMutex.Lock()
		sseRoutineNum -= 1
		sseMutex.Unlock()
	}()

	res.Header().Set("Content-Type", "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")

	for {
		select {
		case <-ctx.Done():
			log.Errorf("client disconnect")
			return
		case response, ok := <-sseStreamRespChan:
			if !ok {
				return
			}
			_, err := res.Write(response.ToEventBytes())
			if err != nil {
				log.Errorf("write event failed, err=%v", err)
				return
			}

			if f, ok := res.(http.Flusher); ok {
				f.Flush()
			}

			<-time.After(50 * time.Millisecond)
		}
	}
}

func getPathParam(req *http.Request, key string) string {
	return mux.Vars(req)[key]
}

func getReqQuery(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

func getReqFromBody[T any](r *http.Request) (*T, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	ins := new(T)
	err = json.Unmarshal(body, &ins)
	if err != nil {
		return nil, err
	}

	return ins, nil
}
