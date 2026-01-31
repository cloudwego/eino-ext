/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"context"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

const (
	MetaDataRow = "_row"
	MetaDataExt = "_ext"
)

// CSVParser parses CSV content from io.Reader.
type CSVParser struct {
	Config *Config
}

// Config Used to configure CsvParser.
type Config struct {
	// NoHeader is set to false by default, which means that the first row is used as the table header
	NoHeader bool
	// IDPrefix is set to customize the prefix of document ID, default 1,2,3, ...
	IDPrefix string
	// Comma is set to ',' by default, which means that the comma is used as the field delimiter
	Comma rune
	// Comment is set to '#' by default, which means that the '#' character is used as the comment character
	Comment rune
}

// NewCSVParser creates a new CSVParser
func NewCSVParser(ctx context.Context, config *Config) (cp *CSVParser, err error) {
	if config == nil {
		config = &Config{}

	}
	if config.Comma == 0 {
		config.Comma = rune(',')
	}
	if config.Comment == 0 {
		config.Comment = rune('#')
	}

	cp = &CSVParser{Config: config}
	return cp, nil
}

// generateID generates document ID based on configuration
func (cp *CSVParser) generateID(i int) string {
	if cp.Config.IDPrefix == "" {
		return fmt.Sprintf("%d", i)
	}
	return fmt.Sprintf("%s%d", cp.Config.IDPrefix, i)
}

func (cp *CSVParser) buildRowMetaData(row []string, headers []string) map[string]any {
	metaData := make(map[string]any)
	if !cp.Config.NoHeader {
		for j, header := range headers {
			if j < len(row) {
				metaData[header] = row[j]
			}
		}
	}
	return metaData
}

func (cp *CSVParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) ([]*schema.Document, error) {
	option := parser.GetCommonOptions(&parser.Options{}, opts...)

	csvFile := csv.NewReader(reader)

	// get all rows
	rows, err := csvFile.ReadAll()
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
	if !cp.Config.NoHeader && len(rows) > 0 {
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
		content := strings.Join(contentParts, string(cp.Config.Comma))

		meta := make(map[string]any)

		// Build the row's Meta
		rowMeta := cp.buildRowMetaData(row, headers)
		meta[MetaDataRow] = rowMeta

		// Get the Common ExtraMeta
		if option.ExtraMeta != nil {
			meta[MetaDataExt] = option.ExtraMeta
		}

		// Create New Document
		nDoc := &schema.Document{
			ID:       cp.generateID(i),
			Content:  content,
			MetaData: meta,
		}

		ret = append(ret, nDoc)
	}

	return ret, nil
}
