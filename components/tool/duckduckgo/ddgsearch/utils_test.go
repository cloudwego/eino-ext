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

package ddgsearch

import (
	"testing"
)

func TestExtractVQD(t *testing.T) {
	tests := []struct {
		name     string
		html     []byte
		keywords string
		want     string
		wantErr  bool
	}{
		{
			name:     "valid vqd double quotes",
			html:     []byte(`<script>vqd="123456";</script>`),
			keywords: "test",
			want:     "123456",
			wantErr:  false,
		},
		{
			name:     "valid vqd single quotes",
			html:     []byte(`<script>vqd='789012';</script>`),
			keywords: "test",
			want:     "789012",
			wantErr:  false,
		},
		{
			name:     "valid vqd with ampersand",
			html:     []byte(`<script>vqd=345678&other=value</script>`),
			keywords: "test",
			want:     "345678",
			wantErr:  false,
		},
		{
			name:     "no vqd",
			html:     []byte(`<script>no vqd here</script>`),
			keywords: "test",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractVQD(tt.html, tt.keywords)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractVQD() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractVQD() = %v, want %v", got, tt.want)
			}
		})
	}
}
