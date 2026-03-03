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
)

// CommandOutput represents the result of executing a command in a sandboxed environment.
// It captures the standard output, standard error, and exit code of the process.
type CommandOutput struct {
	// Stdout contains the data written by the command to standard output (stdout).
	// This is typically the "normal" output of a successful command.
	Stdout string

	// Stderr contains the data written by the command to standard error (stderr).
	// This often includes warnings, errors, or diagnostic messages,
	// even if the command exits successfully (exit code 0).
	Stderr string

	// ExitCode is the numeric exit status returned by the command.
	// - 0 usually indicates success.
	// - Non-zero values (1–255) typically indicate failure.
	// - Special values like 137 (128 + 9) suggest the process was killed by signal 9 (SIGKILL),
	//   often due to exceeding memory limits (OOM).
	ExitCode int
}

// Operator defines the interface for file operations
type Operator interface {
	ReadFile(ctx context.Context, path string) (string, error)
	WriteFile(ctx context.Context, path string, content string) error
	IsDirectory(ctx context.Context, path string) (bool, error)
	Exists(ctx context.Context, path string) (bool, error)
	RunCommand(ctx context.Context, command []string) (*CommandOutput, error)
}
