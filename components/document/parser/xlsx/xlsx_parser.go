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

package xlsx

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/xuri/excelize/v2"
)

const (
	MetaDataRow   = "_row"
	MetaDataRowNr = "_row_nr"
)

// XlsxParser Custom parser for parsing Xlsx file content
// Can be used to work with Xlsx files with headers or without headers
// You can also select a specific table from the xlsx file in multiple sheet tables
// You can also customize the prefix of the document ID
type XlsxParser struct {
	Config *Config
}

type Columns struct {
	// NoHeader is set to false by default, which means that the first row is used as the table header
	NoHeader  bool `yaml:"no_header" json:"no_header" jsonschema:"description=Whether the first row is used as the table header,optional"`
	NoRowMeta bool `yaml:"no_row_meta" json:"no_row_meta" jsonschema:"description=Whether to generate _row metadata,optional"`

	Content     []string          `yaml:"content,omitempty" json:"content,omitempty" jsonschema:"description=Content columns,optional"`               // e.g., ["A", "D", "F"]
	Meta        []string          `yaml:"meta,omitempty" json:"meta,omitempty" jsonschema:"description=Meta columns,optional"`                        // e.g., ["B", "C"]
	CustomNames map[string]string `yaml:"custom_names,omitempty" json:"custom_names,omitempty" jsonschema:"description=Custom column names,optional"` // e.g., {"A": "Name", "B": "Age"}
}

// Config Used to configure xlsxParser
type Config struct {
	// SheetName is set to Sheet1 by default, which means that the first table is processed
	SheetName string `yaml:"sheet_name,omitempty" json:"sheet_name,omitempty" jsonschema:"description=Sheet name to be processed,default is the first sheet,optional"`
	// IDPrefix is set to customize the prefix of document ID, default 1,2,3, ...
	IDPrefix string `yaml:"id_prefix,omitempty" json:"id_prefix,omitempty" jsonschema:"description=Custom prefix for document ID,default is 1,2,3, ... ,optional"`

	Columns Columns `yaml:"columns" json:"columns" jsonschema:"description=Columns to be processed,if not set,all columns will be processed"`
}

// implOptions is used to extract the config from the generic parser.Option
type implOptions struct {
	Config *Config
}

// WithConfig specifies the xlsx parser config
func WithConfig(config *Config) parser.Option {
	return parser.WrapImplSpecificOptFn(func(o *implOptions) {
		o.Config = config
	})
}

// NewXlsxParser Create a new xlsxParser
func NewXlsxParser(ctx context.Context, config *Config) (xlp parser.Parser, err error) {
	// Default configuration
	if config == nil {
		config = &Config{}
	}
	// NoHeader is false by default, which means HasHeader is true by default
	xlp = &XlsxParser{Config: config}
	return xlp, nil
}

// columnLetterToIndex converts a column letter (A, B, C, ..., Z, AA, AB, ...) to a 0-based index
func columnLetterToIndex(letter string) int {
	letter = strings.ToUpper(letter)
	index := 0
	for i := 0; i < len(letter); i++ {
		index = index*26 + int(letter[i]-'A'+1)
	}
	return index - 1
}

// Parse parses the XLSX content from io.Reader.
func (xlp *XlsxParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) ([]*schema.Document, error) {

	// Extract implementation-specific options
	config := parser.GetImplSpecificOptions(&implOptions{}, opts...).Config

	// Use config from options if provided, otherwise use default from parser instance
	if config == nil {
		config = xlp.Config
	}

	// Return error if no config is available
	if config == nil {
		return nil, fmt.Errorf("xlsx parser config not provided in options and no default config available")
	}

	xlFile, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, err
	}
	defer xlFile.Close()

	// Get all worksheets
	sheets := xlFile.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil
	}

	// Default
	sheetName := sheets[0]
	if config.SheetName != "" {
		sheetName = config.SheetName
	}

	// Get all rows, header + data rows
	rows, err := xlFile.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	var ret []*schema.Document

	// Process the header
	startIdx := 0
	var headers []string
	if !config.Columns.NoHeader && len(rows) > 0 {
		headers = rows[0]
		startIdx = 1
	}

	// Process rows of data
	for i := startIdx; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		// Build content string based on Columns.Content if specified
		var contentParts []string
		if len(config.Columns.Content) > 0 {
			// Only use specified columns for content
			contentParts = make([]string, 0, len(config.Columns.Content))
			for _, colLetter := range config.Columns.Content {
				colIndex := columnLetterToIndex(colLetter)
				if colIndex < len(row) {
					contentParts = append(contentParts, strings.TrimSpace(row[colIndex]))
				} else {
					contentParts = append(contentParts, "")
				}
			}
		} else {
			// Use all columns for content
			contentParts = make([]string, len(row))
			for j, cell := range row {
				contentParts[j] = strings.TrimSpace(cell)
			}
		}
		content := strings.Join(contentParts, "\t")

		meta := make(map[string]any)

		// Only add row metadata if NoRowMeta is false
		if !config.Columns.NoRowMeta {
			meta[MetaDataRow] = buildAllColumnsMetaData(row, headers, config.Columns.NoHeader)
		}

		// Add Meta columns directly to the document's MetaData (not inside _row)
		if len(config.Columns.Meta) > 0 {
			for _, colLetter := range config.Columns.Meta {
				colIndex := columnLetterToIndex(colLetter)
				if colIndex < len(row) {
					// Determine the key name - use custom name if available
					keyName := colLetter
					if !config.Columns.NoHeader && colIndex < len(headers) {
						keyName = headers[colIndex]
					}
					if customName, ok := config.Columns.CustomNames[colLetter]; ok {
						keyName = customName
					}
					meta[keyName] = row[colIndex]
				}
			}
		}

		nr := i + 1
		meta[MetaDataRowNr] = nr

		// Create New Document
		nDoc := &schema.Document{
			ID:       generateID(config, nr),
			Content:  content,
			MetaData: meta,
		}

		ret = append(ret, nDoc)
	}

	return ret, nil
}

// buildAllColumnsMetaData builds metadata containing all columns using header names or A,B,C if NoHeader is true
func buildAllColumnsMetaData(row []string, headers []string, noHeader bool) map[string]any {
	metaData := make(map[string]any)

	// For test files that expect C column in Sheet3
	maxCols := len(row)

	for j := 0; j < maxCols; j++ {
		var keyName string
		colLetter := indexToColumnLetter(j)

		if !noHeader && j < len(headers) {
			// Use header name as key
			keyName = headers[j]
		} else {
			// Use column letter as key (A, B, C, ...)
			keyName = colLetter
		}

		// Only add value if we have data for this column
		if j < len(row) {
			metaData[keyName] = row[j]
		} else {
			// Add empty string for columns we need to include but don't have data for
			metaData[keyName] = ""
		}
	}

	return metaData
}

// generateID generates document ID based on configuration - extracted from the XlsxParser method
func generateID(config *Config, i int) string {
	if config.IDPrefix == "" {
		return fmt.Sprintf("%d", i)
	}
	return fmt.Sprintf("%s%d", config.IDPrefix, i)
}

// indexToColumnLetter converts a 0-based index to a column letter (A, B, C, ..., Z, AA, AB, ...)
func indexToColumnLetter(index int) string {
	var result string
	for index >= 0 {
		remainder := index % 26
		result = string(rune('A'+remainder)) + result
		index = index/26 - 1
	}
	return result
}
