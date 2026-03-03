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
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// IDGenerator generates new IDs for split chunks
type IDGenerator func(ctx context.Context, originalID string, splitIndex int) string

// defaultIDGenerator keeps the original ID
func defaultIDGenerator(ctx context.Context, originalID string, _ int) string {
	return originalID
}

// HeaderConfig configures how HTML headers are identified and mapped to metadata keys
type HeaderConfig struct {
	// Headers specify the headers to be identified and their names in document metadata.
	// Header must be in the format of starting with 'h' followed by a number.
	// Example: {"h1": "Title", "h2": "Section"} will track h1 and h2 headers
	Headers map[string]string
	// IDGenerator is an optional function to generate new IDs for split chunks.
	// If nil, the original document ID will be used for all splits.
	IDGenerator IDGenerator
}

// NewHeaderSplitter creates a transformer that splits HTML content based on header tags.
// It tracks header hierarchy and attaches header text as metadata to the resulting chunks.
//
// Example:
//
//	Input HTML:
//	  <h1>Chapter 1</h1>
//	  <p>Introduction text</p>
//	  <h2>Section 1.1</h2>
//	  <p>Section content</p>
//
//	With config Headers: {"h1": "Chapter", "h2": "Section"}
//
//	Will produce two documents:
//	1. {
//	     Content: "Introduction text",
//	     Metadata: {"Chapter": "Chapter 1"}
//	   }
//	2. {
//	     Content: "Section content",
//	     Metadata: {
//	       "Chapter": "Chapter 1",
//	       "Section": "Section 1.1"
//	     }
//	   }
func NewHeaderSplitter(ctx context.Context, config *HeaderConfig) (document.Transformer, error) {
	idGenerator := config.IDGenerator
	if idGenerator == nil {
		idGenerator = defaultIDGenerator
	}
	return &headerSplitter{
		headers:     config.Headers,
		idGenerator: idGenerator,
	}, nil
}

type headerSplitter struct {
	headers     map[string]string
	idGenerator IDGenerator
}

func (h *headerSplitter) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	var ret []*schema.Document
	for _, doc := range docs {
		result, err := h.splitText(ctx, doc.Content)
		if err != nil {
			return nil, err
		}
		for i := range result {
			nDoc := &schema.Document{
				ID:       h.idGenerator(ctx, doc.ID, i),
				Content:  result[i].chunk,
				MetaData: deepCopyAnyMap(doc.MetaData),
			}
			if nDoc.MetaData == nil {
				nDoc.MetaData = make(map[string]any, len(result[i].meta))
			}
			for k, v := range result[i].meta {
				nDoc.MetaData[k] = v
			}
			ret = append(ret, nDoc)
		}
	}
	return ret, nil
}

func (h *headerSplitter) GetType() string {
	return "HTMLHeaderSplitter"
}

type splitResult struct {
	chunk string
	meta  map[string]string
}

type metaRecord struct {
	name  string
	level int
	data  string
}

func (h *headerSplitter) splitText(ctx context.Context, text string) ([]splitResult, error) {
	var recordedMetaList []metaRecord
	recordedMetaMap := make(map[string]string)
	currentText := &strings.Builder{}
	var ret []splitResult

	tree, err := html.Parse(strings.NewReader(text))
	if err != nil {
		return nil, err
	}

	err = h.dfs(tree, recordedMetaList, recordedMetaMap, currentText, &ret)
	if err != nil {
		return nil, err
	}
	if currentText.Len() > 0 {
		ret = append(ret, splitResult{
			chunk: currentText.String(),
			meta:  map[string]string{},
		})
	}
	return ret, nil
}

func (h *headerSplitter) dfs(node *html.Node, recordedMetaList []metaRecord, recordedMetaMap map[string]string, currentText *strings.Builder, ret *[]splitResult) error {
	hasHeader := false
	for ; node != nil; node = node.NextSibling {
		if _, ok := h.headers[node.Data]; ok && node.Type == html.ElementNode {
			hasHeader = true

			if currentText.Len() > 0 {
				*ret = append(*ret, splitResult{
					chunk: currentText.String(),
					meta:  deepCopyMap(recordedMetaMap),
				})
				currentText.Reset()
			}

			newLevel, success := calHLevel(node.Data)
			if !success {
				return fmt.Errorf("calculate header level fail: %s", node.Data)
			}
			for i := len(recordedMetaList) - 1; i >= 0; i-- {
				if recordedMetaList[i].level >= newLevel {
					delete(recordedMetaMap, recordedMetaList[i].name)
					recordedMetaList = recordedMetaList[:i]
				} else {
					break
				}
			}

			data, err := extractText(node)
			if err != nil {
				return err
			}
			record := metaRecord{
				name:  h.headers[node.Data],
				level: newLevel,
				data:  data,
			}
			recordedMetaList = append(recordedMetaList, record)
			if recordedMetaMap == nil {
				return fmt.Errorf("recorded meta map is empty")
			}
			recordedMetaMap[record.name] = record.data
			continue
		}
		if node.Type == html.TextNode && len(strings.TrimSpace(node.Data)) != 0 {
			currentText.WriteString(node.Data)
		}

		err := h.dfs(node.FirstChild, deepCopySlice(recordedMetaList), deepCopyMap(recordedMetaMap), currentText, ret)
		if err != nil {
			return err
		}
	}
	if hasHeader && currentText.Len() > 0 {
		*ret = append(*ret, splitResult{
			chunk: currentText.String(),
			meta:  deepCopyMap(recordedMetaMap),
		})
		currentText.Reset()
	}
	return nil
}

func extractText(node *html.Node) (string, error) {
	sb := strings.Builder{}

	if node == nil {
		return sb.String(), nil
	}
	ll := map[*html.Node]bool{}
	orig := node
	node = node.FirstChild
	for node != nil {
		if node == orig {
			break
		}
		if _, ok := ll[node]; !ok {
			if node.Type == html.TextNode && len(strings.TrimSpace(node.Data)) != 0 {
				sb.WriteString(node.Data)
			}
			ll[node] = true

			if node.FirstChild != nil {
				node = node.FirstChild
				continue
			}
		}
		if node.NextSibling != nil {
			node = node.NextSibling
			continue
		}
		node = node.Parent
	}
	return sb.String(), nil
}

func deepCopySlice(s []metaRecord) []metaRecord {
	ret := make([]metaRecord, len(s))
	copy(ret, s)
	return ret
}

func deepCopyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	ret := make(map[string]string)
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

func deepCopyAnyMap(anyMap map[string]any) map[string]any {
	if anyMap == nil {
		return nil
	}
	ret := make(map[string]any)
	for k, v := range anyMap {
		ret[k] = v
	}
	return ret
}

func calHLevel(h string) (int, bool) {
	num, err := strconv.Atoi(h[1:])
	if err != nil {
		return 0, false
	}
	return num, true
}
