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

	sre, err := commandline.NewStrReplaceEditor(ctx, &commandline.EditorConfig{Operator: op})
	if err != nil {
		log.Fatal(err)
	}

	info, err := sre.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tool name: %s, tool desc: %s", info.Name, info.Desc)

	content := "hello world"

	log.Println("create file[test.txt]...")
	result, err := sre.Execute(ctx, &commandline.StrReplaceEditorParams{
		Command:  commandline.CreateCommand,
		Path:     "./test.txt",
		FileText: &content,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("create file result: ", result)

	log.Println("view file[test.txt]...")
	result, err = sre.Execute(ctx, &commandline.StrReplaceEditorParams{
		Command: commandline.ViewCommand,
		Path:    "./test.txt",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("view file result: ", result)
}
