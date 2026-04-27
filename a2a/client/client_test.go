package client

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino-ext/a2a/models"
)

type fakeResponseReader struct {
	resp  *models.SendMessageStreamingResponseUnion
	err   error
	close bool
}

func (f *fakeResponseReader) Read() (*models.SendMessageStreamingResponseUnion, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func (f *fakeResponseReader) Close() error {
	f.close = true
	return nil
}

type fakeTransport struct {
	checkedParts bool
	reader       *fakeResponseReader
}

func (f *fakeTransport) AgentCard(ctx context.Context) (*models.AgentCard, error) {
	return &models.AgentCard{Name: "n", Description: "d"}, nil
}

func (f *fakeTransport) SendMessage(ctx context.Context, params *models.MessageSendParams) (*models.SendMessageResponseUnion, error) {
	if params.Message.Parts != nil {
		f.checkedParts = true
	}
	return &models.SendMessageResponseUnion{Message: &models.Message{Role: models.RoleAgent}}, nil
}

func (f *fakeTransport) SendMessageStreaming(ctx context.Context, params *models.MessageSendParams) (models.ResponseReader, error) {
	return f.reader, nil
}

func (f *fakeTransport) GetTask(ctx context.Context, params *models.TaskQueryParams) (*models.Task, error) {
	return &models.Task{ID: "t"}, nil
}

func (f *fakeTransport) CancelTask(ctx context.Context, params *models.TaskIDParams) (*models.Task, error) {
	return &models.Task{ID: "t"}, nil
}

func (f *fakeTransport) ResubscribeTask(ctx context.Context, params *models.TaskIDParams) (models.ResponseReader, error) {
	return f.reader, nil
}

func (f *fakeTransport) SetPushNotificationConfig(ctx context.Context, params *models.TaskPushNotificationConfig) (*models.TaskPushNotificationConfig, error) {
	return params, nil
}

func (f *fakeTransport) GetPushNotificationConfig(ctx context.Context, params *models.GetTaskPushNotificationConfigParams) (*models.TaskPushNotificationConfig, error) {
	return &models.TaskPushNotificationConfig{TaskID: params.PushNotificationConfigID}, nil
}

func (f *fakeTransport) Close() error {
	return nil
}

func TestNewClientNilConfig(t *testing.T) {
	if _, err := NewA2AClient(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSendMessageEnsuresFields(t *testing.T) {
	trans := &fakeTransport{}
	cli, err := NewA2AClient(context.Background(), &Config{Transport: trans})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = cli.SendMessage(context.Background(), &models.MessageSendParams{
		Message: models.Message{Role: models.RoleUser},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !trans.checkedParts {
		t.Fatalf("expected parts set")
	}
}

func TestServerStreamingWrapper(t *testing.T) {
	reader := &fakeResponseReader{err: errors.New("read")}
	trans := &fakeTransport{reader: reader}
	cli, err := NewA2AClient(context.Background(), &Config{Transport: trans})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stream, err := cli.SendMessageStreaming(context.Background(), &models.MessageSendParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = stream.Recv()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !reader.close {
		t.Fatalf("expected close")
	}
}
