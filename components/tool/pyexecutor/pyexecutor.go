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

package pyexecutor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
)

const defaultPythonCommand = "python3"

type Config struct {
	Command string `json:"command"`
}

func NewPyExecutor(_ context.Context, cfg *Config) (*PyExecutor, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	command := cfg.Command
	if len(command) == 0 {
		command = defaultPythonCommand
	}

	return &PyExecutor{
		info: &schema.ToolInfo{
			Name: "python_execute",
			Desc: "Executes Python code string. Note: Only print outputs are visible, function return values are not captured. Use print statements to see results.",
			ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(&openapi3.Schema{
				Type: openapi3.TypeObject,
				Properties: map[string]*openapi3.SchemaRef{
					"code": {
						Value: &openapi3.Schema{
							Type:        openapi3.TypeString,
							Description: "The Python code to execute.",
						},
					},
				},
			}),
		},
		command: command,
	}, nil
}

type PyExecutor struct {
	info    *schema.ToolInfo
	command string
}

func (p *PyExecutor) Info(_ context.Context) (*schema.ToolInfo, error) {
	return p.info, nil
}

type Input struct {
	Code string `json:"code"`
}

func (p *PyExecutor) Execute(ctx context.Context, args *Input) (string, error) {
	cmd := exec.CommandContext(ctx, p.command, "-c", args.Code)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if ctx.Err() != nil {
		return "", fmt.Errorf("context error: %w", ctx.Err())
	}

	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("execute error: %s", stderr.String())
		}
		return "", fmt.Errorf("execute error: %w", err)
	}

	result := stdout.String()
	if result == "" && stderr.Len() > 0 {
		result = fmt.Sprintf("warning: %s", stderr.String())
	}

	return result, nil
}

func (p *PyExecutor) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	args := &Input{}
	if err := json.Unmarshal([]byte(argumentsInJSON), args); err != nil {
		return "", fmt.Errorf("extract argument fail: %w", err)
	}

	return p.Execute(ctx, args)
}
