# Eino - DOCX Parser
The Docx parser is Eino's document parsing component that implements the 'Parser' interface for parsing Microsoft Word (docx) files. This package is designed to parse Microsoft Word (`.docx`) files and extract their content into a structured, plain-text format.

## 📜 Overview
The `docx` package offers a configurable parser to read `.docx` documents from an `io.Reader`. It can selectively extract various parts of the document, including the main body text, headers, footers and tables. The output can be returned as a single consolidated document or split into separate sections.

This parser is built upon the `docx2md` library to handle the underlying XML structure of `.docx` files.

## ✨ Features
+ **Main Content Extraction**: Parses all paragraphs and text runs from the main document body.
+ **Configurable Extraction**: Easily enable or disable the inclusion of:
    - Headers
    - Footers
    - Tables
+ **Flexible Output**:
    - Combine all extracted content into a single document.
    - Split content into separate sections (e.g., main, headers, footers).
+ Lightweight wrapper around the `docx2md` library. 

## ⚙️ Configuration
The behavior of the `DocxParser` is controlled by a `Config` struct. If no configuration is provided, a default one is used.

Here are the available configuration options:

| Field | Type | Description | Default |
| --- | --- | --- | --- |
| `ToSections` | `bool` | If `true`, splits the extracted content into different sections (main, headers, footers, etc.). Otherwise, combines all content. | `false` |
| `IncludeComments` | `bool` | **Deprecated**. This option is no longer supported due to a change in the underlying parsing library. | `false` |
| `IncludeHeaders` | `bool` | If `true`, includes content from all document headers. | `false` |
| `IncludeFooters` | `bool` | If `true`, includes content from all document footers. | `false` |
| `IncludeTables` | `bool` | If `true`, extracts and formats content from all tables in the document. | `false` |


## 🚀 Usage Example
Below is a basic example of how to use the `DocxParser` to read a `.docx` file and print its contents.

First, ensure you have the necessary packages:

```bash
go get github.com/cloudwego/eino
go get github.com/eino-contrib/docx2md
```

### Example Code
```go
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
    file, err := os.Open("your_document.docx")
    if err != nil {
        log.Fatalf("Failed to open file: %v", err)
    }
    defer file.Close()

    ctx := context.Background()

    // 2. Configure the parser to include everything.
    config := &docx.Config{
        ToSections:      true, // Split content into sections
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
```

### Configuration Options
```go
config := &docx.Config{
    ToSections:      false, // Whether to split content by sections
    IncludeHeaders:  true,  // Include headers in output
    IncludeFooters:  true,  // Include footers in output
    IncludeTables:   true,  // Include table content
}

parser, err := docx.NewDocxParser(context.Background(), config)
```

## Output Format
When `ToSections` is `false` (default), the parser returns a single document with all content concatenated.

When `ToSections` is `true`, the parser returns multiple documents split by section type:

+ "main" - Main document content
+ "headers" - Header content (if enabled)
+ "footers" - Footer content (if enabled)
+ "tables" - Table content (if enabled)

Each section is preceded by a header line (e.g., "=== MAIN CONTENT ===") to identify the section type.

## Limitations
+ Currently only extracts plain text content
+ Formatting, images, and other rich content are not preserved
+ Complex table structures may not be perfectly represented
+ Note on Comments: The `IncludeComments` option is now deprecated. The underlying DOCX parsing library was switched from an AGPL-licensed dependency to `docx2md` (MIT-licensed) to resolve a license conflict with the project's Apache 2.0 license. The new library does not currently support comment extraction.
