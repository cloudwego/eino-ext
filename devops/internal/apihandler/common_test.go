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
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/devops/internal/utils/log"
)

func TestStreamLog(t *testing.T) {
	w := &mockWriter{t: t}
	req, _ := http.NewRequest(http.MethodGet, "", nil)
	logCh <- log.Message{Msg: "1", Level: "1"}
	logCh <- log.Message{Msg: "1", Level: "1"}
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*3)
	_ = cancelFunc
	req = req.WithContext(ctx)
	StreamLog(w, req)

}
