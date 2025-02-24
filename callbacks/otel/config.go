package otel

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type config struct {
	scopeName string
	tp        trace.TracerProvider
	mp        metric.MeterProvider
}

type Option interface {
	apply(*config)
}

type optionFunc func(*config)

func (f optionFunc) apply(o *config) {
	f(o)
}

func newOptions(opts ...Option) *config {
	o := &config{
		scopeName: scopeName,
		tp:        otel.GetTracerProvider(),
	}
	for _, opt := range opts {
		opt.apply(o)
	}
	return o
}

func WithScopeName(name string) Option {
	return optionFunc(func(o *config) {
		o.scopeName = name
	})
}

func WithTracerProvider(tp trace.TracerProvider) Option {
	return optionFunc(func(o *config) {
		o.tp = tp
	})
}

func WithMeterProvider(mp metric.MeterProvider) Option {
	return optionFunc(func(o *config) {
		o.mp = mp
	})
}
