/*
 * Copyright 2024 CloudWeGo Authors
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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/devops/internal/model"
	"github.com/cloudwego/eino-ext/devops/internal/utils/safego"
)

func Test_NewCallbackOption(t *testing.T) {
	op := newCallbackOption("mock_node", "thread_1", compose.GraphNodeInfo{}, nil)
	assert.NotNil(t, op)
}

func Test_OnStart(t *testing.T) {
	ctx := context.Background()
	info := &callbacks.RunInfo{
		Name:      "TestName",
		Type:      "TestType",
		Component: components.Component("TestComponent"),
	}
	cb := &callbackHandler{
		nodeKey:  "nodeKey",
		threadID: "threadID",
	}
	PatchConvey("Test json marshal error", t, func() {
		cb.stateCh = make(chan *model.NodeDebugState, 1)
		Mock(json.Marshal).Return(nil, errors.New("json marshal error")).Build()
		actualCtx := cb.OnStart(ctx, info, "TestInput")
		safego.Go(ctx, func() {
			res, _ := <-cb.stateCh
			assert.Equal(t, res.ErrorType, model.SystemError)
		})
		assert.Equal(t, actualCtx, ctx)
	})
	PatchConvey("Test successful", t, func() {
		actualCtx := cb.OnStart(ctx, info, "TestInput")
		ctxVal, ctxValOK := getNodeDebugStateCtx(actualCtx)
		assert.True(t, ctxValOK)
		assert.Equal(t, ctxVal.callbackInput, "\"TestInput\"")
	})
	PatchConvey("Test multi-depth successful", t, func() {
		ctx = setNodeDebugStateCtx(ctx, &nodeDebugStateCtxValue{
			depth: 2,
		})
		actualCtx := cb.OnStart(ctx, info, "TestInput")
		ctxVal, ctxValOK := getNodeDebugStateCtx(actualCtx)
		assert.True(t, ctxValOK)
		assert.NotEqual(t, ctxVal.callbackInput, "\"TestInput\"")
	})

}

func Test_OnEnd(t *testing.T) {
	ctx := context.Background()
	info := &callbacks.RunInfo{
		Name:      "TestRun",
		Type:      "TestType",
		Component: components.Component("TestComponent"),
	}

	PatchConvey("Test when ctxValOK is false", t, func() {
		cb := &callbackHandler{
			nodeKey:  "nodeKey",
			threadID: "threadID",
		}
		cb.stateCh = make(chan *model.NodeDebugState, 1)
		Mock(getNodeDebugStateCtx).Return(nil, false).Build()
		actualCtx := cb.OnEnd(ctx, info, "TestOutput")
		safego.Go(ctx, func() {
			res, _ := <-cb.stateCh
			assert.True(t, res.Metrics.InvokeTimeMS == 0)
		})
		assert.Equal(t, actualCtx, ctx)
	})
	PatchConvey("Test when json.Marshal fails", t, func() {
		cb := &callbackHandler{
			nodeKey:  "nodeKey",
			threadID: "threadID",
		}
		cb.stateCh = make(chan *model.NodeDebugState, 1)
		Mock(getNodeDebugStateCtx).Return(&nodeDebugStateCtxValue{invokeTimeMS: int64(1728630000), callbackInput: "input"}, true).Build()
		Mock(json.Marshal).Return(nil, errors.New("json marshal error")).Build()
		actualCtx := cb.OnEnd(ctx, info, "TestOutput")
		safego.Go(ctx, func() {
			res, _ := <-cb.stateCh
			assert.Equal(t, res.ErrorType, model.SystemError)
		})
		assert.Equal(t, actualCtx, ctx)
	})
	PatchConvey("Test normal case", t, func() {
		cb := &callbackHandler{
			nodeKey:  "nodeKey",
			threadID: "threadID",
		}
		cb.stateCh = make(chan *model.NodeDebugState, 1)
		Mock(getNodeDebugStateCtx).Return(&nodeDebugStateCtxValue{invokeTimeMS: int64(1728630000), callbackInput: "input"}, true).Build()
		actualCtx := cb.OnEnd(ctx, info, "TestOutput")
		safego.Go(ctx, func() {
			res, _ := <-cb.stateCh
			assert.Equal(t, res.Output, "\"TestOutput\"")
		})
		assert.Equal(t, actualCtx, ctx)
	})
	PatchConvey("Test multi-depth successful", t, func() {
		cb := &callbackHandler{
			nodeKey:  "nodeKey",
			threadID: "threadID",
		}
		cb.stateCh = make(chan *model.NodeDebugState, 1)
		Mock(getNodeDebugStateCtx).Return(&nodeDebugStateCtxValue{invokeTimeMS: int64(1728630000), callbackInput: "input", depth: 2}, true).Build()
		actualCtx := cb.OnEnd(ctx, info, "TestOutput")
		safego.Go(ctx, func() {
			res, _ := <-cb.stateCh
			assert.NotEqual(t, res.Output, "\"TestOutput\"")
		})
		assert.Equal(t, actualCtx, ctx)
	})
}

func Test_OnError(t *testing.T) {
	ctx := context.Background()
	info := &callbacks.RunInfo{
		Name:      "TestRun",
		Type:      "TestType",
		Component: components.Component("TestComponent"),
	}
	cb := &callbackHandler{
		nodeKey:  "nodeKey",
		threadID: "threadID",
		stateCh:  make(chan *model.NodeDebugState, 100),
	}
	PatchConvey("Test normal case", t, func() {
		Mock(getNodeDebugStateCtx).Return(&nodeDebugStateCtxValue{invokeTimeMS: int64(1728630000), callbackInput: "input"}, true).Build()
		actualCtx := cb.OnError(ctx, info, errors.New("test error"))
		safego.Go(ctx, func() {
			res, _ := <-cb.stateCh
			assert.Equal(t, res.Error, "test error")
		})
		assert.Equal(t, actualCtx, ctx)
	})
}

func Test_OnStartWithStreamInput(t *testing.T) {
	info := &callbacks.RunInfo{
		Name:      "TestRun",
		Type:      "TestType",
		Component: components.Component("TestComponent"),
	}
	cb := &callbackHandler{
		nodeKey:  "nodeKey",
		threadID: "threadID",
	}

	PatchConvey("Test normal case", t, func() {
		r, w := schema.Pipe[callbacks.CallbackInput](1)
		go func() {
			defer w.Close()
			str := "stream"
			for i := 0; i < len(str); i++ {
				if closed := w.Send(str[i:i+1], nil); closed {
					break
				}
			}
		}()

		actualCtx := cb.OnStartWithStreamInput(context.Background(), info, r)
		ctxVal, ctxValOK := getNodeDebugStateCtx(actualCtx)
		assert.True(t, ctxValOK)
		assert.Equal(t, ctxVal.callbackInput, "[\"s\",\"t\",\"r\",\"e\",\"a\",\"m\"]")
	})

	PatchConvey("Test multi-depth normal case", t, func() {
		ctx := context.Background()
		ctx = setNodeDebugStateCtx(ctx, &nodeDebugStateCtxValue{
			depth: 2,
		})
		r, w := schema.Pipe[callbacks.CallbackInput](1)
		go func() {
			defer w.Close()
			str := "stream"
			for i := 0; i < len(str); i++ {
				if closed := w.Send(str[i:i+1], nil); closed {
					break
				}
			}
		}()

		actualCtx := cb.OnStartWithStreamInput(ctx, info, r)
		ctxVal, ctxValOK := getNodeDebugStateCtx(actualCtx)
		assert.True(t, ctxValOK)
		assert.NotEqual(t, ctxVal.callbackInput, "[\"s\",\"t\",\"r\",\"e\",\"a\",\"m\"]")
	})
}

func Test_OnEndWithStreamOutput(t *testing.T) {
	ctx := context.Background()
	info := &callbacks.RunInfo{
		Name:      "TestRun",
		Type:      "TestType",
		Component: components.Component("TestComponent"),
	}
	cb := &callbackHandler{
		nodeKey:  "nodeKey",
		threadID: "threadID",
		stateCh:  make(chan *model.NodeDebugState, 100),
	}

	PatchConvey("Test normal case", t, func() {
		r, w := schema.Pipe[callbacks.CallbackOutput](1)
		go func() {
			defer w.Close()
			str := "stream"
			for i := 0; i < len(str); i++ {
				if closed := w.Send(str[i:i+1], nil); closed {
					break
				}
			}
		}()
		Mock(getNodeDebugStateCtx).Return(&nodeDebugStateCtxValue{invokeTimeMS: int64(1728630000), callbackInput: "input", depth: 2}, true).Build()
		actualCtx := cb.OnEndWithStreamOutput(context.Background(), info, r)
		safego.Go(ctx, func() {
			<-cb.stateCh
		})
		assert.Equal(t, actualCtx, ctx)
	})
}

func Test_convCallbackInput(t *testing.T) {
	// 创建 callbackHandler 实例
	c := &callbackHandler{}

	PatchConvey("Test when input is *einomodel.CallbackInput", t, func() {
		einomodelInput := &einomodel.CallbackInput{Messages: []*schema.Message{}}
		actual := c.convCallbackInput(einomodelInput)
		assert.Equal(t, actual, einomodelInput.Messages)
	})

	PatchConvey("Test when input is *embedding.CallbackInput", t, func() {
		embeddingInput := &embedding.CallbackInput{Texts: []string{}}
		actual := c.convCallbackInput(embeddingInput)
		assert.Equal(t, actual, embeddingInput.Texts)
	})

	PatchConvey("Test when input is *indexer.CallbackInput", t, func() {
		indexerInput := &indexer.CallbackInput{Docs: []*schema.Document{}}
		actual := c.convCallbackInput(indexerInput)
		assert.Equal(t, actual, indexerInput.Docs)
	})

	PatchConvey("Test when input is *prompt.CallbackInput", t, func() {
		promptInput := &prompt.CallbackInput{Variables: map[string]any{"": ""}}
		actual := c.convCallbackInput(promptInput)
		assert.Equal(t, actual, promptInput.Variables)
	})

	PatchConvey("Test when input is *retriever.CallbackInput", t, func() {
		retrieverInput := &retriever.CallbackInput{Query: "retriever query"}
		actual := c.convCallbackInput(retrieverInput)
		assert.Equal(t, actual, retrieverInput.Query)
	})

	PatchConvey("Test when input is of unknown type", t, func() {
		unknownInput := "unknown input"
		actual := c.convCallbackInput(unknownInput)
		assert.Equal(t, actual, unknownInput)
	})
}

func Test_convCallbackOutput(t *testing.T) {
	c := &callbackHandler{}
	PatchConvey("Test when output is of type *einomodel.CallbackOutput", t, func() {
		mockOutput := &einomodel.CallbackOutput{Message: &schema.Message{}}
		actual := c.convCallbackOutput(mockOutput)
		assert.Equal(t, actual, mockOutput.Message)
	})
	PatchConvey("Test when output is of type *embedding.CallbackOutput", t, func() {
		mockOutput := &embedding.CallbackOutput{Embeddings: [][]float64{{1.0, 2.0}}}
		actual := c.convCallbackOutput(mockOutput)
		assert.Equal(t, actual, mockOutput.Embeddings)
	})
	PatchConvey("Test when output is of type *indexer.CallbackOutput", t, func() {
		mockOutput := &indexer.CallbackOutput{IDs: []string{"1", "2"}}
		actual := c.convCallbackOutput(mockOutput)
		assert.Equal(t, actual, mockOutput.IDs)
	})
	PatchConvey("Test when output is of type *prompt.CallbackOutput", t, func() {
		mockOutput := &prompt.CallbackOutput{Result: []*schema.Message{}}
		actual := c.convCallbackOutput(mockOutput)
		assert.Equal(t, actual, mockOutput.Result)
	})
	PatchConvey("Test when output is of type *retriever.CallbackOutput", t, func() {
		mockOutput := &retriever.CallbackOutput{Docs: []*schema.Document{}}
		actual := c.convCallbackOutput(mockOutput)
		assert.Equal(t, actual, mockOutput.Docs)
	})
	PatchConvey("Test when output is of unknown type", t, func() {
		mockOutput := "unknown"
		actual := c.convCallbackOutput(mockOutput)
		assert.Equal(t, actual, mockOutput)
	})
}
