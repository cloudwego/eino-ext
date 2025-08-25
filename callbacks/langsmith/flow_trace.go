package langsmith

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

type FlowTrace struct { // associating multiple sessions with the same trace
	cli Langsmith
	cfg *Config
}

func NewFlowTrace(cfg *Config) *FlowTrace {
	cli := NewLangsmith(cfg.APIKey, cfg.APIURL)
	if cfg.RunIDGen == nil {
		cfg.RunIDGen = func(ctx context.Context) string {
			return uuid.NewString()
		}
	}
	return &FlowTrace{cli: cli, cfg: cfg}
}

func (ft *FlowTrace) StartSpan(ctx context.Context, name string, state *LangsmithState) (context.Context, string, error) {
	opts, _ := ctx.Value(langsmithTraceOptionKey{}).(*traceOptions)
	if opts == nil {
		opts = &traceOptions{}
	}
	if state == nil {
		state = &LangsmithState{}
	}
	runID := ft.cfg.RunIDGen(ctx)
	run := &Run{
		ID:          runID,
		TraceID:     state.TraceID,
		Name:        name,
		RunType:     RunTypeChain,
		StartTime:   time.Now().UTC(),
		SessionName: opts.SessionName,
		Extra:       opts.Metadata,
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
	err := ft.cli.CreateRun(ctx, run)
	if err != nil {
		return nil, "", err
	}
	newState := &LangsmithState{
		TraceID:           state.TraceID,
		ParentRunID:       runID,
		ParentDottedOrder: run.DottedOrder,
	}

	return context.WithValue(ctx, langsmithStateKey{}, newState), runID, nil
}

func (ft *FlowTrace) FinishSpan(ctx context.Context, runID string) {
	endTime := time.Now().UTC()
	patch := &RunPatch{
		EndTime: &endTime,
	}

	err := ft.cli.UpdateRun(ctx, runID, patch)
	if err != nil {
		log.Printf("[langsmith] failed to FinishSpan: %v", err)
	}
}

// SpanToString parse ctx's LangsmithState to string
func (ft *FlowTrace) SpanToString(ctx context.Context) (string, error) {
	ctx, state := GetState(ctx)
	if state == nil {
		return "", nil
	}
	val, err := sonic.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// StringToSpan parse string to LangsmithState
func (ft *FlowTrace) StringToSpan(val string) (*LangsmithState, error) {
	if val == "" {
		return nil, nil
	}
	state := &LangsmithState{}
	err := sonic.Unmarshal([]byte(val), state)
	return state, err
}
