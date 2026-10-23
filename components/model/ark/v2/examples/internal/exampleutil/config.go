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

package exampleutil

import (
	"context"
	"errors"
	"os"
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
)

// Run shares environment configuration and bounds the entire example, including
// follow-up requests. The caller handles errors after all deferred cleanup runs.
func Run(timeout time.Duration, run func(context.Context, *ark.ChatModelConfig) error) error {
	config := &ark.ChatModelConfig{
		APIKey:  os.Getenv("ARK_API_KEY"),
		Model:   os.Getenv("ARK_MODEL"),
		BaseURL: os.Getenv("ARK_BASE_URL"),
	}
	if config.APIKey == "" || config.Model == "" {
		return errors.New("set ARK_API_KEY and ARK_MODEL before running this example")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return run(ctx, config)
}
