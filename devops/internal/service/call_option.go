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
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/cloudwego/eino-ext/devops/internal/model"
	"github.com/cloudwego/eino-ext/devops/internal/utils/log"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func newCallbackOption(nodeKey, threadID string, node compose.GraphNodeInfo, stateCh chan *model.NodeDebugState) compose.Option {
	cb := &callbackHandler{
		nodeKey:  nodeKey,
		threadID: threadID,
		stateCh:  stateCh,
		node:     node,
	}
	op := compose.WithCallbacks(cb).DesignateNode(nodeKey)
	return op
}

type callbackHandler struct {
	nodeKey  string
	stateCh  chan *model.NodeDebugState
	threadID string
	node     compose.GraphNodeInfo
}

func (c *callbackHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	invokeTime := time.Now().UnixMilli()

	ctxVal, ctxValOK := getNodeDebugStateCtx(ctx)
	if ctxValOK && ctxVal.depth > 0 {
		ctxVal.depth++
		return ctx
	}

	inputValue := reflect.ValueOf(input)
	if !inputValue.IsValid() {
		c.systemErrorProcess("callback input is invalid", invokeTime, 0)
		return ctx
	}

	callbackInput := c.convCallbackInput(input)

	// If input key exists, use map string any instead of input value.
	if len(c.node.InputKey) > 0 {
		callbackInput = map[string]interface{}{c.node.InputKey: callbackInput}
	}

	jsonInput, err := json.Marshal(callbackInput)
	if err != nil {
		c.systemErrorProcess(fmt.Sprintf("error serializing callback input to json, err=%v", err), invokeTime, 0)
		return ctx
	}

	return setNodeDebugStateCtx(ctx, &nodeDebugStateCtxValue{
		invokeTimeMS:  invokeTime,
		callbackInput: string(jsonInput),
		depth:         1,
	})
}

func (c *callbackHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	completionTime := time.Now().UnixMilli()
	var (
		invokeTime int64
		jsonInput  string
	)
	ctxVal, ctxValOK := getNodeDebugStateCtx(ctx)

	if ctxValOK {
		if ctxVal.depth > 1 {
			ctxVal.depth--
			return ctx
		}
		invokeTime = ctxVal.invokeTimeMS
		jsonInput = ctxVal.callbackInput
	}
	// get output
	outputValue := reflect.ValueOf(output)
	if !outputValue.IsValid() {
		c.systemErrorProcess("callback output is invalid", invokeTime, completionTime)
		return ctx
	}

	callbackOutput := c.convCallbackOutput(output)

	// If output key exists, use map string any instead of output value.
	if len(c.node.OutputKey) > 0 {
		callbackOutput = map[string]interface{}{c.node.OutputKey: callbackOutput}
	}

	jsonOutput, err := json.Marshal(callbackOutput)
	if err != nil {
		c.systemErrorProcess(fmt.Sprintf("error serializing callback output to json, err=%v", err), invokeTime, completionTime)
		return ctx
	}

	state := &model.NodeDebugState{
		NodeKey: c.nodeKey,
		Input:   jsonInput,
		Output:  string(jsonOutput),
		Metrics: model.NodeDebugMetrics{
			InvokeTimeMS:     invokeTime,
			CompletionTimeMS: completionTime,
		},
	}

	// if the node has the token info, go get it
	ext := c.ConvCallbackOutput(output)
	if ext != nil && ext.TokenUsage != nil {
		state.Metrics.PromptTokens = int64(ext.TokenUsage.PromptTokens)
		state.Metrics.CompletionTokens = int64(ext.TokenUsage.PromptTokens)
	}

	// append result
	c.stateCh <- state
	return ctx
}

func (c *callbackHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	var (
		invokeTime int64
		jsonInput  string
	)
	ctxVal, ctxValOK := getNodeDebugStateCtx(ctx)
	if ctxValOK {
		invokeTime = ctxVal.invokeTimeMS
		jsonInput = ctxVal.callbackInput
	}

	state := &model.NodeDebugState{
		NodeKey:   c.nodeKey,
		Input:     jsonInput,
		Error:     err.Error(),
		ErrorType: model.NodeError,
		Metrics: model.NodeDebugMetrics{
			InvokeTimeMS:     invokeTime,
			CompletionTimeMS: time.Now().UnixMilli(),
		},
	}
	// append result
	c.stateCh <- state
	return ctx
}

func (c *callbackHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo,
	input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	defer input.Close()

	ctxVal, ctxValOK := getNodeDebugStateCtx(ctx)
	if ctxValOK && ctxVal.depth > 0 {
		ctxVal.depth++
		return ctx
	}

	invokeTime := time.Now().UnixMilli()
	chunks, recvErr := c.parseDefaultStreamInput(ctx, input)
	if recvErr != nil {
		c.systemErrorProcess(fmt.Sprintf("parse stream input failed, err=%v", recvErr), invokeTime, 0)
		return ctx
	}

	jsonData, err := json.Marshal(chunks)
	if err != nil {
		c.systemErrorProcess(fmt.Sprintf("error serializing input to JSON, err=%v", err), invokeTime, 0)
		return ctx
	}

	return setNodeDebugStateCtx(ctx, &nodeDebugStateCtxValue{
		invokeTimeMS:  invokeTime,
		callbackInput: string(jsonData),
	})
}

func (c *callbackHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo,
	output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	defer output.Close()
	completionTime := time.Now().UnixMilli()
	var (
		startTime int64
		jsonInput string
	)
	ctxVal, ctxValOK := getNodeDebugStateCtx(ctx)
	if ctxValOK {
		if ctxVal.depth > 1 {
			ctxVal.depth--
			return ctx
		}
		startTime = ctxVal.invokeTimeMS
		jsonInput = ctxVal.callbackInput
	}

	state, recvErr := c.parseDefaultStreamOutput(ctx, output)
	if recvErr != nil {
		c.systemErrorProcess(fmt.Sprintf("parse stream output failed, err=%v", recvErr), startTime, completionTime)
		return ctx
	}

	state.NodeKey = c.nodeKey
	state.Input = jsonInput
	state.Metrics.InvokeTimeMS = startTime
	state.Metrics.CompletionTimeMS = time.Now().UnixMilli()
	c.stateCh <- state
	return ctx
}

func (c *callbackHandler) parseDefaultStreamInput(ctx context.Context, input *schema.StreamReader[callbacks.CallbackInput]) (chunks []callbacks.CallbackInput, err error) {
	for {
		item, recvErr := input.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				break
			}

			return chunks, recvErr
		}
		callbackInput := c.convCallbackInput(item)
		if len(c.node.InputKey) > 0 {
			callbackInput = map[string]any{c.node.InputKey: callbackInput}
		}
		chunks = append(chunks, callbackInput)
	}

	return chunks, nil
}

func (c *callbackHandler) parseDefaultStreamOutput(ctx context.Context, output *schema.StreamReader[callbacks.CallbackOutput]) (state *model.NodeDebugState, err error) {
	state = &model.NodeDebugState{}
	chunks := make([]callbacks.CallbackOutput, 0)
	for {
		item, recvErr := output.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				break
			}
			return nil, recvErr
		}

		cbOutput := c.ConvCallbackOutput(item)
		if cbOutput != nil && cbOutput.TokenUsage != nil {
			state.Metrics.PromptTokens += int64(cbOutput.TokenUsage.PromptTokens)
			state.Metrics.CompletionTokens += int64(cbOutput.TokenUsage.CompletionTokens)
		}

		callbackOutput := c.convCallbackOutput(item)
		if len(c.node.OutputKey) > 0 {
			callbackOutput = map[string]any{c.node.OutputKey: callbackOutput}
		}
		chunks = append(chunks, callbackOutput)
	}
	jsonData, err := json.Marshal(chunks)
	if err != nil {
		log.Errorf("error serializing output to JSON, err=%v", err)
		return nil, err
	}

	state.Output = string(jsonData)
	return state, nil
}

func (c *callbackHandler) ConvCallbackOutput(src callbacks.CallbackOutput) *einomodel.CallbackOutput {
	switch t := src.(type) {
	case *einomodel.CallbackOutput:
		return t
	case *schema.Message:
		return &einomodel.CallbackOutput{
			Message: t,
		}
	default:
		return nil
	}
}

func (c *callbackHandler) systemErrorProcess(errorStr string, invokeTime, completionTime int64) {
	log.Errorf(errorStr)
	state := &model.NodeDebugState{
		NodeKey:   c.nodeKey,
		Error:     errorStr,
		ErrorType: model.SystemError,
		Metrics: model.NodeDebugMetrics{
			InvokeTimeMS:     invokeTime,
			CompletionTimeMS: completionTime,
		},
	}
	c.stateCh <- state
}

type nodeDebugStateCtxKey struct{}

type nodeDebugStateCtxValue struct {
	invokeTimeMS  int64
	callbackInput string
	// when components are used nested, depth indicates the level of nesting
	depth int
}

// setNodeDebugStateCtx set temporary storage node debug state to context.
func setNodeDebugStateCtx(ctx context.Context, val *nodeDebugStateCtxValue) context.Context {
	if val == nil {
		return ctx
	}

	return context.WithValue(ctx, nodeDebugStateCtxKey{}, val)
}

// getNodeDebugStateCtx get temporary storage node debug state from context.
func getNodeDebugStateCtx(ctx context.Context) (*nodeDebugStateCtxValue, bool) {
	val, ok := ctx.Value(nodeDebugStateCtxKey{}).(*nodeDebugStateCtxValue)
	return val, ok
}

func (c *callbackHandler) convCallbackInput(input callbacks.CallbackInput) any {
	switch t := input.(type) {
	case *einomodel.CallbackInput:
		return t.Messages
	case *embedding.CallbackInput:
		return t.Texts
	case *indexer.CallbackInput:
		return t.Docs
	case *prompt.CallbackInput:
		return t.Variables
	case *retriever.CallbackInput:
		return t.Query
	default:
		return input
	}
}

func (c *callbackHandler) convCallbackOutput(output callbacks.CallbackOutput) any {
	switch t := output.(type) {
	case *einomodel.CallbackOutput:
		return t.Message
	case *embedding.CallbackOutput:
		return t.Embeddings
	case *indexer.CallbackOutput:
		return t.IDs
	case *prompt.CallbackOutput:
		return t.Result
	case *retriever.CallbackOutput:
		return t.Docs
	default:
		return output
	}
}
