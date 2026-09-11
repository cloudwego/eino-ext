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

package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"
)

type contextErrorTransport func(*http.Request) (*http.Response, error)

func (f contextErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type contextErrorBody struct {
	data   []byte
	finish func() error
}

func (b *contextErrorBody) Read(p []byte) (int, error) {
	if len(b.data) > 0 {
		n := copy(p, b.data)
		b.data = b.data[n:]
		return n, nil
	}
	return 0, b.finish()
}

func (*contextErrorBody) Close() error { return nil }

func TestStreamContextErrors(t *testing.T) {
	partial := `data: {"candidates": [{"content": {"role": "model","p`
	transportErr := errors.New("test transport read failed")
	validEvent := `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}]}` + "\n\n"
	tests := []struct {
		name        string
		data        string
		canceled    bool
		deadline    bool
		customCause bool
		readErr     error
		wantErr     error
		wantSyntax  bool
		wantContent string
		preCanceled bool
	}{
		{name: "cancel_without_partial_event", canceled: true, wantErr: context.Canceled},
		{name: "cancel_with_partial_event", data: partial, canceled: true, wantErr: context.Canceled},
		{name: "cancel_with_custom_cause", data: partial, canceled: true, customCause: true, wantErr: context.Canceled},
		{name: "deadline_without_partial_event", deadline: true, wantErr: context.DeadlineExceeded},
		{name: "deadline_with_partial_event", data: partial, deadline: true, wantErr: context.DeadlineExceeded},
		{name: "malformed_event_without_cancel", data: "data: {invalid}\n\n", readErr: io.EOF, wantSyntax: true},
		{name: "partial_event_without_cancel", data: partial, readErr: io.EOF, wantSyntax: true},
		{name: "transport_error_without_cancel", readErr: transportErr, wantErr: transportErr},
		{name: "deadline_read_error_with_live_context", readErr: context.DeadlineExceeded, wantErr: context.DeadlineExceeded},
		{name: "cancel_before_request", preCanceled: true, wantErr: context.Canceled},
		{name: "cancel_after_content", data: validEvent + partial, canceled: true, wantErr: context.Canceled, wantContent: "hello"},
		{name: "normal_completion", data: validEvent, readErr: io.EOF, wantErr: io.EOF, wantContent: "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			if tt.deadline {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, time.Second)
				defer stop()
			}
			body := &contextErrorBody{data: []byte(tt.data), finish: func() error {
				if tt.canceled {
					if tt.customCause {
						cancel(errors.New("test domain cause"))
					} else {
						cancel(nil)
					}
					return ctx.Err()
				}
				if tt.deadline {
					<-ctx.Done()
					return ctx.Err()
				}
				return tt.readErr
			}}
			client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
				APIKey: "test", Backend: genai.BackendGeminiAPI,
				HTTPClient: &http.Client{Transport: contextErrorTransport(func(req *http.Request) (*http.Response, error) {
					if tt.preCanceled {
						return nil, req.Context().Err()
					}
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: body, Request: req}, nil
				})},
			})
			if err != nil {
				t.Fatal(err)
			}
			cm, err := NewChatModel(ctx, &Config{Client: client, Model: "test-model"})
			if err != nil {
				t.Fatal(err)
			}
			callbackErr := make(chan error, 1)
			handler := callbacks.NewHandlerBuilder().OnEndWithStreamOutputFn(func(callbackCtx context.Context, _ *callbacks.RunInfo, stream *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
				go func() {
					defer stream.Close()
					for {
						_, err := stream.Recv()
						if err != nil {
							callbackErr <- err
							return
						}
					}
				}()
				return callbackCtx
			}).Build()
			ctx = callbacks.InitCallbacks(ctx, nil, handler)
			if tt.preCanceled {
				cancel(nil)
			}
			stream, err := cm.Stream(ctx, []*schema.Message{schema.UserMessage("test")})
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			var got error
			var content string
			for {
				msg, recvErr := stream.Recv()
				if recvErr != nil {
					got = recvErr
					break
				}
				content += msg.Content
			}
			if content != tt.wantContent {
				t.Errorf("caller: want content %q, got %q", tt.wantContent, content)
			}
			checkStreamContextError(t, "caller", got, tt.wantErr, tt.wantSyntax)
			select {
			case got = <-callbackErr:
				checkStreamContextError(t, "callback", got, tt.wantErr, tt.wantSyntax)
			case <-time.After(5 * time.Second):
				t.Fatal("callback did not receive stream error")
			}
		})
	}
}

func checkStreamContextError(t *testing.T, source string, got, want error, wantSyntax bool) {
	t.Helper()
	if wantSyntax {
		var syntaxErr *json.SyntaxError
		if !errors.As(got, &syntaxErr) || errors.Is(got, context.Canceled) || errors.Is(got, context.DeadlineExceeded) {
			t.Errorf("%s: want JSON syntax error, got %v", source, got)
		}
	} else if !errors.Is(got, want) {
		t.Errorf("%s: want %v, got %v", source, want, got)
	}
}
