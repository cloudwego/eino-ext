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
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

type batchIngestionRequest struct {
	Batch    []*event          `json:"batch,omitempty"`
	MetaData map[string]string `json:"metadata,omitempty"`
}

type batchIngestError struct {
	ID      string `json:"id"`
	Status  int    `json:"status"`
	Message string `json:"message,omitempty"`
	Error   any    `json:"error,omitempty"`
}
type batchIngestionResponse struct {
	Success []*struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
	} `json:"success"`
	Errors []*batchIngestError `json:"errors"`
}

type LevelType string

const (
	LevelTypeDEBUG   LevelType = "DEBUG"
	LevelTypeDEFAULT LevelType = "DEFAULT"
	LevelTypeWARNING LevelType = "WARNING"
	LevelTypeERROR   LevelType = "ERROR"
)

type EventType string

const (
	EventTypeTraceCreate      EventType = "trace-create"
	EventTypeSpanCreate       EventType = "span-create"
	EventTypeSpanUpdate       EventType = "span-update"
	EventTypeGenerationCreate EventType = "generation-create"
	EventTypeGenerationUpdate EventType = "generation-update"
	EventTypeEventCreate      EventType = "event-create"

	EventTypeScoreCreate EventType = "score-create"
	EventTypeSDKLog      EventType = "sdk-log"
)

type event struct {
	ID        string            `json:"id"`
	Type      EventType         `json:"type"`
	TimeStamp time.Time         `json:"timestamp"`
	MetaData  map[string]string `json:"metadata"`

	Body eventBodyUnion `json:"body"`
}
type eventBodyUnion struct {
	Trace      *TraceEventBody      `json:",inline,omitempty"`
	Span       *SpanEventBody       `json:",inline,omitempty"`
	Generation *GenerationEventBody `json:",inline,omitempty"`
	Event      *EventEventBody      `json:",inline,omitempty"`
	Log        *SDKLogEventBody     `json:",inline,omitempty"`
}

func (e *eventBodyUnion) MarshalJSON() ([]byte, error) {
	if e.Trace != nil {
		return sonic.Marshal(e.Trace)
	} else if e.Span != nil {
		return sonic.Marshal(e.Span)
	} else if e.Generation != nil {
		return sonic.Marshal(e.Generation)
	} else if e.Event != nil {
		return sonic.Marshal(e.Event)
	}
	return nil, fmt.Errorf("event body is empty")
}

func (e *eventBodyUnion) getTraceID() string {
	if e.Trace != nil {
		return e.Trace.ID
	} else if e.Span != nil {
		return e.Span.TraceID
	} else if e.Generation != nil {
		return e.Generation.TraceID
	} else if e.Event != nil {
		return e.Event.TraceID
	}
	return ""
}

func (e *eventBodyUnion) getObservationID() string {
	if e.Span != nil {
		return e.Span.ID
	} else if e.Generation != nil {
		return e.Generation.ID
	} else if e.Event != nil {
		return e.Event.ID
	}
	return ""
}

func (e *eventBodyUnion) getInput() string {
	if e.Trace != nil {
		return e.Trace.Input
	} else if e.Span != nil {
		return e.Span.Input
	} else if e.Generation != nil {
		return e.Generation.Input
	} else if e.Event != nil {
		return e.Event.Input
	}
	return ""
}
func (e *eventBodyUnion) setInput(in string) {
	if e.Trace != nil {
		e.Trace.Input = in
	} else if e.Span != nil {
		e.Span.Input = in
	} else if e.Generation != nil {
		e.Generation.Input = in
	} else if e.Event != nil {
		e.Event.Input = in
	}
	return
}
func (e *eventBodyUnion) getOutput() string {
	if e.Trace != nil {
		return e.Trace.Output
	} else if e.Span != nil {
		return e.Span.Output
	} else if e.Generation != nil {
		return e.Generation.Output
	} else if e.Event != nil {
		return e.Event.Output
	}
	return ""
}
func (e *eventBodyUnion) setOutput(out string) {
	if e.Trace != nil {
		e.Trace.Output = out
	} else if e.Span != nil {
		e.Span.Output = out
	} else if e.Generation != nil {
		e.Generation.Output = out
	} else if e.Event != nil {
		e.Event.Output = out
	}
	return
}
func (e *eventBodyUnion) getMetadata() any {
	if e.Trace != nil {
		return e.Trace.MetaData
	} else if e.Span != nil {
		return e.Span.MetaData
	} else if e.Generation != nil {
		return e.Generation.MetaData
	} else if e.Event != nil {
		return e.Event.MetaData
	}
	return nil
}
func (e *eventBodyUnion) setMetadata(data any) {
	if e.Trace != nil {
		e.Trace.MetaData = data
	} else if e.Span != nil {
		e.Span.MetaData = data
	} else if e.Generation != nil {
		e.Generation.MetaData = data
	} else if e.Event != nil {
		e.Event.MetaData = data
	}
	return
}

type BaseEventBody struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	MetaData    any    `json:"metadata,omitempty"`
	Version     string `json:"version,omitempty"`
	Environment string `json:"environment,omitempty"`
}

type TraceEventBody struct {
	BaseEventBody
	TimeStamp time.Time `json:"timestamp,omitempty"`
	UserID    string    `json:"userId,omitempty"`
	Input     string    `json:"input,omitempty"`
	Output    string    `json:"output,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`
	Release   string    `json:"release,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Public    bool      `json:"public,omitempty"`
}

type BaseObservationEventBody struct {
	BaseEventBody
	TraceID             string    `json:"traceId,omitempty"`
	Input               string    `json:"input,omitempty"`
	Output              string    `json:"output,omitempty"`
	StatusMessage       string    `json:"statusMessage,omitempty"`
	ParentObservationID string    `json:"parentObservationId,omitempty"`
	Level               LevelType `json:"level,omitempty"`
	StartTime           time.Time `json:"startTime,omitempty"`
}

type SpanEventBody struct {
	BaseObservationEventBody

	EndTime time.Time `json:"endTime,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"promptTokens,omitempty"`
	CompletionTokens int `json:"completionTokens,omitempty"`
	TotalTokens      int `json:"totalTokens,omitempty"`
}

type GenerationEventBody struct {
	BaseObservationEventBody

	InMessages          []*schema.Message `json:"-"`
	OutMessage          *schema.Message   `json:"-"`
	EndTime             time.Time         `json:"endTime,omitempty"`
	CompletionStartTime time.Time         `json:"completionStartTime,omitempty"`
	Model               string            `json:"model,omitempty"`
	PromptName          string            `json:"promptName,omitempty"`
	PromptVersion       int               `json:"promptVersion,omitempty"`
	ModelParameters     any               `json:"modelParameters,omitempty"`
	Usage               *Usage            `json:"usage,omitempty"`
}

type EventEventBody struct {
	BaseObservationEventBody
}

type SDKLogEventBody struct {
	Log string `json:"log"`
}

// TODO: ScoreEvent
