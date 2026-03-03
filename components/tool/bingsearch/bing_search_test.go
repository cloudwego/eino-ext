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

package bingsearch

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_validate(t *testing.T) {
	type fields struct {
		ToolName   string
		ToolDesc   string
		APIKey     string
		Region     Region
		MaxResults int
		SafeSearch SafeSearch
		TimeRange  TimeRange
		Headers    map[string]string
		Timeout    time.Duration
		ProxyURL   string
		Cache      time.Duration
		MaxRetries int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "TestConfig_Validate_Vase",
			fields: fields{
				ToolName:   "TestConfig_validate",
				ToolDesc:   "test config validate",
				APIKey:     "api_key_to_validate",
				Region:     RegionUS,
				MaxResults: 0,
				SafeSearch: "",
				TimeRange:  "",
				Headers:    nil,
				Timeout:    0,
				ProxyURL:   "",
				Cache:      0,
				MaxRetries: 0,
			},
			wantErr: false,
		},
		{
			name: "TestConfig_Validate_Missing_API_Key",
			fields: fields{
				ToolName:   "TestConfig_validate",
				ToolDesc:   "test config validate",
				APIKey:     "",
				Region:     RegionUS,
				MaxResults: 10,
				SafeSearch: SafeSearchModerate,
				TimeRange:  "",
				Headers:    nil,
				Timeout:    0,
				ProxyURL:   "",
				Cache:      0,
				MaxRetries: 0,
			},
			wantErr: true,
		},
		{
			name: "TestConfig_Validate_Max_Results_Exceed",
			fields: fields{
				ToolName:   "TestConfig_validate",
				ToolDesc:   "test config validate",
				APIKey:     "api_key_to_validate",
				Region:     RegionUS,
				MaxResults: 100,
				SafeSearch: SafeSearchModerate,
				TimeRange:  "",
				Headers:    nil,
				Timeout:    0,
				ProxyURL:   "",
				Cache:      0,
				MaxRetries: 0,
			},
			wantErr: false,
		},
		{
			name: "TestConfig_Validate_Default_Values",
			fields: fields{
				ToolName:   "",
				ToolDesc:   "",
				APIKey:     "api_key_to_validate",
				Region:     "",
				MaxResults: 0,
				SafeSearch: "",
				TimeRange:  "",
				Headers:    nil,
				Timeout:    0,
				ProxyURL:   "",
				Cache:      0,
				MaxRetries: 0,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				ToolName:   tt.fields.ToolName,
				ToolDesc:   tt.fields.ToolDesc,
				APIKey:     tt.fields.APIKey,
				Region:     tt.fields.Region,
				MaxResults: tt.fields.MaxResults,
				SafeSearch: tt.fields.SafeSearch,
				TimeRange:  tt.fields.TimeRange,
				Headers:    tt.fields.Headers,
				Timeout:    tt.fields.Timeout,
				ProxyURL:   tt.fields.ProxyURL,
				Cache:      tt.fields.Cache,
				MaxRetries: tt.fields.MaxRetries,
			}
			if err := c.validate(); (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewTool(t *testing.T) {
	type args struct {
		ctx    context.Context
		config *Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "TestNewTool_Base",
			args: args{
				ctx: context.Background(),
				config: &Config{
					APIKey: "api_key_to_validate",
				},
			},
			wantErr: false,
		},
		{
			name: "TestNewTool_Missing_API_Key",
			args: args{
				ctx:    context.Background(),
				config: &Config{},
			},
			wantErr: true,
		},
		{
			name: "TestNewTool_Config_Is_Nil",
			args: args{
				ctx:    context.Background(),
				config: nil,
			},
			wantErr: true,
		},
		{
			name: "TestNewTool_BingConfig_Proxy_Url",
			args: args{
				ctx: context.Background(),
				config: &Config{
					APIKey:   "api_key_to_test",
					ProxyURL: "http://localhost:9878",
				},
			},
			wantErr: false,
		},
		{
			name: "TestNewTool_BingConfig_Proxy_Url_not_Supported",
			args: args{
				ctx: context.Background(),
				config: &Config{
					APIKey:   "api_key_to_validate",
					ProxyURL: "ftp://proxy.examples.com",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTool(tt.args.ctx, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got == nil) != tt.wantErr {
				t.Errorf("NewTool() got = %v, want not nil", got)
			}

			if tt.wantErr {
				return
			}

			info, err := got.Info(tt.args.ctx)
			assert.Nil(t, err)

			doc, err := info.ParamsOneOf.ToJSONSchema()
			assert.Nil(t, err)
			assert.Equal(t, 2, doc.Properties.Len())
			for pair := doc.Properties.Oldest(); pair != nil; pair = pair.Next() {
				assert.NotEqual(t, "", pair.Value.Description)
			}
		})
	}
}

func Test_bingSearch_Search(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *SearchRequest
	}
	tests := []struct {
		name         string
		fields       *Config
		args         args
		wantResponse *SearchResponse
		wantErr      bool
	}{
		{
			name: "Test_bingSearch_Missing_Query",
			fields: &Config{
				APIKey: "api_key_to_test",
			},
			args: args{
				ctx: context.Background(),
				request: &SearchRequest{
					Query: "",
				},
			},
			wantResponse: nil,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newBingSearch(tt.fields)
			if err != nil {
				t.Errorf("failed to create bing search tool: %t", err)
			}
			gotResponse, err := s.Search(tt.args.ctx, tt.args.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResponse, tt.wantResponse) {
				t.Errorf("Search() gotResponse = %v, want %v", gotResponse, tt.wantResponse)
				return
			}
		})
	}
}
