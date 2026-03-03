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
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/docx2md"
)

const (
	SectionTypeKey = "sectionType"
)

// Config is the configuration for Docx parser.
type Config struct {
	ToSections bool // whether to split content by sections
	// deprecated: Comments are not supported yet.
	IncludeComments bool // whether to include comments in the parsed content
	IncludeHeaders  bool // whether to include headers in the parsed content
	IncludeFooters  bool // whether to include footers in the parsed content
	IncludeTables   bool // whether to include table content
}

// DocxParser reads from io.Reader and parse Docx document content as plain text.
type DocxParser struct {
	toSections bool
	// deprecated: Comments are not supported yet.
	includeComments bool
	includeHeaders  bool
	includeFooters  bool
	includeTables   bool
}

// sectionTitles defines the custom display titles for specific section keys.
var sectionTitles = map[string]string{
	"main": "MAIN CONTENT",
}

// getSectionTitle returns the appropriate display title for a given section key.
// It checks for a custom title in the sectionTitles map first.
// If not found, it defaults to the uppercased version of the key.
func getSectionTitle(key string) string {
	if title, ok := sectionTitles[key]; ok {
		return title
	}
	return strings.ToUpper(key)
}

// NewDocxParser creates a new Docx parser.
func NewDocxParser(_ context.Context, config *Config) (*DocxParser, error) {
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
func (wp *DocxParser) Parse(_ context.Context, reader io.Reader, opts ...parser.Option) (docs []*schema.Document, err error) {
	commonOpts := parser.GetCommonOptions(nil, opts...)

	// Create a temporary file to hold the docx content
	tempFile, err := os.CreateTemp("", "eino-docx-*.docx")
	if err != nil {
		return nil, fmt.Errorf("docx parser failed to create temporary file: %w", err)
	}
	defer func() {
		err := os.Remove(tempFile.Name()) // Clean up
		if err != nil {
			log.Printf("Docx parser failed to remove temporary file: %v", err)
		}
	}()

	// Copy the reader content to the temporary file
	if _, err = io.Copy(tempFile, reader); err != nil {
		return nil, fmt.Errorf("docx parser failed to write to temporary file: %w", err)
	}
	// Close the file so it can be read by the library
	if err = tempFile.Close(); err != nil {
		return nil, fmt.Errorf("docx parser failed to close temporary file: %w", err)
	}

	convertConfig := &docx2md.Config{
		IncludeHeaders: wp.includeHeaders,
		IncludeFooters: wp.includeFooters,
		IncludeTables:  wp.includeTables,
	}

	sections, err := docx2md.DocxConvert(tempFile.Name(), convertConfig)
	if err != nil {
		return nil, fmt.Errorf("open Docx document failed: %w", err)
	}

	// Extract content based on configuration
	if wp.toSections {
		for key, section := range sections {
			content := strings.TrimSpace(section)
			if content != "" {
				content = fmt.Sprintf("=== %s ===\n%s", getSectionTitle(key), content)
				metadata := make(map[string]interface{})
				for k, v := range commonOpts.ExtraMeta {
					metadata[k] = v
				}
				metadata[SectionTypeKey] = key
				docs = append(docs, &schema.Document{
					ID:       uuid.New().String(),
					Content:  content,
					MetaData: metadata,
				})
			}
		}
	} else {
		var contentBuilder strings.Builder
		for key, section := range sections {
			if trimmed := strings.TrimSpace(section); trimmed != "" {
				contentBuilder.WriteString(fmt.Sprintf("=== %s ===\n", getSectionTitle(key)))
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
