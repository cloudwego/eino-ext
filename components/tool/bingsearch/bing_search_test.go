package bingsearch

import (
	"context"
	"errors"
	"fmt"
	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch/bingcore"
	"reflect"
	"testing"
)

func MockBingClient() *mockey.Mocker {
	return mockey.Mock((*bingcore.BingClient).Search).To(func(ctx context.Context, params *bingcore.SearchParams) ([]*bingcore.SearchResult, error) {
		if params == nil {
			return nil, errors.New("params is nil")
		}

		if params.Query == "" {
			return nil, fmt.Errorf("search query cannot be empty")
		}

		if params.Count <= 0 {
			return nil, fmt.Errorf("search count must be greater than 0")
		}

		if params.Offset < 0 {
			return nil, fmt.Errorf("search offset must be greater than or equal to 0")
		}

		return []*bingcore.SearchResult{
			{
				Title:       "test title",
				Description: "test description",
				URL:         "test link",
			},
			{
				Title:       "test title 2",
				Description: "test description 2",
				URL:         "test link 2",
			},
		}, nil
	}).Build()
}

func TestConfig_validate(t *testing.T) {
	type fields struct {
		ToolName   string
		ToolDesc   string
		APIKey     string
		Region     bingcore.Region
		MaxResults int
		SafeSearch bingcore.SafeSearch
		TimeRange  bingcore.TimeRange
		BingConfig *bingcore.Config
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
				Region:     bingcore.RegionUS,
				MaxResults: 0,
				SafeSearch: "",
				TimeRange:  "",
				BingConfig: nil,
			},
			wantErr: false,
		},
		{
			name: "TestConfig_Validate_Missing_API_Key",
			fields: fields{
				ToolName:   "TestConfig_validate",
				ToolDesc:   "test config validate",
				APIKey:     "",
				Region:     bingcore.RegionUS,
				MaxResults: 10,
				SafeSearch: bingcore.SafeSearchModerate,
				TimeRange:  "",
				BingConfig: nil,
			},
			wantErr: true,
		},
		{
			name: "TestConfig_Validate_Max_Results_Exceed",
			fields: fields{
				ToolName:   "TestConfig_validate",
				ToolDesc:   "test config validate",
				APIKey:     "api_key_to_validate",
				Region:     bingcore.RegionUS,
				MaxResults: 100,
				SafeSearch: bingcore.SafeSearchModerate,
				TimeRange:  "",
				BingConfig: nil,
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
				BingConfig: nil,
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
				BingConfig: tt.fields.BingConfig,
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
					APIKey: "api_key_to_test",
					BingConfig: &bingcore.Config{
						ProxyURL: "http://localhost:9878",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "TestNewTool_BingConfig_Proxy_Url_not_Supported",
			args: args{
				ctx: context.Background(),
				config: &Config{
					APIKey: "api_key_to_validate",
					BingConfig: &bingcore.Config{
						ProxyURL: "ftp://proxy.example.com",
					},
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
			name: "Test_bingSearch_Search_Base",
			fields: &Config{
				APIKey: "api_key_to_test",
			},
			args: args{
				ctx: nil,
				request: &SearchRequest{
					Query:  "test",
					Offset: 0,
				},
			},
			wantResponse: &SearchResponse{
				Results: []*SearchResult{
					{
						Title:       "test title",
						Description: "test description",
						URL:         "test link",
					},
					{
						Title:       "test title 2",
						Description: "test description 2",
						URL:         "test link 2",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Test_bingSearch_Max_Results",
			fields: &Config{
				APIKey:     "api_key_to_test",
				MaxResults: 1,
			},
			args: args{
				ctx: context.Background(),
				request: &SearchRequest{
					Query:  "test",
					Offset: 0,
				},
			},
			wantResponse: &SearchResponse{
				Results: []*SearchResult{
					{
						Title:       "test title",
						Description: "test description",
						URL:         "test link",
					},
				},
			},
			wantErr: false,
		},
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
		{
			name: "Test_bingSearch_With_Cache",
			fields: &Config{
				APIKey: "api_key_to_test",
				BingConfig: &bingcore.Config{
					Cache:      true,
					MaxRetries: 3,
				},
			},
			args: args{
				ctx: context.Background(),
				request: &SearchRequest{
					Query: "test",
				},
			},
			wantResponse: &SearchResponse{
				Results: []*SearchResult{
					{
						Title:       "test title",
						Description: "test description",
						URL:         "test link",
					},
					{
						Title:       "test title 2",
						Description: "test description 2",
						URL:         "test link 2",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newBingSearch(tt.fields)
			if err != nil {
				t.Errorf("failed to create bing search tool: %t", err)
			}
			mock := MockBingClient()
			defer mock.UnPatch()
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
