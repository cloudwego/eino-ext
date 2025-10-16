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

package langsmith

import (
	"context"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// Config LangsmithHandler configuration
type Config struct {
	APIKey   string                           // langsmith api key
	APIURL   string                           // langsmith api url, default:https://api.smith.langchain.com
	RunIDGen func(ctx context.Context) string // langsmith run_id generator
}

// CallbackHandler implements eino's Handler interface
type CallbackHandler struct {
	cli Langsmith
	cfg *Config
}

// NewLangsmithHandler creates a new CallbackHandler
func NewLangsmithHandler(cfg *Config) (*CallbackHandler, error) {
	// default run id generator
	if cfg.RunIDGen == nil {
		cfg.RunIDGen = func(ctx context.Context) string {
			return uuid.NewString()
		}
	}
	cli := NewLangsmith(cfg.APIKey, cfg.APIURL)
	return &CallbackHandler{
		cli: cli,
		cfg: cfg,
	}, nil
}

// LangsmithState maintains Langsmith call chain state
type LangsmithState struct {
	TraceID           string                 `json:"trace_id"`
	ParentRunID       string                 `json:"parent_run_id"`
	ParentDottedOrder string                 `json:"parent_dotted_order"`
	Metadata          *sync.Map              `json:"metadata"`
	Tags              []string               `json:"tags"`
	MarshalMetadata   map[string]interface{} `json:"marshal_metadata"`
}

type langsmithStateKey struct{}

// OnStart handles call start event
func (c *CallbackHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil {
		return ctx
	}

	ctx, state := GetOrInitState(ctx)
	runID := c.cfg.RunIDGen(ctx)

	opts, _ := ctx.Value(langsmithTraceOptionKey{}).(*traceOptions)
	if opts == nil {
		opts = &traceOptions{}
	}
	in, err := sonic.MarshalString(input)
	if err != nil {
		log.Printf("marshal input error: %v, runinfo: %+v", err, info)
		return ctx
	}
	var metaData = SafeDeepCopySyncMapMetadata(opts.Metadata)
	if input != nil {
		modelConf, _, _, _ := extractModelInput(convModelCallbackInput([]callbacks.CallbackInput{input}))
		if modelConf != nil {
			var tmp = metaData["metadata"].(map[string]interface{})
			tmp["ls_model_name"] = modelConf.Model
			tmp["ls_max_tokens"] = modelConf.MaxTokens
			tmp["model_conf"] = modelConf
			metaData["metadata"] = tmp
		}
	}

	run := &Run{
		ID:          runID,
		TraceID:     state.TraceID,
		Name:        runInfoToName(info),
		RunType:     runInfoToRunType(info),
		StartTime:   time.Now().UTC(),
		Inputs:      map[string]interface{}{"input": in},
		SessionName: opts.SessionName,
		Extra:       metaData,
		Tags:        opts.Tags,
	}
	if state.TraceID == "" {
		run.TraceID = runID
	}

	if opts.ReferenceExampleID != "" {
		run.ReferenceExampleID = &opts.ReferenceExampleID
	}
	if state.ParentRunID != "" {
		run.ParentRunID = &state.ParentRunID
	}
	nowTime := run.StartTime.Format("20060102T150405000000")
	if state.ParentDottedOrder != "" {
		run.DottedOrder = fmt.Sprintf("%s.%sZ%s", state.ParentDottedOrder, nowTime, runID)
	} else {
		run.DottedOrder = fmt.Sprintf("%sZ%s", nowTime, runID)
	}

	err = c.cli.CreateRun(ctx, run)
	if err != nil {
		log.Printf("[langsmith] failed to create run: %v", err)
	}
	fmt.Printf("[langsmith] runinfo: %+v\n", run)
	var newSyncMap = &sync.Map{}
	for k, v := range run.Extra {
		newSyncMap.Store(k, v)
	}
	newState := &LangsmithState{
		TraceID:           run.TraceID,
		ParentRunID:       runID,
		ParentDottedOrder: run.DottedOrder,
		Metadata:          newSyncMap,
		Tags:              run.Tags,
	}
	return context.WithValue(ctx, langsmithStateKey{}, newState)
}

// OnEnd handles successful call completion event
func (c *CallbackHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if info == nil {
		return ctx
	}
	state, ok := ctx.Value(langsmithStateKey{}).(*LangsmithState)
	if !ok || state == nil {
		log.Printf("[langsmith] no state in context on OnEnd, runinfo: %+v", info)
		return ctx
	}
	out, err := sonic.MarshalString(output)
	if err != nil {
		log.Printf("marshal output error: %v, runinfo: %+v", err, info)
		return ctx
	}

	endTime := time.Now().UTC()
	patch := &RunPatch{
		EndTime: &endTime,
		Outputs: map[string]interface{}{"output": out},
	}

	err = c.cli.UpdateRun(ctx, state.ParentRunID, patch)
	if err != nil {
		log.Printf("[langsmith] failed to update run: %v", err)
	}
	return ctx
}

// OnError handles call failure event
func (c *CallbackHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if info == nil {
		return ctx
	}
	state, ok := ctx.Value(langsmithStateKey{}).(*LangsmithState)
	if !ok || state == nil {
		log.Printf("[langsmith] no state in context on OnError, runinfo: %+v", info)
		return ctx
	}

	endTime := time.Now().UTC()
	errStr := err.Error()
	patch := &RunPatch{
		EndTime: &endTime,
		Error:   &errStr,
	}

	updateErr := c.cli.UpdateRun(ctx, state.ParentRunID, patch)
	if updateErr != nil {
		log.Printf("[langsmith] failed to update run with error: %v", updateErr)
	}
	return ctx
}

// OnStartWithStreamInput handles streaming input initialization
func (c *CallbackHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if info == nil {
		input.Close()
		return ctx
	}
	ctx, state := GetOrInitState(ctx)
	runID := c.cfg.RunIDGen(ctx)

	opts, _ := ctx.Value(langsmithTraceOptionKey{}).(*traceOptions)
	if opts == nil {
		opts = &traceOptions{}
	}

	run := &Run{
		ID:          runID,
		TraceID:     state.TraceID,
		Name:        runInfoToName(info),
		RunType:     runInfoToRunType(info),
		StartTime:   time.Now().UTC(),
		SessionName: opts.SessionName,
		Tags:        opts.Tags,
	}
	if state.TraceID == "" {
		run.TraceID = runID
	}
	nowTime := run.StartTime.Format("20060102T150405000000")
	if state.ParentDottedOrder != "" {
		run.DottedOrder = fmt.Sprintf("%s.%sZ%s", state.ParentDottedOrder, nowTime, runID)
	} else {
		run.DottedOrder = fmt.Sprintf("%sZ%s", nowTime, runID)
	}
	var metaData = SafeDeepCopySyncMapMetadata(opts.Metadata)
	var newSyncMap = &sync.Map{}
	for k, v := range metaData {
		newSyncMap.Store(k, v)
	}
	// start goroutine to handle stream input
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[langsmith] recovered in OnStartWithStreamInput: %v\n%s", r, debug.Stack())
			}
			input.Close()
		}()

		var inputs []callbacks.CallbackInput
		for {
			chunk, err := input.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("[langsmith] error receiving stream input: %v", err)
				break
			}
			inputs = append(inputs, chunk)
		}
		modelConf, inMessage, extra, err_ := extractModelInput(convModelCallbackInput(inputs))
		if err_ != nil {
			log.Printf("extract stream model input error: %v, runinfo: %+v", err_, info)
			return
		}

		if extra != nil {
			for k, v := range extra {
				metaData[k] = v
			}
		}
		if modelConf != nil {
			var tmp = metaData["metadata"].(map[string]interface{})
			tmp["ls_model_name"] = modelConf.Model
			tmp["ls_max_tokens"] = modelConf.MaxTokens
			tmp["model_conf"] = modelConf
			metaData["metadata"] = tmp
			newSyncMap.Store("metadata", tmp)
		}

		if opts.ReferenceExampleID != "" {
			run.ReferenceExampleID = &opts.ReferenceExampleID
		}
		if state.ParentRunID != "" {
			run.ParentRunID = &state.ParentRunID
		}

		run.Inputs = map[string]interface{}{"stream_inputs": inMessage}
		run.Extra = metaData
		err := c.cli.CreateRun(ctx, run)
		if err != nil {
			log.Printf("[langsmith] failed to create run for stream: %v", err)
		}
	}()

	newState := &LangsmithState{
		TraceID:           run.TraceID,
		ParentRunID:       runID,
		ParentDottedOrder: run.DottedOrder,
		Metadata:          newSyncMap,
		Tags:              run.Tags,
	}
	return context.WithValue(ctx, langsmithStateKey{}, newState)
}

// OnEndWithStreamOutput handles streaming output completion
func (c *CallbackHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if info == nil {
		output.Close()
		return ctx
	}
	state, ok := ctx.Value(langsmithStateKey{}).(*LangsmithState)
	if !ok || state == nil {
		log.Printf("[langsmith] no state in context on OnEndWithStreamOutput, runinfo: %+v", info)
		output.Close()
		return ctx
	}
	var metaData = SafeDeepCopySyncMapMetadata(state.Metadata)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[langsmith] recovered in OnEndWithStreamOutput: %v\n%s", r, debug.Stack())
			}
			output.Close()
		}()

		var outputs []callbacks.CallbackOutput
		for {
			chunk, err := output.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("[langsmith] error receiving stream output: %v", err)
				break
			}
			outputs = append(outputs, chunk)
		}
		usage, outMessage, extra, err_ := extractModelOutput(convModelCallbackOutput(outputs))
		if err_ != nil {
			log.Printf("extract stream model output error: %v, runinfo: %+v", err_, info)
			return
		}
		if extra != nil {
			for k, v := range extra {
				metaData[k] = v
			}
		}
		if usage != nil {
			var tmp = metaData["metadata"].(map[string]interface{})
			var langsmithUsage = map[string]int{
				"input_tokens":  usage.PromptTokens,
				"output_tokens": usage.CompletionTokens,
				"total_tokens":  usage.TotalTokens,
			}
			tmp["usage_metadata"] = langsmithUsage
			metaData["metadata"] = tmp
		}
		endTime := time.Now().UTC()
		patch := &RunPatch{
			EndTime: &endTime,
			Outputs: map[string]interface{}{"stream_outputs": outMessage},
			Extra:   metaData,
		}

		// 使用后台 context
		err := c.cli.UpdateRun(context.Background(), state.ParentRunID, patch)
		if err != nil {
			log.Printf("[langsmith] failed to update run with stream output: %v", err)
		}
	}()

	return ctx
}
