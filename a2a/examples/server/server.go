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

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	hertz_server "github.com/cloudwego/hertz/pkg/app/server"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/server"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc"
)

func main() {
	ctx := context.Background()
	hz := hertz_server.Default(hertz_server.WithSenseClientDisconnection(true))
	r, err := jsonrpc.NewRegistrar(ctx, &jsonrpc.ServerConfig{
		Router:        hz,
		HandlerPath:   "/test",
		AgentCardPath: nil,
	})
	if err != nil {
		log.Fatal(err)
	}
	err = server.RegisterHandlers(ctx, r, &server.Config{
		AgentCardConfig: server.AgentCardConfig{
			Name:             "test agent",
			Description:      "a agent used for testing",
			URL:              "https://127.0.0.1:8080",
			Version:          "1",
			DocumentationURL: "xxx",
			Provider: &models.AgentProvider{
				Organization: "megumin",
				URL:          "yyy",
			},
			SecuritySchemes:    nil,
			Security:           nil,
			DefaultInputModes:  nil,
			DefaultOutputModes: nil,
			Skills:             nil,
		},
		MessageHandler: func(ctx context.Context, params *server.InputParams) (*models.TaskContent, error) {
			return &models.TaskContent{
				Status: models.TaskStatus{
					State: models.TaskStateCompleted,
					Message: &models.Message{
						Role: models.RoleAgent,
						Parts: []models.Part{
							{
								Kind: models.PartKindText,
								Text: ptrOf("hello world"),
							},
						},
					},
				},
				History:   params.Task.History,
				Artifacts: params.Task.Artifacts,
				Metadata:  params.Task.Metadata,
			}, nil
		},
		MessageStreamingHandler: func(ctx context.Context, params *server.InputParams, writer server.ResponseEventWriter) error {
			for i := 0; i < 3; i++ {
				time.Sleep(time.Second)
				err := writer.Write(
					models.ResponseEvent{
						TaskContent: &models.TaskContent{
							Status: models.TaskStatus{
								State: models.TaskStateWorking,
								Message: &models.Message{
									Role: models.RoleAgent,
									Parts: []models.Part{
										{
											Kind: models.PartKindText,
											Text: ptrOf(fmt.Sprintf("task message %d", i)),
										},
									},
								},
							},
						},
					})
				if err != nil {
					return err
				}
			}
			if strings.Contains(*params.Input.Parts[0].Text, "trigger error") {
				return fmt.Errorf("error has been triggered")
			}
			for i := 0; i < 3; i++ {
				time.Sleep(time.Second)
				err := writer.Write(
					models.ResponseEvent{
						TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{
							Status: models.TaskStatus{
								State: models.TaskStateCompleted,
								Message: &models.Message{
									Role: models.RoleAgent,
									Parts: []models.Part{
										{
											Kind: models.PartKindText,
											Text: ptrOf(fmt.Sprintf("status update message %d", i)),
										},
									},
								},
							},
							Final:    false,
							Metadata: nil,
						},
					},
				)
				if err != nil {
					return err
				}
			}
			return nil
		},
		CancelTaskHandler: func(ctx context.Context, params *server.InputParams) (*models.TaskContent, error) {
			return &models.TaskContent{
				Status: models.TaskStatus{
					State: models.TaskStateCanceled,
				},
				History:   params.Task.History,
				Artifacts: params.Task.Artifacts,
				Metadata:  params.Task.Metadata,
			}, nil
		},
		TaskEventsConsolidator: myTaskModifier,
		Logger: func(ctx context.Context, format string, v ...any) {
			log.Printf(format, v...)
		},
		TaskStore:    nil,
		TaskLocker:   nil,
		Queue:        nil,
		PushNotifier: server.NewInMemoryPushNotifier(),
	})
	if err != nil {
		log.Fatal(err)
	}

	hz.Run()
}

func myTaskModifier(ctx context.Context, t *models.Task, events []models.ResponseEvent, _ error) *models.TaskContent {
	result := &models.TaskContent{
		Status:    t.Status,
		Artifacts: make([]*models.Artifact, len(t.Artifacts)),
		History:   make([]*models.Message, len(t.History)),
		Metadata:  t.Metadata,
	}
	copy(result.Artifacts, t.Artifacts)
	copy(result.History, t.History)

	for _, e := range events {
		if e.TaskContent != nil {
			if result.Status.Message != nil {
				result.History = append(result.History, result.Status.Message)
			}
			result.Status = e.TaskContent.Status
			result.Metadata = e.TaskContent.Metadata
			result.Artifacts = append(result.Artifacts, e.TaskContent.Artifacts...)
			result.History = append(result.History, e.TaskContent.History...)
		} else if e.TaskStatusUpdateEventContent != nil {
			if result.Status.Message != nil {
				result.History = append(result.History, result.Status.Message)
			}
			result.Status = e.TaskStatusUpdateEventContent.Status
		} else if e.TaskArtifactUpdateEventContent != nil {
			if e.TaskArtifactUpdateEventContent.Append {
				result.Artifacts[len(result.Artifacts)-1].Parts = append(result.Artifacts[len(result.Artifacts)-1].Parts, e.TaskArtifactUpdateEventContent.Artifact.Parts...)
			} else {
				result.Artifacts = append(result.Artifacts, &e.TaskArtifactUpdateEventContent.Artifact)
			}
		}
	}
	return result
}

func ptrOf[T any](v T) *T {
	return &v
}
