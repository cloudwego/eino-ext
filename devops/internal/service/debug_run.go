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
	"fmt"
	"reflect"
	"sync"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino-ext/devops/internal/model"
	"github.com/cloudwego/eino-ext/devops/internal/utils/log"
	"github.com/cloudwego/eino-ext/devops/internal/utils/safego"
)

// TODO@liujian: implement debug run service

var _ DebugService = &debugServiceImpl{}

//go:generate mockgen -source=debug_run.go -destination=../mock/debug_run_mock.go -package=mock
type DebugService interface {
	CreateDebugThread(ctx context.Context, graphID string) (threadID string, err error)
	DebugRun(ctx context.Context, m *model.DebugRunMeta, userInput string) (debugID string, stateCh chan *model.NodeDebugState, errCh chan error, err error)
}

type debugServiceImpl struct {
	mu sync.RWMutex
	// debugGraphs: graphID vs DebugGraph
	debugGraphs map[string]*model.DebugGraph
}

func newDebugService() DebugService {
	return &debugServiceImpl{
		mu:          sync.RWMutex{},
		debugGraphs: make(map[string]*model.DebugGraph, 10),
	}
}

func (d *debugServiceImpl) CreateDebugThread(ctx context.Context, graphID string) (threadID string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	dg := d.debugGraphs[graphID]
	if dg == nil {
		dg = &model.DebugGraph{
			DT: make([]*model.DebugThread, 0, 10),
		}
		d.debugGraphs[graphID] = dg
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generate thread id failed, err=%w", err)
	}
	threadID = id.String()

	dg.DT = append(dg.DT, &model.DebugThread{ID: threadID})

	return threadID, nil
}

func (d *debugServiceImpl) DebugRun(ctx context.Context, rm *model.DebugRunMeta, userInput string) (debugID string,
	stateCh chan *model.NodeDebugState, errCh chan error, err error) {
	d.mu.RLock()
	dg := d.debugGraphs[rm.GraphID]
	if dg == nil {
		d.mu.RUnlock()
		return "", nil, nil, fmt.Errorf("graph=%s not exist", rm.GraphID)
	}
	d.mu.RUnlock()

	_, ok := dg.GetDebugThread(rm.ThreadID)
	if !ok {
		return "", nil, nil, fmt.Errorf("thread=%s not exist", rm.ThreadID)
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return "", nil, nil, err
	}
	debugID = id.String()

	r, ok := ContainerSVC.GetRunnable(rm.GraphID, rm.FromNode)
	if !ok {
		r, err = ContainerSVC.CreateRunnable(rm.GraphID, rm.FromNode)
		if err != nil {
			return "", nil, nil, fmt.Errorf("create runnable failed, err=%w", err)
		}
	}

	gi, ok := ContainerSVC.GetGraphInfo(rm.GraphID)
	if !ok {
		return "", nil, nil, fmt.Errorf("graph=%s not exist", rm.GraphID)
	}

	var (
		needInfer      = true
		unmarshalInput model.UnmarshalInput
	)
	for _, nn := range gi.Option.NodeInputUnmarshal {
		if nn.NodeKey == rm.FromNode {
			needInfer = false
			unmarshalInput = nn.UnmarshalInput
			break
		}
	}

	var input reflect.Value
	if !needInfer {
		inputIns, err := unmarshalInput(ctx, userInput)
		if err != nil {
			return "", nil, nil, err
		}
		input = reflect.ValueOf(inputIns)
	} else {
		inputType, ok, err := gi.InferGraphInputType(rm.FromNode)
		if err != nil {
			return "", nil, nil, err
		}
		if !ok {
			return "", nil, nil, fmt.Errorf("node=%s is not operational", rm.FromNode)
		}
		input, err = inputType.UnmarshalJson(userInput)
		if err != nil {
			return "", nil, nil, err
		}
	}

	stateCh = make(chan *model.NodeDebugState, 100)

	opts, err := d.getInvokeOptions(rm.GraphID, rm.ThreadID, stateCh)
	if err != nil {
		close(stateCh)
		return "", nil, nil, fmt.Errorf("get invoke option failed, err=%w", err)
	}

	errCh = make(chan error, 1)
	safego.Go(ctx, func() {
		defer close(stateCh)
		defer close(errCh)

		_, e := r.Invoke(ctx, input, opts...)
		if e != nil {
			errCh <- e
			log.Errorf("invoke failed, userInput=%s\nerr=%s", userInput, e)
			return
		}
	})

	return debugID, stateCh, errCh, nil
}

func (d *debugServiceImpl) getInvokeOptions(graphID, threadID string, stateCh chan *model.NodeDebugState) (opts []compose.Option, err error) {
	gi, ok := ContainerSVC.GetGraphInfo(graphID)
	if !ok {
		return nil, fmt.Errorf("graph=%s not exist", graphID)
	}

	opts = make([]compose.Option, 0, len(gi.Nodes))
	for key, node := range gi.Nodes {
		opts = append(opts, newCallbackOption(key, threadID, node, stateCh))
	}

	return opts, nil
}
