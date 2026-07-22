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
	"errors"
	"io"
	"testing"

	"github.com/cloudwego/eino-ext/a2a/models"
)

type fakeReader struct {
	frames   []*models.SendMessageStreamingResponseUnion
	errs     []error
	idx      int
	closed   bool
	closeErr error
	closeCnt int
}

func (f *fakeReader) Read() (*models.SendMessageStreamingResponseUnion, error) {
	if f.idx >= len(f.frames) {
		return nil, io.EOF
	}
	frame := f.frames[f.idx]
	var err error
	if f.idx < len(f.errs) {
		err = f.errs[f.idx]
	}
	f.idx++
	return frame, err
}

func (f *fakeReader) Close() error {
	f.closed = true
	f.closeCnt++
	return f.closeErr
}

func TestServerStreamingWrapper_Recv_successDoesNotClose(t *testing.T) {
	r := &fakeReader{
		frames: []*models.SendMessageStreamingResponseUnion{{Task: &models.Task{ID: "t-1"}}},
	}
	w := &ServerStreamingWrapper{s: r}
	got, err := w.Recv()
	if err != nil {
		t.Fatalf("Recv err: %v", err)
	}
	if got == nil || got.Task == nil || got.Task.ID != "t-1" {
		t.Errorf("Recv got %+v", got)
	}
	if r.closed {
		t.Errorf("underlying reader should not be closed on successful Recv")
	}
}

func TestServerStreamingWrapper_Recv_errorClosesReader(t *testing.T) {
	wantErr := errors.New("boom")
	r := &fakeReader{
		frames: []*models.SendMessageStreamingResponseUnion{nil},
		errs:   []error{wantErr},
	}
	w := &ServerStreamingWrapper{s: r}
	_, err := w.Recv()
	if !errors.Is(err, wantErr) {
		t.Errorf("err: got %v, want %v", err, wantErr)
	}
	if !r.closed {
		t.Errorf("underlying reader should be closed when Recv returns an error")
	}
}

func TestServerStreamingWrapper_Recv_eofClosesReader(t *testing.T) {
	r := &fakeReader{} // immediately returns io.EOF
	w := &ServerStreamingWrapper{s: r}
	_, err := w.Recv()
	if err != io.EOF {
		t.Errorf("err: got %v, want io.EOF", err)
	}
	if !r.closed {
		t.Errorf("EOF is non-nil error, reader should be closed")
	}
}

func TestServerStreamingWrapper_Close(t *testing.T) {
	r := &fakeReader{}
	w := &ServerStreamingWrapper{s: r}
	if err := w.Close(); err != nil {
		t.Errorf("Close err: %v", err)
	}
	if !r.closed {
		t.Errorf("Close should propagate to underlying reader")
	}
}

func TestServerStreamingWrapper_Close_propagatesError(t *testing.T) {
	wantErr := errors.New("close-fail")
	r := &fakeReader{closeErr: wantErr}
	w := &ServerStreamingWrapper{s: r}
	if err := w.Close(); !errors.Is(err, wantErr) {
		t.Errorf("Close err: got %v, want %v", err, wantErr)
	}
}
