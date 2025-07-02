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
	"io"
	"os"
	"strings"
)

// Config is the configuration for Docx parser.
type Config struct {
	ToPages         bool // whether to split content by pages (not applicable for Docx, will be treated as sections)
	IncludeComments bool // whether to include comments in the parsed content
	IncludeHeaders  bool // whether to include headers in the parsed content
	IncludeFooters  bool // whether to include footers in the parsed content
	IncludeTables   bool // whether to include table content
}

// options represents the specific options for Docx parser.
type options struct {
	toPages         *bool
	includeComments *bool
	includeHeaders  *bool
	includeFooters  *bool
	includeTables   *bool
}

// DocxParser reads from io.Reader and parse Docx document content as plain text.
type DocxParser struct {
	ToPages         bool
	IncludeComments bool
	IncludeHeaders  bool
	IncludeFooters  bool
	IncludeTables   bool
}

// NewDocxParser creates a new Docx parser.
func NewDocxParser(ctx context.Context, config *Config) (*DocxParser, error) {
	if config == nil {
		config = &Config{
			IncludeComments: true,
			IncludeHeaders:  true,
			IncludeFooters:  true,
			IncludeTables:   true,
		}
	}
	return &DocxParser{
		ToPages:         config.ToPages,
		IncludeComments: config.IncludeComments,
		IncludeHeaders:  config.IncludeHeaders,
		IncludeFooters:  config.IncludeFooters,
		IncludeTables:   config.IncludeTables,
	}, nil
}

// Parse parses the Docx document content from io.Reader.
func (wp *DocxParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) (docs []*schema.Document, err error) {
	commonOpts := parser.GetCommonOptions(nil, opts...)

	specificOpts := parser.GetImplSpecificOptions(&options{
		toPages:         &wp.ToPages,
		includeComments: &wp.IncludeComments,
		includeHeaders:  &wp.IncludeHeaders,
		includeFooters:  &wp.IncludeFooters,
		includeTables:   &wp.IncludeTables,
	}, opts...)

	// Read all data from reader
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("Docx parser read all from reader failed: %w", err)
	}

	// Create a temporary file to save the Docx document
	// because gooxml requires a file path
	tmpFile, err := os.CreateTemp("", "docx_parser_*.docx")
	if err != nil {
		return nil, fmt.Errorf("create temp file failed: %w", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {

		}
	}(tmpFile.Name()) // Clean up temp file
	defer func(tmpFile *os.File) {
		err := tmpFile.Close()
		if err != nil {

		}
	}(tmpFile)

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		return nil, fmt.Errorf("write to temp file failed: %w", err)
	}

	// Close the file to ensure all data is written
	err = tmpFile.Close()
	if err != nil {
		return nil, err
	}

	// Open the Docx document
	doc, err := document.Open(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("open Docx document failed: %w", err)
	}

	var toPages = specificOpts.toPages != nil && *specificOpts.toPages

	// Extract content based on configuration
	sections := wp.extractContent(doc, specificOpts)
	if toPages {
		for _, section := range sections {
			if strings.TrimSpace(section) != "" {
				docs = append(docs, &schema.Document{
					Content:  strings.TrimSpace(section),
					MetaData: commonOpts.ExtraMeta,
				})
			}
		}
	} else {
		var content string
		for _, section := range sections {
			if strings.TrimSpace(section) != "" {
				content += strings.TrimSpace(section) + "\n"
			}
		}
		docs = append(docs, &schema.Document{
			Content:  content,
			MetaData: commonOpts.ExtraMeta,
		})

	}

	return docs, nil
}

// extractContent extracts all content from the Docx document based on configuration.
func (wp *DocxParser) extractContent(doc *document.Document, opts *options) map[string]string {
	sections := make(map[string]string)

	includeComments := opts.includeComments != nil && *opts.includeComments
	includeHeaders := opts.includeHeaders != nil && *opts.includeHeaders
	includeFooters := opts.includeFooters != nil && *opts.includeFooters
	includeTables := opts.includeTables != nil && *opts.includeTables

	// Extract main document content
	var mainContentBuf bytes.Buffer
	mainContentBuf.WriteString("=== MAIN CONTENT ===\n")
	mainContent := wp.extractMainContent(doc)
	mainContentBuf.WriteString(mainContent)
	mainContentBuf.WriteString("\n")
	sections["main"] = mainContentBuf.String()

	// Extract comments if enabled
	if includeComments {
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
	if includeHeaders {
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
	if includeTables {
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
	if includeFooters {
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

// extractTables extracts table content from the Docx document.
func (wp *DocxParser) extractTables(doc *document.Document) string {
	var buf bytes.Buffer

	for tableIdx, table := range doc.Tables() {
		buf.WriteString(fmt.Sprintf("Table %d:\n", tableIdx+1))
		for rowIdx, row := range table.Rows() {
			buf.WriteString(fmt.Sprintf("Row %d: ", rowIdx+1))
			for cellIdx, cell := range row.Cells() {
				var text string
				for _, para := range cell.Paragraphs() {
					for _, run := range para.Runs() {
						text += run.Text()
					}
				}
				if len(text) > 0 {
					buf.WriteString(fmt.Sprintf("Cell %d: %s | ", cellIdx+1, text))
				}
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}
