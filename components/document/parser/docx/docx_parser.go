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

package docx

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"github.com/carmel/gooxml/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"io"
	"os"
	"strings"
)

const (
	SectionTypeKey = "sectionType"
)

// Config is the configuration for Docx parser.
type Config struct {
	ToSections      bool // whether to split content by sections
	IncludeComments bool // whether to include comments in the parsed content
	IncludeHeaders  bool // whether to include headers in the parsed content
	IncludeFooters  bool // whether to include footers in the parsed content
	IncludeTables   bool // whether to include table content
}

// DocxParser reads from io.Reader and parse Docx document content as plain text.
type DocxParser struct {
	toSections      bool
	includeComments bool
	includeHeaders  bool
	includeFooters  bool
	includeTables   bool
}

// NewDocxParser creates a new Docx parser.
func NewDocxParser(ctx context.Context, config *Config) (*DocxParser, error) {
	if config == nil {
		config = &Config{}
	}
	return &DocxParser{
		toSections:      config.ToSections,
		includeComments: config.IncludeComments,
		includeHeaders:  config.IncludeHeaders,
		includeFooters:  config.IncludeFooters,
		includeTables:   config.IncludeTables,
	}, nil
}

// Parse parses the Docx document content from io.Reader.
func (wp *DocxParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) (docs []*schema.Document, err error) {
	commonOpts := parser.GetCommonOptions(nil, opts...)

	// Read all data from reader
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("Docx parser read all from reader failed: %w", err)
	}

	// Open the Docx document from memory
	doc, err := document.Read(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open Docx document failed: %w", err)
	}

	// Extract content based on configuration
	sections := wp.extractContent(doc)
	if wp.toSections {
		for key, section := range sections {
			content := strings.TrimSpace(section)
			metadata := make(map[string]interface{})
			for k, v := range commonOpts.ExtraMeta {
				metadata[k] = v
			}
			metadata[SectionTypeKey] = key
			if content != "" {
				docs = append(docs, &schema.Document{
					ID:       uuid.New().String(),
					Content:  content,
					MetaData: metadata,
				})
			}
		}
	} else {
		var contentBuilder strings.Builder
		for _, section := range sections {
			if trimmed := strings.TrimSpace(section); trimmed != "" {
				contentBuilder.WriteString(trimmed)
				contentBuilder.WriteString("\n")
			}
		}
		content := contentBuilder.String()
		metadata := make(map[string]interface{})
		for k, v := range commonOpts.ExtraMeta {
			metadata[k] = v
		}
		metadata[SectionTypeKey] = "fullContent"
		if content != "" {
			docs = append(docs, &schema.Document{
				ID:       uuid.New().String(),
				Content:  content,
				MetaData: metadata,
			})
		}
	}

	return docs, nil
}

func GetSectionType(doc *schema.Document) (string, bool) {
	if doc == nil {
		return "", false
	}
	sectionType, ok := doc.MetaData[SectionTypeKey].(string)
	return sectionType, ok
}

// extractContent extracts all content from the Docx document based on configuration.
func (wp *DocxParser) extractContent(doc *document.Document) map[string]string {
	sections := make(map[string]string)

	// Extract main document content
	var mainContentBuf bytes.Buffer
	mainContentBuf.WriteString("=== MAIN CONTENT ===\n")
	mainContent := wp.extractMainContent(doc)
	mainContentBuf.WriteString(mainContent)
	mainContentBuf.WriteString("\n")
	sections["main"] = mainContentBuf.String()

	// Extract comments if enabled
	if wp.includeComments {
		comments := wp.extractComments(doc)
		if comments != "" {
			var commentBuf bytes.Buffer
			commentBuf.WriteString("=== COMMENTS ===\n")
			commentBuf.WriteString(comments)
			commentBuf.WriteString("\n")
			sections["comments"] = commentBuf.String()
		}
	}

	// Extract headers if enabled
	if wp.includeHeaders {
		headers := wp.extractHeaders(doc)
		if headers != "" {
			var headerBuf bytes.Buffer
			headerBuf.WriteString("=== HEADERS ===\n")
			headerBuf.WriteString(headers)
			headerBuf.WriteString("\n")
			sections["headers"] = headerBuf.String()
		}
	}

	// Extract table content if enabled
	if wp.includeTables {
		tables := wp.extractTables(doc)
		if tables != "" {
			var tableBuf bytes.Buffer
			tableBuf.WriteString("=== TABLES ===\n")
			tableBuf.WriteString(tables)
			tableBuf.WriteString("\n")
			sections["tables"] = tableBuf.String()
		}
	}

	// Extract footers if enabled
	if wp.includeFooters {
		footers := wp.extractFooters(doc)
		if footers != "" {
			var footerBuf bytes.Buffer
			footerBuf.WriteString("=== FOOTERS ===\n")
			footerBuf.WriteString(footers)
			footerBuf.WriteString("\n")
			sections["footers"] = footerBuf.String()
		}
	}

	return sections
}

// extractComments extracts comments from the Docx document.
func (wp *DocxParser) extractComments(doc *document.Document) string {
	var buf bytes.Buffer

	for _, docfile := range doc.DocBase.ExtraFiles {
		if docfile.ZipPath != "word/comments.xml" {
			continue
		}

		file, err := os.Open(docfile.DiskPath)
		if err != nil {
			continue
		}
		defer file.Close()

		decoder := xml.NewDecoder(file)

		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}

			if startElement, ok := token.(xml.StartElement); ok {
				if startElement.Name.Local == "t" {
					innerText, err := decoder.Token()
					if err != nil {
						break
					}

					if charData, ok := innerText.(xml.CharData); ok {
						text := string(charData)
						if text != "" {
							buf.WriteString(text)
							buf.WriteString("\n")
						}
					}
				}
			}
		}
	}

	return buf.String()
}

// extractHeaders extracts headers from the Docx document.
func (wp *DocxParser) extractHeaders(doc *document.Document) string {
	var buf bytes.Buffer

	for _, head := range doc.Headers() {
		var text string
		for _, para := range head.Paragraphs() {
			for _, run := range para.Runs() {
				text += run.Text()
			}
		}
		if len(text) > 0 {
			buf.WriteString(text)
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

// extractFooters extracts footers from the Docx document.
func (wp *DocxParser) extractFooters(doc *document.Document) string {
	var buf bytes.Buffer

	for _, footer := range doc.Footers() {
		for _, para := range footer.Paragraphs() {
			var text string
			for _, run := range para.Runs() {
				text += run.Text()
			}
			if len(text) > 0 {
				buf.WriteString(text)
				buf.WriteString("\n")
			}
		}
	}

	return buf.String()
}

// extractMainContent extracts the main document content.
func (wp *DocxParser) extractMainContent(doc *document.Document) string {
	var buf bytes.Buffer

	// Extract paragraphs
	for _, para := range doc.Paragraphs() {
		var text string
		for _, run := range para.Runs() {
			text += run.Text()
		}
		if len(text) > 0 {
			buf.WriteString(text)
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

// extractTables extracts table content from the Docx document in Markdown format.
func (wp *DocxParser) extractTables(doc *document.Document) string {
	var buf bytes.Buffer

	for tableIdx, table := range doc.Tables() {
		buf.WriteString(fmt.Sprintf("### Table %d\n\n", tableIdx+1))

		rows := table.Rows()
		if len(rows) == 0 {
			continue
		}

		// Process each row
		for rowIdx, row := range rows {
			cells := row.Cells()
			if len(cells) == 0 {
				continue
			}

			// Extract cell content
			var cellContents []string
			for _, cell := range cells {
				var cellText string
				for _, para := range cell.Paragraphs() {
					for _, run := range para.Runs() {
						cellText += run.Text()
					}
				}
				// Clean up cell text - remove newlines and trim spaces
				cellText = strings.ReplaceAll(strings.TrimSpace(cellText), "\n", " ")
				if cellText == "" {
					cellText = " " // Empty cell placeholder
				}
				cellContents = append(cellContents, cellText)
			}

			// Write table row
			buf.WriteString("| ")
			buf.WriteString(strings.Join(cellContents, " | "))
			buf.WriteString(" |\n")

			// Add separator row after first row (header)
			if rowIdx == 0 {
				buf.WriteString("|")
				for range cellContents {
					buf.WriteString(" --- |")
				}
				buf.WriteString("\n")
			}
		}

		buf.WriteString("\n") // Add spacing between tables
	}

	return buf.String()
}
