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

package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/dslipak/pdf"
)

// Config is the configuration for PDF parser.
type Config struct {
	ToPages bool // whether to
}

// PDFParser reads from io.Reader and parse its content as plain text.
// Attention: This is in alpha stage, and may not support all PDF use cases well enough.
// For example, it will not preserve whitespace and new line for now.
type PDFParser struct {
	ToPages bool
}

// NewPDFParser creates a new PDF parser.
func NewPDFParser(ctx context.Context, config *Config) (*PDFParser, error) {
	if config == nil {
		config = &Config{}
	}
	return &PDFParser{ToPages: config.ToPages}, nil
}

// Parse parses the PDF content from io.Reader.
func (pp *PDFParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) (docs []*schema.Document, err error) {
	commonOpts := parser.GetCommonOptions(nil, opts...)

	specificOpts := parser.GetImplSpecificOptions(&options{
		toPages: &pp.ToPages,
	}, opts...)

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("pdf parser read all from reader failed: %w", err)
	}

	readerAt := bytes.NewReader(data)

	f, err := pdf.NewReader(readerAt, int64(readerAt.Len()))
	if err != nil {
		return nil, fmt.Errorf("create new pdf reader failed: %w", err)
	}

	pages := f.NumPage()
	var (
		buf     bytes.Buffer
		toPages = specificOpts.toPages != nil && *specificOpts.toPages
	)
	fonts := make(map[string]*pdf.Font)
	for i := 1; i <= pages; i++ {
		p := f.Page(i)
		for _, name := range p.Fonts() { // cache fonts so we don't continually parse charmap
			if _, ok := fonts[name]; !ok {
				font := p.Font(name)
				fonts[name] = &font
			}
		}
		text, err := p.GetPlainText(fonts)
		if err != nil {
			return nil, fmt.Errorf("read pdf page failed: %w, page= %d", err, i)
		}

		if toPages {
			docs = append(docs, &schema.Document{
				Content:  text,
				MetaData: commonOpts.ExtraMeta,
			})
		} else {
			buf.WriteString(text + "\n")
		}
	}

	if !toPages {
		docs = append(docs, &schema.Document{
			Content:  buf.String(),
			MetaData: commonOpts.ExtraMeta,
		})
	}

	return docs, nil
}
