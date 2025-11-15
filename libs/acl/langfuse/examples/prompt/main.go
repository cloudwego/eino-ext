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

package examples

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/libs/acl/langfuse"
)

func main() {
	ctx := context.Background()

	lf := langfuse.NewLangfuse(
		"https://cloud.your-langfuse-instance.com",
		"your-public-key",
		"your-secret-key",
	)

	promptClient := lf.Prompt()

	prompt, err := promptClient.GetPrompt(ctx, langfuse.GetParams{
		Name: "welcome-message",
	})
	if err != nil {
		log.Fatalf("failed to fetch prompt: %v", err)
	}

	fmt.Printf("Loaded prompt %q version %d\n", prompt.Name, prompt.Version)
}
