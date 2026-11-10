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
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/document/parser/csv"
	"github.com/cloudwego/eino/components/document/parser"
)

func main() {

	// 1. Open the CSV file.
	f, err := os.Open("./testdata/test.csv")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	// 2. Create a new CSVParser.
	ctx := context.Background()
	cp, err := csv.NewCSVParser(ctx, &csv.Config{
		NoHeader: false,
		IDPrefix: "",
	})
	if err != nil {
		log.Fatalf("Failed to create CSVParser: %v", err)
	}

	// 3. Parse the CSV content.
	docs, err := cp.Parse(ctx, f, parser.WithURI("./testdata/test.csv"), parser.WithExtraMeta(map[string]any{
		"_extension": ".csv",
		"_file_name": "test.csv",
		"_source":    "local",
	}))

	if err != nil {
		log.Fatalf("Failed to parse CSV: %v", err)
	}
	log.Printf("Parsed documents content %s \n", docs)
	return
}
