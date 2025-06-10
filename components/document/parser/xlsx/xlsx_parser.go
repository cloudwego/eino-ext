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
	MetaDataRow = "_row"
	MetaDataExt = "_ext"
)

// XlsxParser Custom parser for parsing Xlsx file content
// Can be used to work with Xlsx files with headers or without headers
// You can also select a specific table from the xlsx file in multiple sheet tables
// You can also customize the prefix of the document ID
type XlsxParser struct {
	Config *Config
}

// Config Used to configure xlsxParser
type Config struct {
	// SheetName is set to Sheet1 by default, which means that the first table is processed
	SheetName string
	// NoHeader is set to false by default, which means that the first row is used as the table header
	NoHeader bool
	// IDPrefix is set to customize the prefix of document ID, default 1,2,3, ...
	IDPrefix string
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

// generateID generates document ID based on configuration
func (xlp *XlsxParser) generateID(i int) string {
	if xlp.Config.IDPrefix == "" {
		return fmt.Sprintf("%d", i)
	}
	return fmt.Sprintf("%s%d", xlp.Config.IDPrefix, i)
}

// buildRowMetaData builds row metadata from row data and headers
func (xlp *XlsxParser) buildRowMetaData(row []string, headers []string) map[string]any {
	metaData := make(map[string]any)
	if !xlp.Config.NoHeader {
		for j, header := range headers {
			if j < len(row) {
				metaData[header] = row[j]
			}
		}
	}
	return metaData
}

// Parse parses the XLSX content from io.Reader.
func (xlp *XlsxParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) ([]*schema.Document, error) {
	option := parser.GetCommonOptions(&parser.Options{}, opts...)
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
	if xlp.Config.SheetName != "" {
		sheetName = xlp.Config.SheetName
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
	if !xlp.Config.NoHeader && len(rows) > 0 {
		headers = rows[0]
		startIdx = 1
	}

	// Process rows of data
	for i := startIdx; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}
		// Convert row data to strings
		contentParts := make([]string, len(row))
		for j, cell := range row {
			contentParts[j] = strings.TrimSpace(cell)
		}
		content := strings.Join(contentParts, "\t")

		meta := make(map[string]any)

		// Build the row's Meta
		rowMeta := xlp.buildRowMetaData(row, headers)
		meta[MetaDataRow] = rowMeta

		// Get the Common ExtraMeta
		if option.ExtraMeta != nil {
			meta[MetaDataExt] = option.ExtraMeta
		}

		// Create New Document
		nDoc := &schema.Document{
			ID:       xlp.generateID(i),
			Content:  content,
			MetaData: meta,
		}

		ret = append(ret, nDoc)
	}

	return ret, nil
}
