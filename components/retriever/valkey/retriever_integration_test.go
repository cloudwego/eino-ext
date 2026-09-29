//go:build integration

/*
 * Copyright 2026 CloudWeGo Authors
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

package valkey

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	glide "github.com/valkey-io/valkey-glide/go/v2"
	"github.com/valkey-io/valkey-glide/go/v2/config"
)

func getTestClient(t *testing.T) *glide.Client {
	t.Helper()
	addr := os.Getenv("VALKEY_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("invalid VALKEY_ADDR %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port in VALKEY_ADDR %q: %v", addr, err)
	}
	cfg := config.NewClientConfiguration().
		WithAddress(&config.NodeAddress{Host: host, Port: port})
	client, err := glide.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create valkey client: %v", err)
	}
	return client
}

// waitForIndex polls FT.INFO until the index reports at least wantDocs indexed
// documents with an empty mutation queue, or fails the test after a timeout.
// This replaces a fixed sleep, which is racy: too short flakes under load, too
// long wastes time on every run.
func waitForIndex(t *testing.T, ctx context.Context, client *glide.Client, indexName string, wantDocs int) {
	t.Helper()
	const (
		timeout  = 5 * time.Second
		interval = 25 * time.Millisecond
	)
	deadline := time.Now().Add(timeout)
	var lastDocs, lastQueue int64
	for {
		raw, err := client.CustomCommand(ctx, []string{"FT.INFO", indexName})
		if err != nil {
			t.Fatalf("FT.INFO %s failed: %v", indexName, err)
		}
		info := ftInfoToMap(raw)
		lastDocs = ftInfoInt(info["num_docs"])
		lastQueue = ftInfoInt(info["mutation_queue_size"])
		if lastDocs >= int64(wantDocs) && lastQueue == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("index %s not ready after %s: num_docs=%d (want >=%d), mutation_queue_size=%d (want 0)",
				indexName, timeout, lastDocs, wantDocs, lastQueue)
		}
		time.Sleep(interval)
	}
}

// ftInfoToMap flattens an FT.INFO reply into a key->value map. Valkey returns a
// flat [key, value, key, value, ...] sequence under RESP2 and may return a map
// under RESP3; this handles both.
func ftInfoToMap(raw any) map[string]any {
	out := make(map[string]any)
	switch v := raw.(type) {
	case map[string]any:
		return v
	case []any:
		for i := 0; i+1 < len(v); i += 2 {
			if key, ok := asString(v[i]); ok {
				out[key] = v[i+1]
			}
		}
	}
	return out
}

func asString(v any) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case []byte:
		return string(s), true
	}
	return "", false
}

// ftInfoInt coerces an FT.INFO numeric field (which may arrive as an integer or
// as a string) to int64; unparsable or missing values yield 0.
func ftInfoInt(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	if s, ok := asString(v); ok {
		if parsed, err := strconv.ParseInt(s, 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

// deterministicEmbedder returns vectors based on text for predictable similarity.
// Texts with similar characters produce similar vectors.
type deterministicEmbedder struct {
	dim int
}

func (e *deterministicEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i, text := range texts {
		vec := make([]float64, e.dim)
		for j := 0; j < e.dim; j++ {
			if j < len(text) {
				vec[j] = float64(text[j]) / 255.0
			}
		}
		vectors[i] = vec
	}
	return vectors, nil
}

func TestIntegration_Retriever_KNN(t *testing.T) {
	ctx := context.Background()
	client := getTestClient(t)
	defer client.Close()

	const (
		prefix    = "inttest:retriever:"
		indexName = "inttest_retriever_idx"
		dim       = 8
	)
	emb := &deterministicEmbedder{dim: dim}

	// Cleanup
	t.Cleanup(func() {
		client.CustomCommand(ctx, []string{"FT.DROPINDEX", indexName})
		client.CustomCommand(ctx, []string{"DEL", prefix + "doc1", prefix + "doc2", prefix + "doc3"})
	})

	// Drop index if it exists from a previous run
	client.CustomCommand(ctx, []string{"FT.DROPINDEX", indexName})

	// Create index
	_, err := client.CustomCommand(ctx, []string{
		"FT.CREATE", indexName,
		"ON", "HASH",
		"PREFIX", "1", prefix,
		"SCHEMA",
		"content", "TEXT",
		"vector_content", "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32",
		"DIM", "8",
		"DISTANCE_METRIC", "L2",
	})
	if err != nil {
		t.Fatalf("FT.CREATE failed: %v", err)
	}

	// Store documents using HSet directly (mimicking what the indexer does)
	docs := []struct {
		id      string
		content string
	}{
		{"doc1", "Valkey is a high-performance key-value store"},
		{"doc2", "Vector search enables semantic similarity queries"},
		{"doc3", "The quick brown fox jumps over the lazy dog"},
	}

	for _, doc := range docs {
		vecs, _ := emb.EmbedStrings(ctx, []string{doc.content})
		vecBytes := vector2Bytes(vecs[0])
		_, err := client.HSet(ctx, prefix+doc.id, map[string]string{
			"content":        doc.content,
			"vector_content": string(vecBytes),
		})
		if err != nil {
			t.Fatalf("HSet failed for %s: %v", doc.id, err)
		}
	}

	// Wait for the index to reflect the documents just written.
	waitForIndex(t, ctx, client, indexName, len(docs))

	// Create retriever
	r, err := NewRetriever(ctx, &RetrieverConfig{
		Client:    client,
		Index:     indexName,
		TopK:      3,
		Embedding: emb,
	})
	if err != nil {
		t.Fatalf("NewRetriever failed: %v", err)
	}

	// Search - query similar to doc1
	results, err := r.Retrieve(ctx, "Valkey is a high-performance key-value store")
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result, got 0")
	}

	t.Logf("Got %d results", len(results))
	for i, doc := range results {
		t.Logf("  [%d] ID=%s Content=%q", i, doc.ID, doc.Content)
	}

	// The first result should be doc1 since the query is identical
	found := false
	for _, doc := range results {
		if doc.ID == prefix+"doc1" {
			found = true
			if doc.Content != "Valkey is a high-performance key-value store" {
				t.Fatalf("unexpected content for doc1: %q", doc.Content)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected doc1 in results (exact match query)")
	}
}

func TestIntegration_Retriever_VectorRange(t *testing.T) {
	// VECTOR_RANGE is a Redis Stack feature not currently supported by Valkey Search.
	// Valkey Search only supports KNN vector queries.
	// This test is kept as a placeholder for when/if Valkey Search adds range support.
	t.Skip("VECTOR_RANGE not supported by Valkey Search module")
}

func TestIntegration_Retriever_WithFilter(t *testing.T) {
	ctx := context.Background()
	client := getTestClient(t)
	defer client.Close()

	const (
		prefix    = "inttest:filter:"
		indexName = "inttest_filter_idx"
		dim       = 8
	)
	emb := &deterministicEmbedder{dim: dim}

	// Cleanup
	t.Cleanup(func() {
		client.CustomCommand(ctx, []string{"FT.DROPINDEX", indexName})
		client.CustomCommand(ctx, []string{"DEL", prefix + "doc1", prefix + "doc2"})
	})

	client.CustomCommand(ctx, []string{"FT.DROPINDEX", indexName})

	_, err := client.CustomCommand(ctx, []string{
		"FT.CREATE", indexName,
		"ON", "HASH",
		"PREFIX", "1", prefix,
		"SCHEMA",
		"content", "TEXT",
		"category", "TAG",
		"vector_content", "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32",
		"DIM", "8",
		"DISTANCE_METRIC", "L2",
	})
	if err != nil {
		t.Fatalf("FT.CREATE failed: %v", err)
	}

	// Store docs with different categories
	vecs, _ := emb.EmbedStrings(ctx, []string{"hello world", "hello world"})

	client.HSet(ctx, prefix+"doc1", map[string]string{
		"content":        "hello world",
		"category":       "tech",
		"vector_content": string(vector2Bytes(vecs[0])),
	})
	client.HSet(ctx, prefix+"doc2", map[string]string{
		"content":        "hello world",
		"category":       "science",
		"vector_content": string(vector2Bytes(vecs[1])),
	})

	// Wait for the index to reflect the two documents just written.
	waitForIndex(t, ctx, client, indexName, 2)

	r, err := NewRetriever(ctx, &RetrieverConfig{
		Client:    client,
		Index:     indexName,
		TopK:      10,
		Embedding: emb,
	})
	if err != nil {
		t.Fatalf("NewRetriever failed: %v", err)
	}

	// Search with filter - only tech category
	results, err := r.Retrieve(ctx, "hello world", WithFilterQuery("@category:{tech}"))
	if err != nil {
		t.Fatalf("Retrieve with filter failed: %v", err)
	}

	t.Logf("Got %d filtered results", len(results))
	for i, doc := range results {
		t.Logf("  [%d] ID=%s Content=%q", i, doc.ID, doc.Content)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result with tech filter, got %d", len(results))
	}
	if results[0].ID != prefix+"doc1" {
		t.Fatalf("expected doc1 (tech), got %s", results[0].ID)
	}
}
