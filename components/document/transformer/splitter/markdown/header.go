package markdown

import (
	"context"
	"fmt"
	"strings"

	"code.byted.org/flow/eino/components/document"
	"code.byted.org/flow/eino/schema"
)

type HeaderConfig struct {
	// Headers specify the headers to be identified and their names in document metadata. Headers can only consist of '#'.
	Headers map[string]string
	// TrimHeaders specify if results contain header lines.
	TrimHeaders bool
}

func NewHeaderSplitter(ctx context.Context, config *HeaderConfig) (document.Transformer, error) {
	if len(config.Headers) == 0 {
		return nil, fmt.Errorf("no headers specified")
	}
	for k := range config.Headers {
		for _, c := range k {
			if c != '#' {
				return nil, fmt.Errorf("header can only consist of '#': %s", k)
			}
		}
	}

	return &headerSplitter{
		headers:     config.Headers,
		trimHeaders: config.TrimHeaders,
	}, nil
}

type headerSplitter struct {
	headers     map[string]string
	trimHeaders bool
}

type splitResult struct {
	chunk string
	meta  map[string]string
}

func (h *headerSplitter) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	var ret []*schema.Document
	for _, doc := range docs {
		result := h.splitText(ctx, doc.Content)
		for i := range result {
			nDoc := &schema.Document{
				ID:       doc.ID,
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

const (
	codeSep1 = "```"
	codeSep2 = "~~~"
)

type metaRecord struct {
	name  string
	level int
	data  string
}

func (h *headerSplitter) splitText(ctx context.Context, text string) []splitResult {
	var recordedMetaList []metaRecord
	recordedMetaMap := make(map[string]string)
	var currentLines []string
	var bInCodeBlock bool
	var openingFence string
	var ret []splitResult
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		line = strings.TrimSpace(line)
		if !bInCodeBlock {
			if strings.HasPrefix(line, codeSep1) && strings.Count(line, codeSep1) == 1 {
				bInCodeBlock = true
				openingFence = codeSep1
			} else if strings.HasPrefix(line, codeSep2) {
				bInCodeBlock = true
				openingFence = codeSep2
			}
		} else {
			if strings.HasPrefix(line, openingFence) {
				bInCodeBlock = false
				openingFence = ""
			}
		}
		if bInCodeBlock {
			currentLines = append(currentLines, line)
			continue
		}
		// check if the line starts with headers
		bNewHeader := false
		for header, name := range h.headers {
			if strings.HasPrefix(line, header) && (len(line) == len(header) || line[len(header)] == ' ') {
				if len(currentLines) > 0 {
					ret = append(ret, splitResult{
						chunk: strings.Join(currentLines, "\n"),
						meta:  deepCopyMap(recordedMetaMap),
					})
					currentLines = currentLines[:0]
				}

				if !h.trimHeaders {
					currentLines = append(currentLines, line)
				}

				newLevel := len(header)
				for i := len(recordedMetaList) - 1; i >= 0; i-- {
					if recordedMetaList[i].level >= newLevel {
						delete(recordedMetaMap, recordedMetaList[i].name)
						recordedMetaList = recordedMetaList[:i]
					} else {
						break
					}
				}

				data := strings.TrimSpace(line[len(header):])
				recordedMetaList = append(recordedMetaList, metaRecord{
					name:  name,
					level: newLevel,
					data:  data,
				})
				recordedMetaMap[name] = data

				bNewHeader = true
				break
			}
		}
		if !bNewHeader {
			currentLines = append(currentLines, line)
		}
	}
	ret = append(ret, splitResult{
		chunk: strings.Join(currentLines, "\n"),
		meta:  deepCopyMap(recordedMetaMap),
	})
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