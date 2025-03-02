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
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestBingClient_Search(t *testing.T) {
	type args struct {
		ctx    context.Context
		params *SearchParams
	}
	tests := []struct {
		name    string
		fields  *Config
		args    args
		want    []*searchResult
		wantErr bool
	}{
		{
			name: "TestBingClient_Search_Params_Missing",
			fields: &Config{
				Headers: map[string]string{
					"Ocp-Apim-Subscription-Key": "api_key_to_test",
				},
			},
			args: args{
				ctx:    context.Background(),
				params: nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "TestBingClient_Search_Params_Query_Missing",
			fields: &Config{
				Headers: map[string]string{
					"Ocp-Apim-Subscription-Key": "api_key_to_test",
				},
			},
			args: args{
				ctx: context.Background(),
				params: &SearchParams{
					Query: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "TestBingClient_Search_Params_Query_Valid",
			fields: &Config{
				Headers: map[string]string{
					"Ocp-Apim-Subscription-Key": "api_key_to_test",
				},
			},
			args: args{
				ctx: context.Background(),
				params: &SearchParams{
					Query: "Test",
					Count: 10,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "TestBingClient_Search_Params_Query_Valid_With_Cache",
			fields: &Config{
				Headers: map[string]string{
					"Ocp-Apim-Subscription-Key": "api_key_to_test",
				},
				Cache: 5 * time.Minute,
			},
			args: args{
				ctx: context.Background(),
				params: &SearchParams{
					Query: "Test",
					Count: 10,
				},
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(tt.fields)
			if err != nil {
				t.Errorf("New() error = %v", err)
				return
			}
			if tt.fields.Cache > 0 {
				if err := tt.args.params.validate(); err != nil {
					return
				}
				cacheKey := tt.args.params.getCacheKey()
				b.cache.Set(cacheKey, []*searchResult{
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
				})
			}
			got, err := b.Search(tt.args.ctx, tt.args.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Search() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBingClient_sendRequestWithRetry(t *testing.T) {
	type fields struct {
		client  *http.Client
		baseURL string
		headers map[string]string
		timeout time.Duration
		config  *Config
	}
	type args struct {
		ctx    context.Context
		req    *http.Request
		params *SearchParams
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*searchResult
		wantErr bool
	}{
		{
			name: "TestBingClient_sendRequestWithRetry_Base",
			fields: fields{
				client: &http.Client{},
				config: &Config{
					Headers: map[string]string{
						"Ocp-Apim-Subscription-Key": "api_key_to_test",
					},
					MaxRetries: 0,
				},
			},
			args: args{
				ctx: context.Background(),
				req: &http.Request{
					Header: http.Header{},
				},
				params: &SearchParams{
					Query: "Test",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "TestBingClient_sendRequestWithRetry_Max_Retries",
			fields: fields{
				client: &http.Client{Timeout: 10 * time.Second},
				config: &Config{
					Headers:    make(map[string]string),
					Timeout:    10 * time.Second,
					MaxRetries: 3,
				},
			},
			args: args{
				ctx: context.Background(),
				req: &http.Request{
					Header: http.Header{},
				},
				params: &SearchParams{
					Query: "Test",
					Count: 10,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{

			name: "TestBingClient_sendRequestWithRetry_Context_Cancelled",
			fields: fields{
				client: &http.Client{Timeout: 10 * time.Second},
				config: &Config{
					Headers:    make(map[string]string),
					Timeout:    10 * time.Second,
					MaxRetries: 3,
				},
			},
			args: args{
				ctx: context.Background(),
				req: &http.Request{
					Header: http.Header{},
				},
				params: &SearchParams{
					Query: "Test",
					Count: 10,
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BingClient{
				client:  tt.fields.client,
				baseURL: tt.fields.baseURL,
				headers: tt.fields.headers,
				timeout: tt.fields.timeout,
				config:  tt.fields.config,
			}
			if tt.name == "TestBingClient_sendRequestWithRetry_Context_Cancelled" {
				ctx, cancel := context.WithTimeout(tt.args.ctx, 1*time.Second)
				cancel()
				tt.args.ctx = ctx
			}
			got, err := b.sendRequestWithRetry(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("sendRequestWithRetry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sendRequestWithRetry() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	type args struct {
		config *Config
	}
	tests := []struct {
		name    string
		args    args
		want    *BingClient
		wantErr bool
	}{
		{
			name: "TestNew_Base",
			args: args{
				config: &Config{
					Headers: map[string]string{
						"Ocp-Apim-Subscription-Key": "api_key_to_test",
					},
				},
			},
			want: &BingClient{
				client:  &http.Client{Timeout: 30 * time.Second},
				baseURL: "https://api.bing.microsoft.com/v7.0/search",
				headers: map[string]string{
					"Ocp-Apim-Subscription-Key": "api_key_to_test",
				},
				timeout: 30 * time.Second,
				config: &Config{
					Headers: map[string]string{
						"Ocp-Apim-Subscription-Key": "api_key_to_test",
					},
					Timeout:    30 * time.Second,
					MaxRetries: 3,
				},
			},
			wantErr: false,
		},
		{
			name: "TestNew_Invalid_Proxy",
			args: args{
				config: &Config{
					Headers: map[string]string{
						"Ocp-Apim-Subscription-Key": "api_key_to_test",
					},
					ProxyURL: "invalid_proxy",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() got = %v, want %v", got, tt.want)
			}
		})
	}
}
