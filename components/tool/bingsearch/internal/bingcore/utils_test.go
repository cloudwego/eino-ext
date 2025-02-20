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

package bingcore

import (
	"reflect"
	"testing"
)

var response = []byte(`
{
  "_type": "SearchResponse",
  "queryContext": {
    "originalQuery": "microsoft edge"
  },
  "webPages": {
    "webSearchUrl": "https://ww.bing.com/search?q=microsoft+edge",
    "totalEstimatedMatches": 203000000,
    "value": [
      {
        "name": "The Better Web Browser for Windows...",
        "url": "https://ww.microsoft.com/en-us/...",
        "isFamilyFriendly": true,
        "displayUrl": "https://ww.microsoft.com/en-us/w...",
        "snippet": "Microsoft Edge, now available on ios...",
        "language": "",
        "isNavigational": true
      },
      {
        "name": "Microsoft Edge",
		"url": "https://ww.microsoft.com/en-us/...",
		"isFamilyFriendly": true,
		"displayUrl": "https://ww.microsoft.com/en-us/w...",
		"snippet": "Microsoft Edge, now available on ios...",
		"language": "",
		"isNavigational": true
      }
    ]
  },
  "computation": {
    "id": "https://api.bing.microsoft.com/api/v7/#Computation",
    "expression": "21000 / 8",
    "value": "2625"
  },
  "relatedSearches": {
    "value": [
      {
        "id": "https://api.bing.microsoft.com/api/v7/#RelatedSearches",
        "text": "microsoft edge new download for windows 10",
        "displayText": "microsoft edge new download for...",
        "webSearchUrl": "https://ww.bing.com/search?q=micr..."
      },
      {
        "text": "download microsoft edge window 10",
        "displayText": "download microsoft edge window 10",
        "webSearchUrl": "https://ww.bing.com/search?q=down..."
      }
    ]
  }
}
`)

func Test_parseSearchResponse(t *testing.T) {
	type args struct {
		body []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []*searchResult
		wantErr bool
	}{
		{
			name: "Test_parseSearchResponse_Base",
			args: args{
				body: response,
			},
			want: []*searchResult{
				{
					Title:       "The Better Web Browser for Windows...",
					URL:         "https://ww.microsoft.com/en-us/...",
					Description: "Microsoft Edge, now available on ios...",
				},
				{
					Title:       "Microsoft Edge",
					URL:         "https://ww.microsoft.com/en-us/...",
					Description: "Microsoft Edge, now available on ios...",
				},
			},
			wantErr: false,
		}, {
			name: "Test_parseSearchResponse_JSON_Error",
			args: args{
				body: []byte(`"error": "error"}`),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSearchResponse(tt.args.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSearchResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSearchResponse() got = %v, want %v", got, tt.want)
			}
		})
	}
}
