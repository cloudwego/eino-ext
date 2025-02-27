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

package opentelemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

func Test_newResource(t *testing.T) {
	type args struct {
		cfg *config
	}
	tests := []struct {
		name              string
		args              args
		wantResources     []attribute.KeyValue
		unwantedResources []attribute.KeyValue
	}{
		{
			name: "with conflict schema version",
			args: args{
				cfg: &config{
					resourceAttributes: []attribute.KeyValue{
						semconv.ServiceNameKey.String("test-semconv-resource"),
					},
				},
			},
			wantResources: []attribute.KeyValue{
				semconv.ServiceNameKey.String("test-semconv-resource"),
			},
			unwantedResources: []attribute.KeyValue{
				semconv.ServiceNameKey.String("unknown_service:___Test_newResource_in_github_com_cloudwego_eino_ext_libs_acl_opentelemetry.test"),
			},
		},
		{
			name: "resource override",
			args: args{
				cfg: &config{
					resource: resource.Default(),
					resourceAttributes: []attribute.KeyValue{
						semconv.ServiceNameKey.String("test-resource"),
					},
				},
			},
			wantResources: nil,
			unwantedResources: []attribute.KeyValue{
				semconv.ServiceNameKey.String("test-resource"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newResource(tt.args.cfg)
			for _, res := range tt.wantResources {
				assert.Contains(t, got.Attributes(), res)
			}
			for _, unwantedResource := range tt.unwantedResources {
				assert.NotContains(t, got.Attributes(), unwantedResource)
			}
		})
	}
}
