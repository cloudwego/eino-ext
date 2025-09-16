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

package langfuse

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	sdkName        = "Golang"
	sdkIntegration = "eino"
	sdkVersion     = "v0.0.1"
)

//go:generate mockgen -source=langfuse.go -destination=./mock/langfuse_mock.go -package=mock Langfuse
type Langfuse interface {
	CreateTrace(body *TraceEventBody) (string, error)
	CreateSpan(body *SpanEventBody) (string, error)
	EndSpan(body *SpanEventBody) error
	CreateGeneration(body *GenerationEventBody) (string, error)
	EndGeneration(body *GenerationEventBody) error
	CreateEvent(body *EventEventBody) (string, error)
	Flush()
	Prompt() *promptClient
}

// NewLangfuse creates a Langfuse client instance
//
// Parameters:
//   - host: The Langfuse API host URL
//   - publicKey: Your Langfuse public API key
//   - secretKey: Your Langfuse secret API key
//   - opts: Optional configuration parameters for the client
//
// Returns:
//   - Langfuse: A new Langfuse client interface implementation
//
// The client handles communication with the Langfuse API for tracking traces,
// spans, generations and events. It includes features like:
//   - Automatic batching and queueing of events
//   - Configurable flush intervals and batch sizes
//   - Retry logic for failed API calls
//   - Sampling rate control
func NewLangfuse(
	host string,
	publicKey string,
	secretKey string,
	opts ...Option,
) Langfuse {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	httpCli := &http.Client{Timeout: o.timeout}
	tm := newTaskManager(
		o.threads,
		httpCli,
		host,
		o.maxTaskQueueSize,
		o.flushAt,
		o.flushInterval,
		o.sampleRate,
		o.logMessage,
		o.maskFunc,
		sdkName,
		sdkVersion,
		sdkIntegration,
		publicKey,
		secretKey,
		o.maxRetry,
	)
	langfuseCli := newClient(httpCli, host, publicKey, secretKey, sdkVersion)
	promptCli := &promptClient{cli: langfuseCli}
	return &langfuseIns{tm: tm, promptCli: promptCli}
}

type langfuseIns struct {
	tm        *taskManager
	promptCli *promptClient
}

// CreateTrace creates a new trace in Langfuse
//
// Parameters:
//   - body: The trace event details. If ID is empty, a new UUID will be generated
//     If TimeStamp is zero, current time will be used
//
// Returns:
//   - string: The ID of the created trace
//   - error: Any error that occurred during creation
func (l *langfuseIns) CreateTrace(body *TraceEventBody) (string, error) {
	if len(body.ID) == 0 {
		body.ID = uuid.NewString()
	}
	if body.TimeStamp.IsZero() {
		body.TimeStamp = time.Now()
	}
	return body.ID, l.tm.push(&event{
		ID:   uuid.NewString(),
		Type: EventTypeTraceCreate,
		Body: eventBodyUnion{Trace: body},
	})
}

// CreateSpan creates a new span within a trace
//
// Parameters:
//   - body: The span event details. If ID is empty, a new UUID will be generated
//
// Returns:
//   - string: The ID of the created span
//   - error: Any error that occurred during creation
func (l *langfuseIns) CreateSpan(body *SpanEventBody) (string, error) {
	if len(body.ID) == 0 {
		body.ID = uuid.NewString()
	}
	return body.ID, l.tm.push(&event{
		ID:   uuid.NewString(),
		Type: EventTypeSpanCreate,
		Body: eventBodyUnion{Span: body},
	})
}

// EndSpan marks an existing span as completed
//
// Parameters:
//   - body: The span event details to update
//
// Returns:
//   - error: Any error that occurred during the update
func (l *langfuseIns) EndSpan(body *SpanEventBody) error {
	return l.tm.push(&event{
		ID:   uuid.NewString(),
		Type: EventTypeSpanUpdate,
		Body: eventBodyUnion{Span: body},
	})
}

// CreateGeneration creates a new generation event
//
// Parameters:
//   - body: The generation event details. If ID is empty, a new UUID will be generated
//
// Returns:
//   - string: The ID of the created generation
//   - error: Any error that occurred during creation
func (l *langfuseIns) CreateGeneration(body *GenerationEventBody) (string, error) {
	if len(body.ID) == 0 {
		body.ID = uuid.NewString()
	}
	return body.ID, l.tm.push(&event{
		ID:   uuid.NewString(),
		Type: EventTypeGenerationCreate,
		Body: eventBodyUnion{Generation: body},
	})
}

// EndGeneration marks an existing generation as completed
//
// Parameters:
//   - body: The generation event details to update. If ID is empty, a new UUID will be generated
//
// Returns:
//   - error: Any error that occurred during the update
func (l *langfuseIns) EndGeneration(body *GenerationEventBody) error {
	if len(body.ID) == 0 {
		body.ID = uuid.NewString()
	}
	return l.tm.push(&event{
		ID:   uuid.NewString(),
		Type: EventTypeGenerationUpdate,
		Body: eventBodyUnion{Generation: body},
	})
}

// CreateEvent creates a new custom event
//
// Parameters:
//   - body: The event details. If ID is empty, a new UUID will be generated
//
// Returns:
//   - string: The ID of the created event
//   - error: Any error that occurred during creation
func (l *langfuseIns) CreateEvent(body *EventEventBody) (string, error) {
	if len(body.ID) == 0 {
		body.ID = uuid.NewString()
	}
	return body.ID, l.tm.push(&event{
		ID:   uuid.NewString(),
		Type: EventTypeEventCreate,
		Body: eventBodyUnion{Event: body},
	})
}

// Flush waits for all queued events to be processed and uploaded to Langfuse
//
// This method blocks until all pending events in the queue have been processed
// and uploaded. It's recommended to call Flush:
//   - Before program exit
//   - Before shutting down the service
//   - When you need to ensure all events have been successfully uploaded
func (l *langfuseIns) Flush() {
	l.tm.flush()
}

// Prompt provides access to the prompt client for managing prompts.
// Use the returned promptClient to create, retrieve and list prompts.
func (l *langfuseIns) Prompt() *promptClient {
	return l.promptCli
}
