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

package runner_wrapper

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

// NewRunner create adk agent runner.
// The only difference between this NewRunner method and adk.NewRunner is that this method runs global callbacks
// For example, when using trace callbacks, this Runner reports the root span for internal aggregation of
// reporting information within the agent.
// This method might become useless after implementing Agent Callback capability.
func NewRunner(ctx context.Context, conf adk.RunnerConfig) *Runner {
	return &Runner{
		name:   conf.Agent.Name(ctx),
		Runner: adk.NewRunner(ctx, conf),
	}
}

type Runner struct {
	name string
	*adk.Runner
}

func (r *Runner) Query(ctx context.Context, query string, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return r.Run(ctx, []adk.Message{schema.UserMessage(query)}, opts...)
}

func (r *Runner) Run(ctx context.Context, messages []adk.Message, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      r.name,
		Type:      "AgentRunner",
		Component: "AgentRunner",
	})
	ctx = callbacks.OnStart(ctx, messages)
	iterator := r.Runner.Run(ctx, messages, opts...)
	nextIterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go r.bypassIter(ctx, iterator, generator)
	return nextIterator
}

func (r *Runner) Resume(ctx context.Context, checkPointID string, opts ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      r.name,
		Type:      "AgentRunner",
		Component: "AgentRunner",
	})
	ctx = callbacks.OnStart(ctx, []adk.Message{})
	iterator, err := r.Runner.Resume(ctx, checkPointID, opts...)
	if err != nil {
		callbacks.OnError(ctx, err)
		return iterator, err
	}
	nextIterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go r.bypassIter(ctx, iterator, generator)
	return nextIterator, nil
}

func (r *Runner) bypassIter(ctx context.Context, iterator *adk.AsyncIterator[*adk.AgentEvent], generator *adk.AsyncGenerator[*adk.AgentEvent]) {
	defer func() {
		if re := recover(); re != nil {
			s := fmt.Errorf("panic error: %v, \nstack: %s", re, string(debug.Stack()))
			log.Printf(s.Error())
			callbacks.OnError(ctx, s)
		}
		generator.Close()
	}()

	var (
		eventErr    error
		foundError  bool
		interrupted bool
		lastMessage adk.MessageStream
	)

	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event.Err != nil && !foundError {
			eventErr = event.Err
			foundError = true
		} else if event.Action != nil && event.Action.Interrupted != nil {
			interrupted = true
		} else if event.Output != nil && event.Output.MessageOutput != nil {
			lastMessage, event = getMessageStream(event)
		}
		generator.Send(event)
	}

	if foundError {
		callbacks.OnError(ctx, eventErr)
	} else if interrupted {
		callbacks.OnEnd(ctx, adk.Message(nil))
	} else if lastMessage != nil {
		msg, concatErr := schema.ConcatMessageStream(lastMessage)
		if concatErr != nil { // skip
			callbacks.OnEnd(ctx, adk.Message(nil))
		} else {
			callbacks.OnEnd(ctx, msg)
		}
	} else { // unexpected
		callbacks.OnEnd(ctx, adk.Message(nil))
	}
}

func getMessageStream(e *adk.AgentEvent) (adk.MessageStream, *adk.AgentEvent) {
	if e.Output == nil || e.Output.MessageOutput == nil {
		return nil, e
	}
	msgOutput := e.Output.MessageOutput
	if msgOutput.IsStreaming {
		ss := msgOutput.MessageStream.Copy(2)
		e.Output.MessageOutput.MessageStream = ss[0]
		return ss[1], e
	}

	sr, sw := schema.Pipe[adk.Message](1)
	sw.Send(msgOutput.Message, nil)
	sw.Close()
	return sr, e
}
