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

package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	einoacp "github.com/cloudwego/eino-ext/acp"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	acp "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
)

// sessionState holds per-session runtime: the adk Runner and the accumulated
// conversation history. A per-session mutex serializes Prompt calls so turns
// don't interleave and history stays consistent.
type sessionState struct {
	runner  *adk.Runner
	mu      sync.Mutex
	history []adk.Message
}

// agent implements acp.Agent by embedding BaseAgent for default stubs,
// and overriding the methods we care about.
type agent struct {
	acp.BaseAgent

	conn               *acpconn.AgentConnection
	clientCapabilities *acp.ClientCapabilities

	sessionSeq atomic.Uint64
	mu         sync.Mutex
	sessions   map[acp.SessionID]*sessionState
}

func newAgent() *agent {
	return &agent{
		sessions: make(map[acp.SessionID]*sessionState),
	}
}

// SetClientConnection is called by the ACP server framework to inject the connection.
func (a *agent) SetClientConnection(conn *acpconn.AgentConnection) {
	a.conn = conn
}

func (a *agent) Initialize(_ context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error) {
	a.clientCapabilities = req.ClientCapabilities
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersion(acp.CurrentProtocolVersion),
		AgentInfo: &acp.Implementation{
			Name:    "eino-acp-example",
			Version: "0.1.0",
		},
	}, nil
}

func (a *agent) NewSession(ctx context.Context, _ acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	sessionID := acp.SessionID(fmt.Sprintf("session-%d", a.sessionSeq.Add(1)))

	chatModel, err := createChatModel(ctx)
	if err != nil {
		return acp.NewSessionResponse{}, fmt.Errorf("failed to create chat model: %w", err)
	}

	var middlewares []adk.ChatModelAgentMiddleware

	// If the client supports filesystem or terminal, add the ACP client tools middleware.
	// This bridges ACP's ReadTextFile/WriteTextFile/Terminal to eino's filesystem tools.
	if a.clientCapabilities != nil {
		m, err := einoacp.NewClientToolsMiddleware(ctx, sessionID, a.clientCapabilities, a.conn)
		if err != nil {
			return acp.NewSessionResponse{}, fmt.Errorf("failed to create client tools middleware: %w", err)
		}
		middlewares = append(middlewares, m)
	}

	adkAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "example-agent",
		Description: "An example agent served over ACP",
		Instruction: "You are a helpful assistant.",
		Model:       chatModel,
		Handlers:    middlewares,
	})
	if err != nil {
		return acp.NewSessionResponse{}, fmt.Errorf("failed to create agent: %w", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           adkAgent,
		EnableStreaming: true,
	})

	a.mu.Lock()
	a.sessions[sessionID] = &sessionState{runner: runner}
	a.mu.Unlock()

	return acp.NewSessionResponse{SessionID: sessionID}, nil
}

func (a *agent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	a.mu.Lock()
	sess, ok := a.sessions[req.SessionID]
	a.mu.Unlock()
	if !ok {
		return acp.PromptResponse{}, fmt.Errorf("session %s not found", req.SessionID)
	}

	// Serialize turns within a session: history append must not interleave with
	// another concurrent Prompt on the same session.
	sess.mu.Lock()
	defer sess.mu.Unlock()

	// Convert ACP prompt content to a plain text query for the eino agent.
	query := extractTextFromPrompt(req.Prompt)
	userMsg := schema.UserMessage(query)

	// Feed prior history + this turn's user message into the runner, so the model
	// sees the full conversation.
	input := make([]adk.Message, 0, len(sess.history)+1)
	input = append(input, sess.history...)
	input = append(input, userMsg)

	iter := sess.runner.Run(ctx, input)

	// Collect messages produced this turn (assistant text, tool calls, tool results)
	// so they can be appended to history after a successful turn.
	var newMessages []adk.Message

	// Stream agent events back to the ACP client as SessionUpdates.
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return acp.PromptResponse{}, event.Err
		}

		// For streaming events the underlying MessageStream is consumed (and closed) by
		// AgentEventToSessionUpdate. Tee it via Copy(2) so we can read the same chunks
		// afterwards and concat them into a full message for history.
		var historyCopy adk.MessageStream
		if mo := event.Output; mo != nil && mo.MessageOutput != nil && mo.MessageOutput.IsStreaming {
			copies := mo.MessageOutput.MessageStream.Copy(2)
			mo.MessageOutput.MessageStream = copies[0]
			historyCopy = copies[1]
		}

		// AgentEventToSessionUpdate converts eino events (messages, tool calls,
		// interrupts, etc.) into ACP SessionUpdate notifications.
		for su, err := range einoacp.AgentEventToSessionUpdate(event, nil) {
			if err != nil {
				if historyCopy != nil {
					historyCopy.Close()
				}
				return acp.PromptResponse{}, err
			}
			if err = a.conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionID: req.SessionID,
				Update:    su,
			}); err != nil {
				if historyCopy != nil {
					historyCopy.Close()
				}
				return acp.PromptResponse{}, fmt.Errorf("failed to send session update, error: %w", err)
			}
		}

		if msg := capturedMessage(event, historyCopy); msg != nil {
			newMessages = append(newMessages, msg)
		}
	}

	// Turn succeeded: persist the user turn + everything the agent produced.
	sess.history = append(sess.history, userMsg)
	sess.history = append(sess.history, newMessages...)

	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// capturedMessage returns the message payload of an event for history recording.
// For non-streaming events it returns event.Output.MessageOutput.Message directly.
// For streaming events it drains historyCopy (the second Copy() branch of the
// original MessageStream) and concatenates the chunks into a single message.
// Returns nil when the event carried no message payload.
func capturedMessage(event *adk.AgentEvent, historyCopy adk.MessageStream) adk.Message {
	if event.Output == nil || event.Output.MessageOutput == nil {
		return nil
	}
	mo := event.Output.MessageOutput
	if !mo.IsStreaming {
		return mo.Message
	}
	if historyCopy == nil {
		return nil
	}
	// ConcatMessageStream drains and closes the stream.
	msg, err := schema.ConcatMessageStream(historyCopy)
	if err != nil {
		return nil
	}
	return msg
}

// --- Helpers ---

func extractTextFromPrompt(blocks []acp.ContentBlock) string {
	for _, block := range blocks {
		if tc, ok := block.AsText(); ok {
			return tc.Text
		}
	}
	return ""
}

func createChatModel(ctx context.Context) (*ark.ChatModel, error) {
	config := &ark.ChatModelConfig{
		APIKey:  os.Getenv("ARK_API_KEY"),
		Model:   os.Getenv("ARK_MODEL"),
		BaseURL: os.Getenv("ARK_BASE_URL"),
	}
	return ark.NewChatModel(ctx, config)
}

// Verify interface compliance at compile time.
var _ acp.Agent = (*agent)(nil)
