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

	"github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

func Test_defaultConfig(t *testing.T) {
	mockey.PatchConvey("Test defaultConfig", t, func() {
		expectedConfig := &config{
			enableTracing: true,
			enableMetrics: true,
			sampler:       sdktrace.AlwaysSample(),
		}
		actualConfig := defaultConfig()
		convey.So(actualConfig.enableTracing, convey.ShouldBeTrue)
		convey.So(actualConfig.enableMetrics, convey.ShouldBeTrue)
		convey.So(actualConfig.sampler, convey.ShouldEqual, expectedConfig.sampler)
	})
}

func Test_WithServiceName(t *testing.T) {
	serviceName := "test-service"

	mockey.PatchConvey("Test WithServiceName", t, func() {
		cfg := &config{
			resourceAttributes: []attribute.KeyValue{},
		}
		opt := WithServiceName(serviceName)
		opt.apply(cfg)

		expectedAttributes := []attribute.KeyValue{
			semconv.ServiceNameKey.String(serviceName),
		}

		convey.So(cfg.resourceAttributes, convey.ShouldResemble, expectedAttributes)
	})
}

func Test_WithDeploymentEnvironment(t *testing.T) {
	mockey.PatchConvey("Test WithDeploymentEnvironment", t, func() {
		cfg := &config{
			resourceAttributes: []attribute.KeyValue{},
		}

		// Test with a valid environment
		mockey.PatchConvey("Test with valid environment", func() {
			env := "production"
			opt := WithDeploymentEnvironment(env)
			opt.apply(cfg)
			expected := []attribute.KeyValue{semconv.DeploymentEnvironmentNameKey.String(env)}
			convey.So(cfg.resourceAttributes, convey.ShouldResemble, expected)
		})

		// Test with an empty environment
		mockey.PatchConvey("Test with empty environment", func() {
			env := ""
			opt := WithDeploymentEnvironment(env)
			opt.apply(cfg)
			expected := []attribute.KeyValue{semconv.DeploymentEnvironmentNameKey.String(env)}
			convey.So(cfg.resourceAttributes, convey.ShouldResemble, expected)
		})
	})
}

func Test_WithServiceNamespace(t *testing.T) {
	namespace := "test-namespace"
	cfg := &config{
		resourceAttributes: []attribute.KeyValue{},
	}

	mockey.PatchConvey("Test WithServiceNamespace", t, func() {
		opt := WithServiceNamespace(namespace)
		opt.apply(cfg)
		expectedAttr := semconv.ServiceNamespaceKey.String(namespace)
		convey.So(cfg.resourceAttributes, convey.ShouldContain, expectedAttr)
	})
}

func Test_WithResourceAttribute(t *testing.T) {
	rAttr := attribute.KeyValue{Key: "test_key", Value: attribute.StringValue("test_value")}

	mockey.PatchConvey("Test WithResourceAttribute", t, func() {
		var cfg config
		cfg.resourceAttributes = []attribute.KeyValue{}

		optionFunc := WithResourceAttribute(rAttr)
		optionFunc.apply(&cfg)

		expectedCfg := config{
			resourceAttributes: []attribute.KeyValue{rAttr},
		}

		convey.So(cfg, convey.ShouldResemble, expectedCfg)
	})
}

func Test_WithResourceAttributes(t *testing.T) {
	mockey.PatchConvey("Test WithResourceAttributes", t, func() {
		rAttrs := []attribute.KeyValue{
			{Key: "key1", Value: attribute.StringValue("value1")},
			{Key: "key2", Value: attribute.Int64Value(123)},
		}
		option := WithResourceAttributes(rAttrs)

		cfg := &config{}
		option.apply(cfg)

		convey.So(cfg.resourceAttributes, convey.ShouldResemble, rAttrs)
	})
}

func Test_WithResource(t *testing.T) {
	r := resource.NewWithAttributes("test-schema-url")

	mockey.PatchConvey("Test WithResource with valid resource", t, func() {
		cfg := &config{}
		opt := WithResource(r)
		opt.apply(cfg)
		convey.So(cfg.resource, convey.ShouldEqual, r)
	})

	mockey.PatchConvey("Test WithResource with nil resource", t, func() {
		cfg := &config{}
		opt := WithResource(nil)
		opt.apply(cfg)
		convey.So(cfg.resource, convey.ShouldBeNil)
	})
}

func Test_WithExportEndpoint(t *testing.T) {
	endpoint := "http://example.com/export"

	mockey.PatchConvey("Test WithExportEndpoint", t, func() {
		cfg := &config{}
		opt := WithExportEndpoint(endpoint)
		opt.apply(cfg)

		convey.So(cfg.exportEndpoint, convey.ShouldEqual, endpoint)
	})
}

func Test_WithEnableTracing(t *testing.T) {
	mockey.PatchConvey("Test WithEnableTracing", t, func() {
		cfg := &config{}

		mockey.PatchConvey("Test enableTracing is true", func() {
			opt := WithEnableTracing(true)
			opt.apply(cfg)
			convey.So(cfg.enableTracing, convey.ShouldBeTrue)
		})

		mockey.PatchConvey("Test enableTracing is false", func() {
			opt := WithEnableTracing(false)
			opt.apply(cfg)
			convey.So(cfg.enableTracing, convey.ShouldBeFalse)
		})
	})
}

func Test_WithEnableMetrics(t *testing.T) {
	mockey.PatchConvey("Test WithEnableMetrics with true", t, func() {
		cfg := &config{}
		opt := WithEnableMetrics(true)
		opt.apply(cfg)
		convey.So(cfg.enableMetrics, convey.ShouldBeTrue)
	})
	mockey.PatchConvey("Test WithEnableMetrics with false", t, func() {
		cfg := &config{}
		opt := WithEnableMetrics(false)
		opt.apply(cfg)
		convey.So(cfg.enableMetrics, convey.ShouldBeFalse)
	})
}

func Test_WithHeaders(t *testing.T) {
	mockey.PatchConvey("Test WithHeaders", t, func() {
		headers := map[string]string{
			"header1": "value1",
			"header2": "value2",
		}
		cfg := &config{
			exportHeaders: map[string]string{},
		}

		opt := WithHeaders(headers)
		opt.apply(cfg)

		convey.So(cfg.exportHeaders, convey.ShouldResemble, headers)
	})

	mockey.PatchConvey("Test WithHeaders with nil headers", t, func() {
		headers := map[string]string{}
		cfg := &config{
			exportHeaders: map[string]string{},
		}

		opt := WithHeaders(headers)
		opt.apply(cfg)

		convey.So(cfg.exportHeaders, convey.ShouldResemble, headers)
	})

	mockey.PatchConvey("Test WithHeaders with empty headers", t, func() {
		headers := map[string]string{
			"header1": "",
			"header2": "",
		}
		cfg := &config{
			exportHeaders: map[string]string{},
		}

		opt := WithHeaders(headers)
		opt.apply(cfg)

		convey.So(cfg.exportHeaders, convey.ShouldResemble, headers)
	})

	mockey.PatchConvey("Test WithHeaders with existing headers", t, func() {
		headers := map[string]string{
			"header1": "value1",
			"header2": "value2",
		}
		cfg := &config{
			exportHeaders: map[string]string{
				"existingHeader": "existingValue",
			},
		}

		opt := WithHeaders(headers)
		opt.apply(cfg)

		expectedHeaders := map[string]string{
			"header1": "value1",
			"header2": "value2",
		}
		convey.So(cfg.exportHeaders, convey.ShouldResemble, expectedHeaders)
	})
}

func Test_WithInsecure(t *testing.T) {
	cfg := &config{}

	mockey.PatchConvey("Test WithInsecure", t, func() {
		option := WithInsecure()
		option.apply(cfg)
		convey.So(cfg.exportInsecure, convey.ShouldBeTrue)
	})
}

func Test_WithSampler(t *testing.T) {
	mockey.PatchConvey("Test WithSampler", t, func() {
		sampler := sdktrace.TraceIDRatioBased(0.5)
		option := WithSampler(sampler)
		cfg := &config{}

		option.apply(cfg)

		convey.So(cfg.sampler, convey.ShouldEqual, sampler)
	})
}

func Test_WithSdkTracerProvider(t *testing.T) {
	mockey.PatchConvey("Test WithSdkTracerProvider with nil provider", t, func() {
		opt := WithSdkTracerProvider(nil)
		cfg := &config{}
		opt.apply(cfg)
		convey.So(cfg.sdkTracerProvider, convey.ShouldBeNil)
	})

	mockey.PatchConvey("Test WithSdkTracerProvider with valid provider", t, func() {
		mockProvider := &sdktrace.TracerProvider{}
		opt := WithSdkTracerProvider(mockProvider)
		cfg := &config{}
		opt.apply(cfg)
		convey.So(cfg.sdkTracerProvider, convey.ShouldEqual, mockProvider)
	})
}

func Test_WithMeterProvider(t *testing.T) {
	meterProvider := &metric.MeterProvider{}

	mockey.PatchConvey("Test WithMeterProvider", t, func() {
		option := WithMeterProvider(meterProvider)
		convey.So(option, convey.ShouldNotBeNil)

		cfg := &config{}
		option.apply(cfg)
		convey.So(cfg.meterProvider, convey.ShouldEqual, meterProvider)
	})
}
