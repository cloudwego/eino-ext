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

package eino

const (
	metadataKeyOfEnableStreaming  = "_a2a_eino_adk_enable_streaming"
	metadataKeyOfInterrupted      = "_a2a_eino_adk_interrupted"
	metadataKeyOfStreamChunkFinal = "_a2a_eino_adk_stream_chunk_final"
)

func setEnableStreaming(metadata map[string]any) {
	metadata[metadataKeyOfEnableStreaming] = true
}

func getEnableStreaming(metadata map[string]any) bool {
	b, ok := metadata[metadataKeyOfEnableStreaming]
	return ok && b == true
}

func setInterrupted(metadata map[string]any) {
	metadata[metadataKeyOfInterrupted] = true
}

func getInterrupted(metadata map[string]any) bool {
	b, ok := metadata[metadataKeyOfInterrupted]
	return ok && b == true
}

// setStreamChunkFinal marks the message as a chunk of a streaming output.
// final=true marks the last chunk of the stream; final=false marks an intermediate chunk.
func setStreamChunkFinal(metadata map[string]any, final bool) {
	metadata[metadataKeyOfStreamChunkFinal] = final
}

// getStreamChunkFinal reports whether the message is a streaming chunk and, if so, whether it's the final chunk.
func getStreamChunkFinal(metadata map[string]any) (isStreamChunk bool, final bool) {
	v, ok := metadata[metadataKeyOfStreamChunkFinal]
	if !ok {
		return false, false
	}
	b, _ := v.(bool)
	return true, b
}
