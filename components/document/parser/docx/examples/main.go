package main

import (
	"context"
	"docx"
	"fmt"
	"log"
	"os"
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
		ToSections:      true, // Split content into sections
		IncludeComments: true,
		IncludeHeaders:  true,
		IncludeFooters:  true,
		IncludeTables:   true,
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
