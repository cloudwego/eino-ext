/*
 * Copyright 2024 CloudWeGo Authors
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
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
)

func main() {
	ctx := context.Background()

	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   1500,
		OverlapSize: 300,
		KeepType:    recursive.KeepTypeNone,
	})
	if err != nil {
		log.Fatal(err)
	}

	file := "./testdata/einodoc.md"
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	docs, err := splitter.Transform(ctx, []*schema.Document{
		{
			Content: string(data),
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	for idx, doc := range docs {
		fmt.Printf("====== %02d ======\n", idx)
		fmt.Println(doc.Content)
	}

}
