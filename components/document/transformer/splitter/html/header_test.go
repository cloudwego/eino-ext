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

package html

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

var commonSuccessHTML = `<!DOCTYPE html>
<html>
<body>
    <div>
        <h1>H1</h1>
        <p>H1 content1</p>
        <div>
            <h2>H2.1</h2>
            <p>H2.1 content</p>
            <h3>H3.1</h3>
            <p>H3.1 content</p>
            <h3>H3.2</h3>
            <p>H3.2 content</p>
            <h2>H2.2</h2>
            <p>H2.2 content</p>
        </div>
        <div>
            <h2>H2.3</h2>
            <p>H2.3 content</p>
        </div>
		<div>
			<p>H1 content2</p>
		</div>
        <br>
        <p>H1 content3</p>
    </div>
	<div>
		<h2>H2.4</h2>
		<p>H2.4 content</p>
	</div>
	<div>
		<p>content</p>
	</div>
</body>
</html>`

func TestHTMLHeaderSplitter(t *testing.T) {
	tests := []struct {
		name   string
		config *HeaderConfig
		input  []*schema.Document
		want   []*schema.Document
	}{
		{
			name: "success",
			config: &HeaderConfig{
				Headers: map[string]string{
					"h1": "Header1",
					"h2": "Header2",
					"h3": "Header3",
				},
				IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
					return fmt.Sprintf("%s_part%d", originalID, splitIndex)
				},
			},
			input: []*schema.Document{{
				ID:       "id",
				Content:  commonSuccessHTML,
				MetaData: map[string]interface{}{},
			}},
			want: []*schema.Document{{
				ID:      "id_part0",
				Content: "H1 content1",
				MetaData: map[string]interface{}{
					"Header1": "H1",
				},
			}, {
				ID:      "id_part1",
				Content: "H2.1 content",
				MetaData: map[string]interface{}{
					"Header1": "H1",
					"Header2": "H2.1",
				},
			}, {
				ID:      "id_part2",
				Content: "H3.1 content",
				MetaData: map[string]interface{}{
					"Header1": "H1",
					"Header2": "H2.1",
					"Header3": "H3.1",
				},
			}, {
				ID:      "id_part3",
				Content: "H3.2 content",
				MetaData: map[string]interface{}{
					"Header1": "H1",
					"Header2": "H2.1",
					"Header3": "H3.2",
				},
			}, {
				ID:      "id_part4",
				Content: "H2.2 content",
				MetaData: map[string]interface{}{
					"Header1": "H1",
					"Header2": "H2.2",
				},
			}, {
				ID:      "id_part5",
				Content: "H2.3 content",
				MetaData: map[string]interface{}{
					"Header1": "H1",
					"Header2": "H2.3",
				},
			}, {
				ID:      "id_part6",
				Content: "H1 content2H1 content3",
				MetaData: map[string]interface{}{
					"Header1": "H1",
				},
			}, {
				ID:      "id_part7",
				Content: "H2.4 content",
				MetaData: map[string]interface{}{
					"Header2": "H2.4",
				},
			}, {
				ID:       "id_part8",
				Content:  "content",
				MetaData: map[string]interface{}{},
			},
			},
		},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			splitter, err := NewHeaderSplitter(ctx, tt.config)
			if err != nil {
				t.Fatal(err)
			}
			ret, err := splitter.Transform(ctx, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ret, tt.want) {
				t.Errorf("NewHeaderSplitter() got = %v, want %v", ret, tt.want)
			}
		})
	}
}
