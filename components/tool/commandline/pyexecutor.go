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

package commandline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/eino-contrib/jsonschema"
	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const defaultPythonCommand = "python3"

type PyExecutorConfig struct {
	Command  string `json:"command"`
	Operator Operator
}

func NewPyExecutor(_ context.Context, cfg *PyExecutorConfig) (*PyExecutor, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if cfg.Operator == nil {
		return nil, errors.New("operator is required")
	}
	command := cfg.Command
	if len(command) == 0 {
		command = defaultPythonCommand
	}

	return &PyExecutor{
		info: &schema.ToolInfo{
			Name: "python_execute",
			Desc: "Executes Python code string. Note: Only print outputs are visible, function return values are not captured. Use print statements to see results.",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(
				&jsonschema.Schema{
					Type: string(schema.Object),
					Properties: orderedmap.New[string, *jsonschema.Schema](
						orderedmap.WithInitialData[string, *jsonschema.Schema](
							orderedmap.Pair[string, *jsonschema.Schema]{
								Key: "code",
								Value: &jsonschema.Schema{
									Type:        string(schema.String),
									Description: "The Python code to execute.",
								},
							},
						),
					),
				},
			),
		},
		command:  command,
		operator: cfg.Operator,
	}, nil
}

type PyExecutor struct {
	info     *schema.ToolInfo
	command  string
	operator Operator
}

func (p *PyExecutor) Info(_ context.Context) (*schema.ToolInfo, error) {
	return p.info, nil
}

type Input struct {
	Code string `json:"code"`
}

func (p *PyExecutor) Execute(ctx context.Context, args *Input) (*CommandOutput, error) {
	fileName := uuid.New().String() + ".py"
	err := p.operator.WriteFile(ctx, fileName, args.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to create python file: %w", err)
	}

	cmdOutput, err := p.operator.RunCommand(ctx, []string{p.command, fileName})
	if err != nil {
		return nil, fmt.Errorf("execute error: %w", err)
	}

	return cmdOutput, nil
}

func (p *PyExecutor) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	args := &Input{}
	if err := json.Unmarshal([]byte(argumentsInJSON), args); err != nil {
		return "", fmt.Errorf("extract argument fail: %w", err)
	}

	result, err := p.Execute(ctx, args)
	if err != nil {
		return "", fmt.Errorf("execute error: %w", err)
	}
	if len(result.Stdout) == 0 {
		return "", errors.New("execute result is empty")
	}
	return result.Stdout, nil
}
