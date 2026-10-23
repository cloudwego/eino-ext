/*
 * Copyright 2026 CloudWeGo Authors
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

package ark

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const modifierChunk = `{"id":"chunk","object":"chat.completion.chunk","created":1,"model":"model","choices":[{"index":0,"delta":{"role":"assistant","content":"done"},"finish_reason":null}],"usage":null,"custom_value":"original"}`

func modifyJSON(raw []byte, edit func(map[string]json.RawMessage)) ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	edit(body)
	return json.Marshal(body)
}

func TestRequestPayloadModifierWire(t *testing.T) {
	for _, mode := range []string{"generate", "stream", "batch"} {
		t.Run(mode, func(t *testing.T) {
			var calls int
			config := &ChatModelConfig{Model: "original", Temperature: ptr(float32(0.7))}
			if mode == "batch" {
				config.BatchChat = &BatchChatConfig{EnableBatchChat: true, BatchChatAsyncRetryTimeout: time.Second}
			}
			config.RequestPayloadModifier = func(ctx context.Context, in []*schema.Message, raw []byte) ([]byte, error) {
				calls++
				equal(t, "input", in[0].Content)
				var original map[string]json.RawMessage
				check(t, json.Unmarshal(raw, &original))
				equal(t, `0.7`, string(original["temperature"]))
				return modifyJSON(raw, func(body map[string]json.RawMessage) {
					delete(body, "temperature")
					body["model"] = json.RawMessage(`"modified-model"`)
					body["custom_param"] = json.RawMessage(`{"integer":9007199254740993}`)
					body["messages"] = json.RawMessage(`[{"role":"user","content":[{"type":"future_part","value":"x"}]}]`)
					body["stream"] = json.RawMessage(`false`)
					if mode == "stream" {
						body["stream_options"] = json.RawMessage(`{"include_usage":false,"chunk_include_usage":true}`)
					}
				})
			}
			cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
				var body map[string]json.RawMessage
				check(t, json.NewDecoder(r.Body).Decode(&body))
				if _, ok := body["temperature"]; ok {
					t.Error("deleted field was restored by SDK")
				}
				equal(t, `"modified-model"`, string(body["model"]))
				equal(t, `{"integer":9007199254740993}`, string(body["custom_param"]))
				equal(t, `[{"role":"user","content":[{"type":"future_part","value":"x"}]}]`, string(body["messages"]))
				if mode == "batch" {
					equal(t, "/api/v3/batch/chat/completions", r.URL.Path)
				}
				if mode == "stream" {
					equal(t, `true`, string(body["stream"]))
					var streamOptions map[string]bool
					check(t, json.Unmarshal(body["stream_options"], &streamOptions))
					equal(t, map[string]bool{"chunk_include_usage": true, "include_usage": true}, streamOptions)
					w.Header().Set("Content-Type", "text/event-stream")
					sseChunk(w, modifierChunk)
					sseChunk(w, "[DONE]")
				} else {
					equal(t, `false`, string(body["stream"]))
					writeResponse(w)
				}
			}, config)
			input := []*schema.Message{{Role: schema.User, Content: "input"}}
			if mode == "stream" {
				sr, err := cm.Stream(context.Background(), input)
				check(t, err)
				defer sr.Close()
				_, err = sr.Recv()
				check(t, err)
				_, err = sr.Recv()
				equal(t, io.EOF, err)
			} else {
				_, err := cm.Generate(context.Background(), input)
				check(t, err)
			}
			equal(t, 1, calls)
		})
	}
}

func TestRequestPayloadModifierOnceBeforeRetries(t *testing.T) {
	var requests, modifications atomic.Int32
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequest(t, r)
		equal(t, "value", body["custom_param"])
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":{"code":"ServiceUnavailable","message":"retry","type":"server_error"}}`)
			return
		}
		writeResponse(w)
	}, &ChatModelConfig{Model: "model", RequestPayloadModifier: func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
		modifications.Add(1)
		return modifyJSON(raw, func(body map[string]json.RawMessage) { body["custom_param"] = json.RawMessage(`"value"`) })
	}})
	// The test helper disables retries, so construct a model with retries enabled.
	config := cm.config
	config.RetryTimes = ptr(1)
	cm, err := NewChatModel(context.Background(), &config)
	check(t, err)
	_, err = cm.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "input"}})
	check(t, err)
	equal(t, int32(2), requests.Load())
	equal(t, int32(1), modifications.Load())
}

func TestRequestPayloadModifierValidation(t *testing.T) {
	sentinel := errors.New("payload modifier failed")
	cases := []struct {
		name string
		body string
		err  error
	}{
		{"error", "", sentinel}, {"empty", "", nil}, {"null", "null", nil}, {"array", "[]", nil},
		{"trailing data", `{"model":"m","messages":[]} trailing`, nil},
		{"truncated object", `{"model":"m","messages":[`, nil},
		{"missing fields", "{}", nil}, {"model type", `{"model":7,"messages":[]}`, nil},
		{"null model", `{"model":null,"messages":[]}`, nil}, {"missing messages", `{"model":"m"}`, nil},
		{"multiple completions", `{"model":"m","messages":[],"n":2}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reported error
			cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("invalid modifier sent an HTTP request")
				writeResponse(w)
			}, nil)
			h := callbacks.NewHandlerBuilder().OnErrorFn(func(ctx context.Context, _ *callbacks.RunInfo, err error) context.Context { reported = err; return ctx }).Build()
			ctx := callbacks.InitCallbacks(context.Background(), &callbacks.RunInfo{}, h)
			_, err := cm.Generate(ctx, nil, WithRequestPayloadModifier(func(context.Context, []*schema.Message, []byte) ([]byte, error) { return []byte(tc.body), tc.err }))
			if err == nil || reported == nil {
				t.Fatalf("error=%v callback=%v", err, reported)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("modifier cause lost: %v", err)
			}
		})
	}
	t.Run("invalid stream options", func(t *testing.T) {
		cm := testModel(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid options sent to server") }, nil)
		_, err := cm.Stream(context.Background(), nil, WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
			return modifyJSON(raw, func(body map[string]json.RawMessage) { body["stream_options"] = json.RawMessage(`[]`) })
		}))
		if err == nil {
			t.Fatal("expected stream options error")
		}
	})
}

func TestResponseMessageModifierAndCallbacks(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "generate", true: "batch"}[batch], func(t *testing.T) {
			wire := strings.TrimSuffix(responseJSON, "}") + `,"unknown_field":"kept"}`
			var observed *fmodel.CallbackOutput
			config := &ChatModelConfig{Model: "model"}
			if batch {
				config.BatchChat = &BatchChatConfig{EnableBatchChat: true, BatchChatAsyncRetryTimeout: time.Second}
			}
			config.ResponseMessageModifier = func(ctx context.Context, msg *schema.Message, raw []byte) (*schema.Message, error) {
				equal(t, "done", msg.Content)
				if batch {
					equal(t, []byte(nil), raw)
				} else {
					equal(t, wire, string(raw))
				}
				msg.Content = "modified"
				msg.ResponseMeta.Usage.TotalTokens = 99
				return msg, nil
			}
			cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, wire)
			}, config)
			h := callbacks.NewHandlerBuilder().OnEndFn(func(ctx context.Context, _ *callbacks.RunInfo, out callbacks.CallbackOutput) context.Context {
				observed = out.(*fmodel.CallbackOutput)
				return ctx
			}).Build()
			ctx := callbacks.InitCallbacks(context.Background(), &callbacks.RunInfo{}, h)
			msg, err := cm.Generate(ctx, nil)
			check(t, err)
			equal(t, "modified", msg.Content)
			if observed == nil {
				t.Fatal("missing callback")
			}
			equal(t, msg, observed.Message)
			equal(t, 99, observed.TokenUsage.TotalTokens)
		})
	}
}

func TestResponseMessageModifierErrors(t *testing.T) {
	sentinel := errors.New("response modifier failed")
	for _, failure := range []error{sentinel, nil} {
		var reported error
		cm := testModel(t, func(w http.ResponseWriter, r *http.Request) { writeResponse(w) }, nil)
		h := callbacks.NewHandlerBuilder().OnErrorFn(func(ctx context.Context, _ *callbacks.RunInfo, err error) context.Context { reported = err; return ctx }).Build()
		ctx := callbacks.InitCallbacks(context.Background(), &callbacks.RunInfo{}, h)
		_, err := cm.Generate(ctx, nil, WithResponseMessageModifier(func(context.Context, *schema.Message, []byte) (*schema.Message, error) { return nil, failure }))
		if err == nil || reported == nil {
			t.Fatalf("error=%v callback=%v", err, reported)
		}
		if failure != nil && !errors.Is(err, failure) {
			t.Fatal("response modifier cause lost")
		}
	}
}

func TestResponseChunkMessageModifier(t *testing.T) {
	payloads := []string{
		`{"id":"c","object":"chat.completion.chunk","created":1,"model":"m","choices":[],"usage":null,"custom_value":"first"}`,
		strings.Replace(modifierChunk, `"custom_value":"original"`, `"custom_value":"skip"`, 1),
		strings.Replace(modifierChunk, `"custom_value":"original"`, `"custom_value":"last"`, 1),
	}
	var calls, ends int
	var retained [][]byte
	type key struct{}
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, payload := range payloads {
			sseChunk(w, payload)
		}
		sseChunk(w, "[DONE]")
	}, nil)
	ctx := context.WithValue(context.Background(), key{}, "context")
	sr, err := cm.Stream(ctx, nil, WithResponseChunkMessageModifier(func(ctx context.Context, msg *schema.Message, raw []byte, end bool) (*schema.Message, error) {
		equal(t, "context", ctx.Value(key{}))
		calls++
		if end {
			ends++
			equal(t, (*schema.Message)(nil), msg)
			equal(t, []byte(nil), raw)
			return &schema.Message{Content: "!"}, nil
		}
		retained = append(retained, raw)
		var custom struct {
			Value string `json:"custom_value"`
		}
		if err := json.Unmarshal(raw, &custom); err != nil {
			return nil, err
		}
		switch custom.Value {
		case "first":
			equal(t, (*schema.Message)(nil), msg)
			return &schema.Message{Role: schema.Assistant, Content: "first"}, nil
		case "skip":
			return nil, nil
		default:
			msg.Content = "last"
			return msg, nil
		}
	}))
	check(t, err)
	defer sr.Close()
	var chunks []*schema.Message
	for {
		msg, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		check(t, err)
		chunks = append(chunks, msg)
	}
	equal(t, 4, calls)
	equal(t, 1, ends)
	equal(t, 3, len(chunks))
	msg, err := schema.ConcatMessages(chunks)
	check(t, err)
	equal(t, "firstlast!", msg.Content)
	for i, payload := range payloads {
		equal(t, payload, string(retained[i]))
	}
}

func TestResponseChunkModifierErrors(t *testing.T) {
	sentinel := errors.New("chunk modifier failed")
	for _, atEnd := range []bool{false, true} {
		t.Run(map[bool]string{false: "chunk", true: "end"}[atEnd], func(t *testing.T) {
			cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				sseChunk(w, modifierChunk)
				sseChunk(w, "[DONE]")
			}, nil)
			sr, err := cm.Stream(context.Background(), nil, WithResponseChunkMessageModifier(func(_ context.Context, msg *schema.Message, _ []byte, end bool) (*schema.Message, error) {
				if end == atEnd {
					return nil, sentinel
				}
				return msg, nil
			}))
			check(t, err)
			defer sr.Close()
			for {
				_, err = sr.Recv()
				if err != nil {
					break
				}
			}
			if !errors.Is(err, sentinel) {
				t.Fatalf("modifier error lost: %v", err)
			}
		})
	}
}

func TestModifiersOverrideDisableAndConcurrency(t *testing.T) {
	var requests atomic.Int32
	config := &ChatModelConfig{Model: "model",
		RequestPayloadModifier: func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
			return modifyJSON(raw, func(body map[string]json.RawMessage) { body["custom_param"] = json.RawMessage(`"configured"`) })
		},
		ResponseMessageModifier: func(_ context.Context, msg *schema.Message, _ []byte) (*schema.Message, error) {
			msg.Content = "configured"
			return msg, nil
		},
	}
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequest(t, r)
		if _, ok := body["custom_param"]; ok {
			t.Error("configured payload modifier was not replaced")
		}
		requests.Add(1)
		writeResponse(w)
	}, config)
	bound, err := cm.WithTools([]*schema.ToolInfo{toolInfo("inspect")})
	check(t, err)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg, err := bound.Generate(context.Background(), nil,
				WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
					var body map[string]json.RawMessage
					if err := json.Unmarshal(raw, &body); err != nil {
						return nil, err
					}
					// Mutating JSON must not mutate bound tool schemas or another call's data.
					var tools []map[string]json.RawMessage
					if err := json.Unmarshal(body["tools"], &tools); err != nil {
						return nil, err
					}
					var function map[string]json.RawMessage
					if err := json.Unmarshal(tools[0]["function"], &function); err != nil {
						return nil, err
					}
					var parameters map[string]json.RawMessage
					if err := json.Unmarshal(function["parameters"], &parameters); err != nil {
						return nil, err
					}
					if _, exists := parameters["additionalProperties"]; exists {
						t.Error("bound tool schema was mutated")
					}
					parameters["additionalProperties"] = json.RawMessage(`false`)
					function["parameters"], _ = json.Marshal(parameters)
					tools[0]["function"], _ = json.Marshal(function)
					body["tools"], _ = json.Marshal(tools)
					body["custom_call"] = json.RawMessage(`true`)
					return json.Marshal(body)
				}), WithResponseMessageModifier(func(_ context.Context, msg *schema.Message, _ []byte) (*schema.Message, error) {
					msg.Content = "call"
					return msg, nil
				}))
			if err != nil {
				t.Error(err)
				return
			}
			equal(t, "call", msg.Content)
		}()
	}
	wg.Wait()
	msg, err := cm.Generate(context.Background(), nil, WithRequestPayloadModifier(nil), WithResponseMessageModifier(nil))
	check(t, err)
	equal(t, "done", msg.Content)
	equal(t, int32(9), requests.Load())
}

func TestResponseCapturePreservesHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeResponse(w) }))
	defer server.Close()
	client := server.Client()
	client.Timeout = time.Second
	originalTransport := client.Transport
	cm, err := NewChatModel(context.Background(), &ChatModelConfig{Model: "m", APIKey: "key", BaseURL: server.URL, HTTPClient: client, Timeout: ptr(2 * time.Second)})
	check(t, err)
	_, err = cm.Generate(context.Background(), nil, WithResponseMessageModifier(func(_ context.Context, msg *schema.Message, raw []byte) (*schema.Message, error) {
		equal(t, responseJSON, string(raw))
		return msg, nil
	}))
	check(t, err)
	equal(t, time.Second, client.Timeout)
	equal(t, originalTransport, client.Transport)
	if cm.client.HTTPClient() == client {
		t.Fatal("caller-owned client was not copied")
	}
}

func TestResponseModifierCanStartStream(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	defer finish()
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequest(t, r)
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			sseChunk(w, modifierChunk)
			w.(http.Flusher).Flush()
			select {
			case <-release:
				sseChunk(w, "[DONE]")
			case <-r.Context().Done():
			}
			return
		}
		writeResponse(w)
	}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := cm.Generate(ctx, nil, WithResponseMessageModifier(func(ctx context.Context, message *schema.Message, raw []byte) (*schema.Message, error) {
		sr, err := cm.Stream(ctx, nil)
		if err != nil {
			finish()
			return nil, err
		}
		defer sr.Close()
		chunk, err := sr.Recv()
		if err != nil {
			finish()
			return nil, err
		}
		equal(t, "done", chunk.Content)
		finish()
		_, err = sr.Recv()
		if !errors.Is(err, io.EOF) {
			return nil, err
		}
		equal(t, responseJSON, string(raw))
		return message, nil
	}))
	check(t, err)
}

func TestDisableConfiguredChunkModifier(t *testing.T) {
	var calls atomic.Int32
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseChunk(w, modifierChunk)
		sseChunk(w, "[DONE]")
	}, &ChatModelConfig{ResponseChunkMessageModifier: func(_ context.Context, msg *schema.Message, _ []byte, _ bool) (*schema.Message, error) {
		calls.Add(1)
		return msg, nil
	}})
	sr, err := cm.Stream(context.Background(), nil, WithResponseChunkMessageModifier(nil))
	check(t, err)
	defer sr.Close()
	msg, err := sr.Recv()
	check(t, err)
	equal(t, "done", msg.Content)
	_, err = sr.Recv()
	equal(t, io.EOF, err)
	equal(t, int32(0), calls.Load())
}
