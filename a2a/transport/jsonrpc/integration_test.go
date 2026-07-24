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

package jsonrpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	hertz_server "github.com/cloudwego/hertz/pkg/app/server"

	"github.com/cloudwego/eino-ext/a2a/models"
)

func ptrOf[T any](v T) *T { return &v }

// echoHandlers returns a minimal set of A2A handlers used by the dual-version
// integration test. They are version-agnostic: they operate purely on the
// shared models types.
func echoHandlers() *models.ServerHandlers {
	return &models.ServerHandlers{
		AgentCard: func(ctx context.Context) *models.AgentCard {
			return &models.AgentCard{Name: "dual", ProtocolVersion: "0.2.5"}
		},
		SendMessage: func(ctx context.Context, params *models.MessageSendParams) (*models.SendMessageResponseUnion, error) {
			// Echo the inbound text back inside a completed task.
			var text string
			if len(params.Message.Parts) > 0 && params.Message.Parts[0].Text != nil {
				text = *params.Message.Parts[0].Text
			}
			return &models.SendMessageResponseUnion{
				Task: &models.Task{
					ID:        "task-1",
					ContextID: "ctx-1",
					Status: models.TaskStatus{
						State: models.TaskStateCompleted,
						Message: &models.Message{
							Role:      models.RoleAgent,
							MessageID: "reply-1",
							Parts:     []models.Part{{Kind: models.PartKindText, Text: ptrOf("echo:" + text)}},
						},
					},
				},
			}, nil
		},
		SendMessageStreaming: func(ctx context.Context, params *models.MessageSendParams, writer models.ResponseWriter) error {
			for i := 0; i < 2; i++ {
				if err := writer.Write(ctx, &models.SendMessageStreamingResponseUnion{
					TaskStatusUpdateEvent: &models.TaskStatusUpdateEvent{
						TaskID:    "task-1",
						ContextID: "ctx-1",
						Status: models.TaskStatus{
							State:   models.TaskStateWorking,
							Message: &models.Message{Role: models.RoleAgent, MessageID: fmt.Sprintf("s-%d", i), Parts: []models.Part{{Kind: models.PartKindText, Text: ptrOf(fmt.Sprintf("chunk-%d", i))}}},
						},
					},
				}); err != nil {
					return err
				}
			}
			return nil
		},
		GetTask: func(ctx context.Context, params *models.TaskQueryParams) (*models.Task, error) {
			return &models.Task{ID: params.ID, Status: models.TaskStatus{State: models.TaskStateCompleted}}, nil
		},
		CancelTask: func(ctx context.Context, params *models.TaskIDParams) (*models.Task, error) {
			return &models.Task{ID: params.ID, Status: models.TaskStatus{State: models.TaskStateCanceled}}, nil
		},
	}
}

// startDualServer starts a real hertz server serving both protocol versions on
// one endpoint, and returns its base URL.
func startDualServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx := context.Background()
	hz := hertz_server.New(hertz_server.WithHostPorts(addr))
	reg, err := NewRegistrar(ctx, &ServerConfig{
		Router:      hz,
		HandlerPath: "/a2a",
		// default ProtocolVersions => {v0.3, v1.0}
	})
	if err != nil {
		t.Fatalf("NewRegistrar: %v", err)
	}
	if err := reg.Register(ctx, echoHandlers()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	go hz.Spin()

	base := "http://" + addr
	// Wait for the server to accept connections.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return base
}

func sendMessageParams(text string) *models.MessageSendParams {
	return &models.MessageSendParams{
		Message: models.Message{
			Role:      models.RoleUser,
			MessageID: "u-1",
			Parts:     []models.Part{{Kind: models.PartKindText, Text: ptrOf(text)}},
		},
	}
}

// TestDualVersionCoexistence verifies a single endpoint serves both a v0.3 and
// a v1.0 client for the core operations, which is the crux of the smooth-migration design.
func TestDualVersionCoexistence(t *testing.T) {
	base := startDualServer(t)
	ctx := context.Background()

	for _, v := range []models.ProtocolVersion{models.ProtocolVersion03, models.ProtocolVersion10} {
		v := v
		t.Run(string(v), func(t *testing.T) {
			tr, err := NewTransport(ctx, &ClientConfig{
				BaseURL:         base,
				HandlerPath:     "/a2a",
				ProtocolVersion: v,
			})
			if err != nil {
				t.Fatalf("NewTransport: %v", err)
			}
			defer tr.Close()

			// SendMessage
			resp, err := tr.SendMessage(ctx, sendMessageParams("hi"))
			if err != nil {
				t.Fatalf("SendMessage: %v", err)
			}
			if resp == nil || resp.Task == nil {
				t.Fatalf("SendMessage got %+v", resp)
			}
			if resp.Task.Status.State != models.TaskStateCompleted {
				t.Errorf("state = %q, want completed", resp.Task.Status.State)
			}
			msg := resp.Task.Status.Message
			if msg == nil || len(msg.Parts) == 0 || msg.Parts[0].Text == nil || *msg.Parts[0].Text != "echo:hi" {
				t.Errorf("reply text lost: %+v", msg)
			}
			if msg.Role != models.RoleAgent {
				t.Errorf("reply role = %q, want agent", msg.Role)
			}

			// GetTask
			task, err := tr.GetTask(ctx, &models.TaskQueryParams{ID: "task-1"})
			if err != nil {
				t.Fatalf("GetTask: %v", err)
			}
			if task.ID != "task-1" || task.Status.State != models.TaskStateCompleted {
				t.Errorf("GetTask got %+v", task)
			}

			// CancelTask
			cancelled, err := tr.CancelTask(ctx, &models.TaskIDParams{ID: "task-1"})
			if err != nil {
				t.Fatalf("CancelTask: %v", err)
			}
			if cancelled.Status.State != models.TaskStateCanceled {
				t.Errorf("CancelTask state = %q, want canceled", cancelled.Status.State)
			}

			// Streaming
			reader, err := tr.SendMessageStreaming(ctx, sendMessageParams("stream"))
			if err != nil {
				t.Fatalf("SendMessageStreaming: %v", err)
			}
			var frames int
			for {
				frame, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("stream Read: %v", err)
				}
				if frame.TaskStatusUpdateEvent == nil {
					t.Errorf("stream frame missing status update: %+v", frame)
					continue
				}
				if frame.TaskStatusUpdateEvent.Status.State != models.TaskStateWorking {
					t.Errorf("stream frame state = %q", frame.TaskStatusUpdateEvent.Status.State)
				}
				frames++
			}
			reader.Close()
			if frames != 2 {
				t.Errorf("stream frames = %d, want 2", frames)
			}
		})
	}
}
