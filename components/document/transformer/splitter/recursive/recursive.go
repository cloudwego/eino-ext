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

package recursive

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

type KeepType uint8

const (
	// KeepTypeNone specifies that each chunk will discard separator.
	KeepTypeNone KeepType = iota
	// KeepTypeStart specifies that each chunk will keep the separator at start.
	KeepTypeStart
	// KeepTypeEnd specifies that each chunk will keep the separator at end.
	KeepTypeEnd
)

type Config struct {
	ChunkSize int
	// OverlapSize is the maximum allowed overlapping length between chunks. Overlapping can mitigate loss of information when context is divided.
	OverlapSize int
	// Separators are sequentially used to split text.
	// When the current separator cannot split the text into a size smaller than ChunkSize, the next separator will be used to attempt to split until the chunk size is smaller than ChunkSize or there are no separator available.
	// ["\n", ".", "?", "!"] by default.
	Separators []string
	// LenFunc is used to calculate string length. Use builtin function len() by default.
	LenFunc func(string) int
	// KeepType specifies if separator will be kept in split chunks. Discard separator by default.
	KeepType KeepType
}

// NewSplitter create a recursive splitter.
func NewSplitter(ctx context.Context, config *Config) (document.Transformer, error) {
	if config.ChunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be greater than zero")
	}
	if config.OverlapSize < 0 {
		return nil, fmt.Errorf("overlap must be greater than or equal to zero")
	}

	lenFunc := config.LenFunc
	if lenFunc == nil {
		lenFunc = func(s string) int { return len(s) }
	}
	seps := config.Separators
	if len(seps) == 0 {
		seps = []string{"\n", ".", "?", "!"}
	}

	return &splitter{
		lenFunc:    lenFunc,
		chunkSize:  config.ChunkSize,
		overlap:    config.OverlapSize,
		separators: seps,
		keepType:   config.KeepType,
	}, nil
}

type splitter struct {
	lenFunc    func(string) int
	chunkSize  int
	overlap    int
	separators []string
	keepType   KeepType
}

func (s *splitter) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	ret := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		splits := s.splitText(ctx, doc.Content, s.separators)
		for _, split := range splits {
			ret = append(ret, &schema.Document{
				ID:       doc.ID,
				Content:  split,
				MetaData: deepCopyMap(doc.MetaData),
			})
		}
	}
	return ret, nil
}

func (s *splitter) splitText(ctx context.Context, text string, separators []string) (output []string) {
	finalChunks := make([]string, 0)

	// find the appropriate separator
	separator := separators[len(separators)-1]
	var newSeparators []string
	for i, c := range separators {
		if c == "" || strings.Contains(text, c) {
			separator = c
			newSeparators = separators[i+1:]
			break
		}
	}

	splits := s.split(text, separator, s.keepType)
	goodSplits := make([]string, 0)

	// merge the splits, recursively splitting larger texts.
	for _, split := range splits {
		if s.lenFunc(split) < s.chunkSize {
			goodSplits = append(goodSplits, split)
			continue
		}

		if len(goodSplits) > 0 {
			mergedText := s.mergeSplits(goodSplits, separator, s.chunkSize, s.lenFunc, s.keepType)

			finalChunks = append(finalChunks, mergedText...)
			goodSplits = make([]string, 0)
		}

		if len(newSeparators) == 0 {
			finalChunks = append(finalChunks, split)
		} else {
			otherInfo := s.splitText(ctx, split, newSeparators)
			finalChunks = append(finalChunks, otherInfo...)
		}
	}

	if len(goodSplits) > 0 {
		mergedText := s.mergeSplits(goodSplits, separator, s.chunkSize, s.lenFunc, s.keepType)
		finalChunks = append(finalChunks, mergedText...)
	}

	return finalChunks
}

func (s *splitter) split(text string, separator string, t KeepType) []string {
	switch t {
	case KeepTypeNone:
		return strings.Split(text, separator)
	case KeepTypeEnd:
		return strings.SplitAfter(text, separator)
	case KeepTypeStart:
		splits := strings.Split(text, separator)
		for i := 1; i < len(splits); i++ {
			splits[i] = separator + splits[i]
		}
		return splits
	default:
		panic(fmt.Sprintf("unknown keep type: %v", t))
	}
}

// mergeSplits merges smaller splits into splits that are closer to the chunkSize.
func (s *splitter) mergeSplits(splits []string, separator string, chunkSize int, lenFunc func(string) int, t KeepType) []string {
	docs := make([]string, 0)
	currentDoc := make([]string, 0)
	total := 0

	for _, split := range splits {
		totalWithSplit := total + lenFunc(split)
		if len(currentDoc) != 0 && t == KeepTypeNone {
			totalWithSplit += lenFunc(separator)
		}

		if totalWithSplit > chunkSize && len(currentDoc) > 0 {
			doc := joinDocs(currentDoc, separator, t)
			if doc != "" {
				docs = append(docs, doc)
			}

			for s.shouldPop(total, lenFunc(split), lenFunc(separator), len(currentDoc)) {
				total -= lenFunc(currentDoc[0])
				if len(currentDoc) > 1 && s.keepType == KeepTypeNone {
					total -= lenFunc(separator)
				}
				currentDoc = currentDoc[1:]
			}
		}

		currentDoc = append(currentDoc, split)
		total += lenFunc(split)
		if len(currentDoc) > 1 && t == KeepTypeNone {
			total += lenFunc(separator)
		}
	}

	doc := joinDocs(currentDoc, separator, t)
	if doc != "" {
		docs = append(docs, doc)
	}

	return docs
}

func (s *splitter) shouldPop(total, splitLen, separatorLen, currentDocLen int) bool {
	docsNeededToAddSep := 2
	if currentDocLen < docsNeededToAddSep {
		separatorLen = 0
	}
	if s.keepType == KeepTypeNone {
		return currentDocLen > 0 && (total > s.overlap || (total+splitLen+separatorLen > s.chunkSize && total > 0))
	}
	return currentDocLen > 0 && (total > s.overlap || (total+splitLen > s.chunkSize && total > 0))
}

func (s *splitter) GetType() string {
	return "RecursiveSplitter"
}

func joinDocs(docs []string, separator string, t KeepType) string {
	if t == KeepTypeNone {
		return strings.TrimSpace(strings.Join(docs, separator))
	}
	return strings.TrimSpace(strings.Join(docs, ""))
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	ret := make(map[string]interface{}, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}
