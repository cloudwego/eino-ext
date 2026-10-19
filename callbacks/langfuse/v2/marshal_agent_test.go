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

package langfuse

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// marshalNoopTool only has to exist: its presence moves the agent onto the ReAct
// path, the path that carries adk.reactRunInput across a callback.
type marshalNoopTool struct{}

func (marshalNoopTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "noop", Desc: "noop"}, nil
}

func (marshalNoopTool) InvokableRun(context.Context, string, ...tool.Option) (string, error) {
	return "ok", nil
}

// TestHandlerExportsUnexportedAgentRunInput is the only test that runs one of
// Eino's own types past the callback. It has to be, since the gate matches by
// package path and name and no stand-in in this package can reproduce that; it is
// also what fails if Eino renames the type. On every ReAct agent run Eino hands
// adk.reactRunInput to the Chain and to the Lambda node that Chain starts with,
// so both observations report the same input — "{}" before this change.
func TestHandlerExportsUnexportedAgentRunInput(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	handler, err := NewHandler(context.Background(), &Config{
		SpanExporter:            exporter,
		MaxAttributeValueLength: -1,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := handler.StartTrace(context.Background(), WithName("root"))
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "writer",
		Instruction: "you are helpful",
		Model:       &scopeModel{name: "writer-model"},
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{marshalNoopTool{}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	iter := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true}).
		Run(ctx, []*schema.Message{schema.UserMessage("write a chapter")}, adk.WithCallbacks(handler))
	for {
		if _, ok := iter.Next(); !ok {
			break
		}
	}
	handler.EndTrace(ctx, "")
	if err := handler.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("no span was exported")
	}
	var carried bool
	for _, span := range spans {
		input := attributesByKey(span.Attributes)["langfuse.observation.input"].Value.AsString()
		t.Logf("span %-24s input %s", span.Name, input)
		if input == "{}" {
			t.Errorf("span %q exported an empty input; the agent input was dropped", span.Name)
		}
		if strings.Contains(input, "write a chapter") {
			carried = true
		}
	}
	if !carried {
		t.Fatal("no span carried the agent input messages")
	}
}
