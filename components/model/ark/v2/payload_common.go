/*
 * Copyright 2026 CloudWeGo Authors
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

package ark

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type responseCaptureKey struct{}
type responseCapture struct {
	rawBody []byte
}

type responseCaptureTransport struct {
	next http.RoundTripper
}

func (t *responseCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	capture, _ := req.Context().Value(responseCaptureKey{}).(*responseCapture)
	if capture == nil || (!strings.HasSuffix(req.URL.Path, "/chat/completions") && !strings.HasSuffix(req.URL.Path, "/responses")) {
		return t.next.RoundTrip(req)
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, err
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response payload: %w", err)
	}
	capture.rawBody = raw
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	return resp, nil
}
