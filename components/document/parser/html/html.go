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

package html

import (
	"context"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

const (
	MetaKeyTitle   = "_title"
	MetaKeyDesc    = "_description"
	MetaKeyLang    = "_language"
	MetaKeyCharset = "_charset"
	MetaKeySource  = "_source"
)

var _ parser.Parser = (*Parser)(nil)

type Config struct {
	// content selector of goquery. eg: body for <body>, #id for <div id="id">
	Selector *string
}

var (
	BodySelector = "body"
)

// NewParser returns a new parser.
func NewParser(ctx context.Context, conf *Config) (*Parser, error) {
	if conf == nil {
		conf = &Config{}
	}

	return &Parser{
		conf: conf,
	}, nil
}

// Parser implements parser.Parser. It parses HTML content to text.
// use goquery to parse the HTML content, will read the <body> content as text (remove tags).
// will extract title/description/language/charset from the HTML content as meta data.
type Parser struct {
	conf *Config
}

func (p *Parser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) ([]*schema.Document, error) {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	option := parser.GetCommonOptions(&parser.Options{}, opts...)

	var contentSel *goquery.Selection

	if p.conf.Selector != nil {
		contentSel = doc.Find(*p.conf.Selector).Contents()
	} else {
		contentSel = doc.Contents()
	}

	meta, err := p.getMetaData(ctx, doc)
	if err != nil {
		return nil, err
	}
	meta[MetaKeySource] = option.URI

	if option.ExtraMeta != nil {
		for k, v := range option.ExtraMeta {
			meta[k] = v
		}
	}

	sanitized := bluemonday.UGCPolicy().Sanitize(contentSel.Text())
	content := strings.TrimSpace(sanitized)

	document := &schema.Document{
		Content:  content,
		MetaData: meta,
	}

	return []*schema.Document{
		document,
	}, nil
}

func (p *Parser) getMetaData(ctx context.Context, doc *goquery.Document) (map[string]any, error) {
	meta := map[string]any{}

	title := doc.Find("title")
	if title != nil {
		if t := title.Text(); t != "" {
			meta[MetaKeyTitle] = t
		}
	}

	description := doc.Find("meta[name=description]")
	if description != nil {
		if desc := description.AttrOr("content", ""); desc != "" {
			meta[MetaKeyDesc] = desc
		}
	}

	html := doc.Find("html")
	if html != nil {
		if language := html.AttrOr("lang", ""); language != "" {
			meta[MetaKeyLang] = language
		}
	}

	charset := doc.Find("meta[charset]")
	if charset != nil {
		if c := charset.AttrOr("charset", ""); c != "" {
			meta[MetaKeyCharset] = c
		}
	}

	return meta, nil
}
