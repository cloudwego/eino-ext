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
	"log"

	"github.com/cloudwego/eino-ext/components/tool/commandline"
	"github.com/cloudwego/eino-ext/components/tool/commandline/sandbox"
)

func main() {
	ctx := context.Background()
	op, err := sandbox.NewDockerSandbox(ctx, &sandbox.Config{})
	if err != nil {
		log.Fatal(err)
	}
	// you should ensure that docker has been started before create a docker container
	err = op.Create(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer op.Cleanup(ctx)

	exec, err := commandline.NewPyExecutor(ctx, &commandline.PyExecutorConfig{Operator: op}) // use python3 by default
	if err != nil {
		log.Fatal(err)
	}

	info, err := exec.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tool name: %s,  tool desc: %s", info.Name, info.Desc)

	code := "# Assuming we calculate a weighted influence index as follows:\n# Influence Index = (2 * Followers) + (3 * Likes) + (0.1 * Downloads[last_month])\n\nfollowers = 55538\nlikes = 11852\ndownloads_last_month = 1342593\n\ninfluence_index = (2 * followers) + (3 * likes) + (0.1 * downloads_last_month)\nprint(f\"Influence Index of DeepSeek R1: {influence_index}\")"
	log.Printf("execute code:\n%s", code)
	result, err := exec.Execute(ctx, &commandline.Input{Code: code})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("result:\n", result)
}
