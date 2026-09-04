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

import "testing"

func TestEnableStreamingMetadata(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		if got := getEnableStreaming(map[string]any{}); got {
			t.Errorf("absent key: got %v, want false", got)
		}
		if got := getEnableStreaming(nil); got {
			t.Errorf("nil map: got %v, want false", got)
		}
	})
	t.Run("set true", func(t *testing.T) {
		md := map[string]any{}
		setEnableStreaming(md)
		if got := getEnableStreaming(md); !got {
			t.Errorf("after set: got %v, want true", got)
		}
	})
	t.Run("non-bool value", func(t *testing.T) {
		md := map[string]any{metadataKeyOfEnableStreaming: "true"}
		if got := getEnableStreaming(md); got {
			t.Errorf("string \"true\" should not match: got %v, want false", got)
		}
	})
	t.Run("explicit false value", func(t *testing.T) {
		md := map[string]any{metadataKeyOfEnableStreaming: false}
		if got := getEnableStreaming(md); got {
			t.Errorf("explicit false: got %v, want false", got)
		}
	})
}

func TestInterruptedMetadata(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		if got := getInterrupted(map[string]any{}); got {
			t.Errorf("absent: got %v, want false", got)
		}
	})
	t.Run("set true", func(t *testing.T) {
		md := map[string]any{}
		setInterrupted(md)
		if !getInterrupted(md) {
			t.Errorf("after set: got false, want true")
		}
	})
}

func TestStreamChunkFinalMetadata(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		isChunk, final := getStreamChunkFinal(map[string]any{})
		if isChunk || final {
			t.Errorf("absent: got isChunk=%v final=%v, want both false", isChunk, final)
		}
		isChunk, final = getStreamChunkFinal(nil)
		if isChunk || final {
			t.Errorf("nil map: got isChunk=%v final=%v, want both false", isChunk, final)
		}
	})
	t.Run("non-final chunk", func(t *testing.T) {
		md := map[string]any{}
		setStreamChunkFinal(md, false)
		isChunk, final := getStreamChunkFinal(md)
		if !isChunk || final {
			t.Errorf("non-final: got isChunk=%v final=%v, want isChunk=true final=false", isChunk, final)
		}
	})
	t.Run("final chunk", func(t *testing.T) {
		md := map[string]any{}
		setStreamChunkFinal(md, true)
		isChunk, final := getStreamChunkFinal(md)
		if !isChunk || !final {
			t.Errorf("final: got isChunk=%v final=%v, want both true", isChunk, final)
		}
	})
	t.Run("non-bool value still counts as chunk", func(t *testing.T) {
		md := map[string]any{metadataKeyOfStreamChunkFinal: "yes"}
		isChunk, final := getStreamChunkFinal(md)
		if !isChunk || final {
			t.Errorf("string value: got isChunk=%v final=%v, want isChunk=true final=false", isChunk, final)
		}
	})
}
