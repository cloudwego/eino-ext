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

	"github.com/cloudwego/eino-ext/components/document/parser/docx"
)

func main() {
	// 1. Open the DOCX file.
	file, err := os.Open("./testdata/test_docx.docx")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	ctx := context.Background()

	// 2. Configure the parser to include everything.
	config := &docx.Config{
		ToSections:     true, // Split content into sections
		IncludeHeaders: true,
		IncludeFooters: true,
		IncludeTables:  true,
	}

	// 3. Create a new parser instance.
	docxParser, err := docx.NewDocxParser(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create parser: %v", err)
	}

	// 4. Parse the document.
	docs, err := docxParser.Parse(ctx, file)
	if err != nil {
		log.Fatalf("Failed to parse document: %v", err)
	}

	// 5. Print the extracted content.
	fmt.Printf("Successfully parsed %d section(s).\n\n", len(docs))
	for _, doc := range docs {
		fmt.Printf("--- Section ID: %s ---\n", doc.ID)
		fmt.Println(doc.Content)
		fmt.Println("--- End of Section ---")
		fmt.Println()
	}
}
