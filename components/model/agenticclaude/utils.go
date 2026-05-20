/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticclaude

import (
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

func getEnvWithFallbacks(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func newClaudeOpt[T comparable](v *T) param.Opt[T] {
	if v == nil {
		return param.Opt[T]{}
	}
	return param.NewOpt(*v)
}

func newClaudeStrOpt(v string) param.Opt[string] {
	if v == "" {
		return param.Opt[string]{}
	}
	return param.NewOpt(v)
}

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v,\nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}

func setStringArgument(m map[string]any, key string, dst *string) error {
	value, ok := m[key]
	if !ok {
		return nil
	}

	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("failed to set %q argument: expected string, got %T", key, value)
	}

	*dst = str
	return nil
}
