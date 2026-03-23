package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

type fakeReader struct {
	msgs   []Message
	index  int
	closed bool
}

func (f *fakeReader) Read(ctx context.Context) (Message, error) {
	if f.index >= len(f.msgs) {
		return nil, io.EOF
	}
	msg := f.msgs[f.index]
	f.index++
	return msg, nil
}

func (f *fakeReader) Close() error {
	f.closed = true
	return nil
}

type fakeWriter struct {
	written []Message
	closed  bool
}

func (f *fakeWriter) WriteStreaming(ctx context.Context, msg Message) error {
	f.written = append(f.written, msg)
	return nil
}

func (f *fakeWriter) Close() error {
	f.closed = true
	return nil
}

func (f *fakeWriter) Write(ctx context.Context, msg Message) error {
	f.written = append(f.written, msg)
	return nil
}

type dynamicRounder struct {
	last Message
}

func (d *dynamicRounder) Round(ctx context.Context, msg Message) (MessageReader, error) {
	d.last = msg
	if req, ok := msg.(*Request); ok {
		resp, _ := NewResponse(req.ID, map[string]string{"ok": "yes"})
		return &fakeReader{msgs: []Message{resp}}, nil
	}
	return &fakeReader{}, nil
}

func TestParseID(t *testing.T) {
	id, err := ParseID("a")
	if err != nil || id.String() != "a" {
		t.Fatalf("unexpected id: %v %v", id.String(), err)
	}
	id, err = ParseID(float64(3))
	if err != nil || id.String() != "3" {
		t.Fatalf("unexpected id: %v %v", id.String(), err)
	}
	id, err = ParseID(nil)
	if err != nil || !id.IsNil() {
		t.Fatalf("unexpected id: %v %v", id.String(), err)
	}
	_, err = ParseID(true)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMessageEncodeDecode(t *testing.T) {
	req, err := NewRequest("m", NewIDFromString("1"), map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buf, err := EncodeMessage(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msgs, isBatch, err := DecodeMessages(buf)
	if err != nil || isBatch || len(msgs) != 1 {
		t.Fatalf("unexpected decode: %v %v %d", err, isBatch, len(msgs))
	}
	if msgs[0].Type() != ObjectTypeRequest {
		t.Fatalf("unexpected type: %v", msgs[0].Type())
	}
}

func TestDecodeMessagesBatch(t *testing.T) {
	req, _ := NewRequest("m", NewIDFromString("1"), map[string]string{"k": "v"})
	notif, _ := NewNotification("n", map[string]string{"k": "v"})
	buf, _ := json.Marshal([]Message{req, notif})
	msgs, isBatch, err := DecodeMessages(buf)
	if err != nil || !isBatch || len(msgs) != 2 {
		t.Fatalf("unexpected decode: %v %v %d", err, isBatch, len(msgs))
	}
	if msgs[1].Type() != ObjectTypeNotification {
		t.Fatalf("unexpected type: %v", msgs[1].Type())
	}
}

func TestNewConnectionCallNotify(t *testing.T) {
	rounder := &dynamicRounder{}
	_, conn, err := NewConnection(context.Background(), WithClientRounder(rounder), WithJSONRPCIDGenerator(func(ctx context.Context) ID {
		return NewIDFromString("1")
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var resp map[string]string
	if err := conn.Call(context.Background(), "m", map[string]string{"k": "v"}, &resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["ok"] != "yes" {
		t.Fatalf("unexpected resp: %v", resp)
	}
	if err := conn.Notify(context.Background(), "n", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rounder.last.Type() != ObjectTypeNotification {
		t.Fatalf("unexpected type: %v", rounder.last.Type())
	}
}

func TestNewConnectionEmpty(t *testing.T) {
	_, _, err := NewConnection(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRpcCallRecvNonResponse(t *testing.T) {
	reader := &fakeReader{msgs: []Message{&Notification{Version: version, Method: "n"}}}
	conn := &connection{outbounds: make(map[string]*rpcCall)}
	call := &rpcCall{id: NewIDFromString("1"), reader: reader, conn: conn}
	if err := call.Recv(context.Background(), &struct{}{}); err == nil {
		t.Fatalf("expected error")
	}
	if !reader.closed {
		t.Fatalf("expected reader closed")
	}
}

func TestRpcCallFinish(t *testing.T) {
	writer := &fakeWriter{}
	conn := &connection{outbounds: make(map[string]*rpcCall)}
	call := &rpcCall{id: NewIDFromString("1"), writer: writer, conn: conn}
	if err := call.Send(context.Background(), map[string]string{"k": "v"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := call.Finish(context.Background(), NewError(InternalErrorCode, "boom", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !writer.closed {
		t.Fatalf("expected closed")
	}
}

func TestCallChainOrder(t *testing.T) {
	order := ""
	mw1 := func(next CallEndpoint) CallEndpoint {
		return func(ctx context.Context, method string, req, resp interface{}) error {
			order += "1"
			return next(ctx, method, req, resp)
		}
	}
	mw2 := func(next CallEndpoint) CallEndpoint {
		return func(ctx context.Context, method string, req, resp interface{}) error {
			order += "2"
			return next(ctx, method, req, resp)
		}
	}
	end := callChain(mw1, mw2)(func(ctx context.Context, method string, req, resp interface{}) error {
		order += "3"
		return nil
	})
	if err := end(context.Background(), "m", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order != "123" {
		t.Fatalf("unexpected order: %s", order)
	}
}

func TestConvertError(t *testing.T) {
	base := errors.New("boom")
	err := ConvertError(base)
	if err.Code != InternalErrorCode {
		t.Fatalf("unexpected code: %v", err.Code)
	}
}
