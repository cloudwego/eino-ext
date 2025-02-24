package model

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type mockModel struct {
	tb testing.TB

	generateFunc func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)
}

type Option func(*mockModel)

func WithGenerateFunc(f func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)) Option {
	return func(m *mockModel) {
		m.generateFunc = f
	}
}

var _ model.ChatModel = (*mockModel)(nil)

func NewMockModel(tb testing.TB, opts ...Option) model.ChatModel {
	mm := &mockModel{tb: tb}
	for _, opt := range opts {
		opt(mm)
	}
	return mm
}

func (m *mockModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Config:   &model.Config{}, // todo: fill in the config
	})

	if m.generateFunc != nil {
		return m.generateFunc(ctx, input, opts...)
	}
	return nil, nil
}

func (m *mockModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m *mockModel) BindTools(tools []*schema.ToolInfo) error {
	return nil
}
