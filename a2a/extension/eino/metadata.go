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
	metadataKeyOfEnableStreaming = "_a2a_eino_adk_enable_streaming"
	metadataKeyOfInterrupted     = "_a2a_eino_adk_interrupted"
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
