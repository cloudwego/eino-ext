package es8

import (
	"github.com/cloudwego/eino-ext/components/indexer/es8/field_mapping"
	"github.com/cloudwego/eino/schema"
)

func GetType() string {
	return typ
}

func toESDoc(doc *schema.Document) map[string]any {
	mp := make(map[string]any)
	if kvs, ok := field_mapping.GetExtraDataFields(doc); ok {
		for k, v := range kvs {
			mp[k] = v
		}
	}

	mp[field_mapping.DocFieldNameContent] = doc.Content

	return mp
}

func chunk[T any](slice []T, size int) [][]T {
	if size <= 0 {
		return nil
	}

	var chunks [][]T
	for size < len(slice) {
		slice, chunks = slice[size:], append(chunks, slice[0:size:size])
	}

	if len(slice) > 0 {
		chunks = append(chunks, slice)
	}

	return chunks
}

func iter[T, D any](src []T, fn func(T) D) []D {
	resp := make([]D, len(src))
	for i := range src {
		resp[i] = fn(src[i])
	}

	return resp
}

func iterWithErr[T, D any](src []T, fn func(T) (D, error)) ([]D, error) {
	resp := make([]D, 0, len(src))
	for i := range src {
		d, err := fn(src[i])
		if err != nil {
			return nil, err
		}

		resp = append(resp, d)
	}

	return resp, nil
}
