package jsonrpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
)

type fakeClientAsync struct {
	payload json.RawMessage
	err     error
	closed  bool
}

func (f *fakeClientAsync) Recv(ctx context.Context, obj interface{}) error {
	if f.err != nil {
		return f.err
	}
	raw := obj.(*json.RawMessage)
	*raw = f.payload
	return nil
}

func (f *fakeClientAsync) Close() error {
	f.closed = true
	return nil
}

type fakeConn struct {
	callMethod  string
	asyncMethod string
	callResp    json.RawMessage
	async       core.ClientAsync
}

func (f *fakeConn) Call(ctx context.Context, method string, req, res interface{}, opts ...core.CallOption) error {
	f.callMethod = method
	if res != nil {
		if raw, ok := res.(*json.RawMessage); ok {
			*raw = f.callResp
		}
	}
	return nil
}

func (f *fakeConn) AsyncCall(ctx context.Context, method string, req interface{}, opts ...core.CallOption) (core.ClientAsync, error) {
	f.asyncMethod = method
	return f.async, nil
}

func (f *fakeConn) Notify(ctx context.Context, method string, params interface{}, opts ...core.CallOption) error {
	return nil
}

func TestExtractSendMessageStreamingResponseUnion(t *testing.T) {
	msg := &models.Message{Role: models.RoleAgent}
	buf, _ := json.Marshal(struct {
		*models.Message
		Kind models.ResponseKind `json:"kind"`
	}{
		Message: msg,
		Kind:    models.ResponseKindMessage,
	})
	res, err := extractSendMessageStreamingResponseUnion(buf)
	if err != nil || res.Message == nil {
		t.Fatalf("unexpected: %v %v", res, err)
	}
	_, err = extractSendMessageStreamingResponseUnion([]byte(`{"kind":"unknown"}`))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestFrameReader(t *testing.T) {
	payload, _ := json.Marshal(struct {
		*models.Message
		Kind models.ResponseKind `json:"kind"`
	}{
		Message: &models.Message{Role: models.RoleAgent},
		Kind:    models.ResponseKindMessage,
	})
	async := &fakeClientAsync{payload: payload}
	reader := &frameReader{a: async}
	resp, err := reader.Read()
	if err != nil || resp.Message == nil {
		t.Fatalf("unexpected: %v %v", resp, err)
	}
	async.err = io.EOF
	_, err = reader.Read()
	if err != io.EOF {
		t.Fatalf("expected eof")
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !async.closed {
		t.Fatalf("expected closed")
	}
}

func TestTransportAgentCard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"n","description":"d","protocolVersion":"0.2.5","url":"u","defaultInputModes":[],"defaultOutputModes":[],"skills":[]}`))
	}))
	defer srv.Close()
	tr := &Transport{
		agentCardURL: srv.URL,
		cli:          srv.Client(),
	}
	card, err := tr.AgentCard(context.Background())
	if err != nil || card.Name != "n" {
		t.Fatalf("unexpected card: %v %v", card, err)
	}
}

func TestTransportSendMessage(t *testing.T) {
	msg := &models.Message{Role: models.RoleAgent}
	buf, _ := json.Marshal(struct {
		*models.Message
		Kind models.ResponseKind `json:"kind"`
	}{
		Message: msg,
		Kind:    models.ResponseKindMessage,
	})
	conn := &fakeConn{callResp: buf, async: &fakeClientAsync{payload: buf}}
	tr := &Transport{conn: conn}
	resp, err := tr.SendMessage(context.Background(), &models.MessageSendParams{})
	if err != nil || resp.Message == nil {
		t.Fatalf("unexpected: %v %v", resp, err)
	}
	if conn.callMethod != "message/send" {
		t.Fatalf("unexpected method: %s", conn.callMethod)
	}
	stream, err := tr.SendMessageStreaming(context.Background(), &models.MessageSendParams{})
	if err != nil || stream == nil {
		t.Fatalf("unexpected stream: %v %v", stream, err)
	}
	if conn.asyncMethod != "message/stream" {
		t.Fatalf("unexpected method: %s", conn.asyncMethod)
	}
}
