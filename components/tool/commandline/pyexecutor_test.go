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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPyExecutor(t *testing.T) {
	ctx := context.Background()

	code := "print('hello world')"
	op := &pyOperator{code: code}
	exec, err := NewPyExecutor(ctx, &PyExecutorConfig{Operator: op})
	assert.Nil(t, err)
	result, err := exec.InvokableRun(ctx, `{"code": "`+code+`"}`)
	assert.Nil(t, err)
	assert.Equal(t, "hello world\n", result)
}

type pyOperator struct {
	code string
}

func (pyOperator) RunCommand(ctx context.Context, command []string) (*CommandOutput, error) {
	return &CommandOutput{Stdout: "hello world\n", Stderr: "", ExitCode: 0}, nil
}
func (pyOperator) ReadFile(ctx context.Context, path string) (string, error) {
	panic("implement me")
}

func (pyOperator) WriteFile(ctx context.Context, path string, content string) error {
	return nil
}

func (pyOperator) IsDirectory(ctx context.Context, path string) (bool, error) {
	panic("implement me")
}

func (pyOperator) Exists(ctx context.Context, path string) (bool, error) {
	panic("implement me")
}
