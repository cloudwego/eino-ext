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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventBodyUnion(t *testing.T) {
	traceID := "traceID"
	observationID := "observationID"
	metadata := map[string]string{"key": "value"}
	input := "input"
	output := "output"

	traceUnion := &eventBodyUnion{Trace: &TraceEventBody{
		BaseEventBody: BaseEventBody{
			ID:       traceID,
			MetaData: metadata,
		},
		Input:  input,
		Output: output,
	}}
	assert.Equal(t, traceID, traceUnion.getTraceID())
	assert.Equal(t, output, traceUnion.getOutput())
	assert.Equal(t, input, traceUnion.getInput())
	assert.Equal(t, "", traceUnion.getObservationID())
	assert.Equal(t, map[string]string{"key": "value"}, traceUnion.getMetadata())

	spanUnion := &eventBodyUnion{Span: &SpanEventBody{
		BaseObservationEventBody: BaseObservationEventBody{
			BaseEventBody: BaseEventBody{
				ID:       observationID,
				MetaData: metadata,
			},
			TraceID: traceID,
			Input:   input,
			Output:  output,
		},
	}}
	assert.Equal(t, traceID, spanUnion.getTraceID())
	assert.Equal(t, output, spanUnion.getOutput())
	assert.Equal(t, input, spanUnion.getInput())
	assert.Equal(t, observationID, spanUnion.getObservationID())
	assert.Equal(t, map[string]string{"key": "value"}, spanUnion.getMetadata())

	eventUnion := &eventBodyUnion{Event: &EventEventBody{
		BaseObservationEventBody: BaseObservationEventBody{
			BaseEventBody: BaseEventBody{
				ID:       observationID,
				MetaData: metadata,
			},
			TraceID: traceID,
			Input:   input,
			Output:  output,
		},
	}}
	assert.Equal(t, traceID, eventUnion.getTraceID())
	assert.Equal(t, output, eventUnion.getOutput())
	assert.Equal(t, input, eventUnion.getInput())
	assert.Equal(t, observationID, eventUnion.getObservationID())
	assert.Equal(t, map[string]string{"key": "value"}, eventUnion.getMetadata())

	generationUnion := &eventBodyUnion{Generation: &GenerationEventBody{
		BaseObservationEventBody: BaseObservationEventBody{
			BaseEventBody: BaseEventBody{
				ID:       observationID,
				MetaData: metadata,
			},
			TraceID: traceID,
			Input:   input,
			Output:  output,
		},
	}}
	assert.Equal(t, traceID, generationUnion.getTraceID())
	assert.Equal(t, output, generationUnion.getOutput())
	assert.Equal(t, input, generationUnion.getInput())
	assert.Equal(t, observationID, generationUnion.getObservationID())
	assert.Equal(t, map[string]string{"key": "value"}, generationUnion.getMetadata())
}
