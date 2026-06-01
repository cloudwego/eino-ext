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

package cozeloop

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetThoughtSignatureFromExtra(t *testing.T) {
	tests := []struct {
		name      string
		extra     map[string]any
		wantSig   []byte
		wantExist bool
	}{
		{
			name:      "nil extra",
			extra:     nil,
			wantSig:   nil,
			wantExist: false,
		},
		{
			name:      "empty extra",
			extra:     map[string]any{},
			wantSig:   nil,
			wantExist: false,
		},
		{
			name: "signature exists as []byte",
			extra: map[string]any{
				thoughtSignatureKey: []byte("test_sig"),
			},
			wantSig:   []byte("test_sig"),
			wantExist: true,
		},
		{
			name: "signature exists as empty []byte",
			extra: map[string]any{
				thoughtSignatureKey: []byte{},
			},
			wantSig:   nil,
			wantExist: true,
		},
		{
			name: "signature exists as string (valid base64)",
			extra: map[string]any{
				thoughtSignatureKey: base64.StdEncoding.EncodeToString([]byte("test_sig")),
			},
			wantSig:   []byte("test_sig"),
			wantExist: true,
		},
		{
			name: "signature exists as empty string",
			extra: map[string]any{
				thoughtSignatureKey: "",
			},
			wantSig:   nil,
			wantExist: true,
		},
		{
			name: "signature exists as invalid base64 string",
			extra: map[string]any{
				thoughtSignatureKey: "invalid_base64!",
			},
			wantSig:   nil,
			wantExist: true,
		},
		{
			name: "signature exists as string (decoded to empty []byte)",
			extra: map[string]any{
				thoughtSignatureKey: base64.StdEncoding.EncodeToString([]byte{}),
			},
			wantSig:   nil,
			wantExist: true,
		},
		{
			name: "signature exists as other type",
			extra: map[string]any{
				thoughtSignatureKey: 123,
			},
			wantSig:   nil,
			wantExist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSig, gotExist := GetThoughtSignatureFromExtra(tt.extra)
			assert.Equal(t, tt.wantExist, gotExist)
			assert.Equal(t, tt.wantSig, gotSig)
		})
	}
}

func TestGetBase64ThoughtSignatureFromExtra(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  string
	}{
		{
			name:  "no signature",
			extra: nil,
			want:  "",
		},
		{
			name: "signature as []byte",
			extra: map[string]any{
				thoughtSignatureKey: []byte("test_sig"),
			},
			want: base64.StdEncoding.EncodeToString([]byte("test_sig")),
		},
		{
			name: "signature as string",
			extra: map[string]any{
				thoughtSignatureKey: base64.StdEncoding.EncodeToString([]byte("test_sig")),
			},
			want: base64.StdEncoding.EncodeToString([]byte("test_sig")),
		},
		{
			name: "nil signature from GetThoughtSignatureFromExtra",
			extra: map[string]any{
				thoughtSignatureKey: 123,
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBase64ThoughtSignatureFromExtra(tt.extra)
			assert.Equal(t, tt.want, got)
		})
	}
}
