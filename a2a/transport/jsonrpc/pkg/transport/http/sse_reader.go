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
	"bufio"
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

// fork from github.com/cloudwego/hertz/pkg/protocol/sse/reader.go
// to read SSE stream from io.ReadCloser

type Reader struct {
	r      io.ReadCloser
	s      *bufio.Scanner
	events int32

	lastEventID string
}

func newSSEReader(body io.ReadCloser) *Reader {
	r := &Reader{r: body}
	r.s = bufio.NewScanner(r.r)
	r.s.Split(scanEOL)
	return r
}

func (r *Reader) SetMaxBufferSize(max int) {
	r.s.Buffer(nil, max)
}

func (r *Reader) ForEach(ctx context.Context, f func(e *sse.Event) error) error {
	e := sse.NewEvent()
	defer e.Release()
	defer r.r.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.Read(e); err != nil {
			if err == io.EOF {
				return nil
			}
			if er := ctx.Err(); er != nil {
				err = er
			}
			return err
		}
		if err := f(e); err != nil {
			return err
		}
	}
}

// LastEventID returns the last event ID read by the reader.
func (r *Reader) LastEventID() string {
	return r.lastEventID
}

func (r *Reader) onEventRead(e *sse.Event) {
	r.events++
	if e.IsSetID() {
		r.lastEventID = e.ID
	}
}

func (r *Reader) Read(e *sse.Event) error {
	e.Reset()
	for i := 0; r.s.Scan(); i++ {
		line := r.s.Bytes()

		// Trim UTF8 BOM
		if i == 0 && r.events == 0 && bytes.HasPrefix(line, []byte{0xEF, 0xBB, 0xBF}) {
			line = line[3:]
		}

		if len(line) == 0 {
			if e.IsSetData() || e.IsSetID() || e.IsSetRetry() || e.IsSetType() {
				r.onEventRead(e)
				return nil
			}
			continue // Skip empty lines at the beginning
		}

		if line[0] == ':' {
			// Comment which starts with colon
			continue
		}

		// Parse field
		var f, v []byte
		i := bytes.IndexByte(line, ':')
		if i < 0 {
			// No colon, the entire line is the field name with an empty value
			f = line
		} else {
			f = line[:i]
			// If the colon is followed by a space, remove it
			if i+1 < len(line) && line[i+1] == ' ' {
				v = line[i+2:]
			} else {
				v = line[i+1:]
			}
		}

		// Process the field
		switch string(f) {
		case "event":
			e.SetEvent(sseEventType(v))
		case "data":
			if len(e.Data) > 0 {
				// If we already have data, append a newline before the new data
				e.Data = append(e.Data, '\n')
			}
			e.AppendData(v)
		case "id":
			id := string(v)
			// Ignore if it contains Null
			if !strings.Contains(id, "\u0000") {
				e.SetID(id)
			}
		case "retry":
			if retry, err := strconv.ParseInt(string(v), 10, 64); err == nil {
				e.SetRetry(time.Duration(retry) * time.Millisecond)
			}
		default:
			// As per spec, ignore if it's not defined.
		}
	}
	// Check if scanner encountered an error
	if err := r.s.Err(); err != nil {
		return err
	}
	if !(e.IsSetData() || e.IsSetID() || e.IsSetRetry() || e.IsSetType()) {
		return io.EOF
	}
	r.onEventRead(e)
	return nil
}

// https://html.spec.whatwg.org/multipage/server-sent-events.html#parsing-an-event-stream
// end-of-line   = ( cr lf / cr / lf )
func scanEOL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	i := bytes.IndexByte(data, '\r')
	j := bytes.IndexByte(data, '\n')
	if i >= 0 {
		if i+1 == j { // \r\n
			return i + 2, data[0:i], nil
		}
		if j >= 0 { // choose the nearer \r or \n as EOL
			if i < j {
				return i + 1, data[0:i], nil // \r
			}
			return j + 1, data[0:j], nil // \n
		}
		// if ends with '\r', we need to check the next char is NOT '\n' as per spec
		// this may cause unexpected blocks on reading more data.
		if i < len(data)-1 || atEOF {
			return i + 1, data[0:i], nil
		}
	} else if j >= 0 {
		return j + 1, data[0:j], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil // more data
}

func sseEventType(v []byte) string {
	switch string(v) {
	case "message":
		return "message"
	}
	return string(v)
}
