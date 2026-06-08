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

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
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

func TestIntegration_Indexer_Hash(t *testing.T) {
	ctx := context.Background()
	client := getTestClient(t)
	defer client.Close()

	prefix := "inttest:indexer:hash:"

	t.Cleanup(func() {
		client.CustomCommand(ctx, []string{"DEL", prefix + "doc1", prefix + "doc2", prefix + "doc3"})
	})

	idx, err := NewIndexer(ctx, &IndexerConfig{
		Client:       client,
		KeyPrefix:    prefix,
		DocumentType: DocumentTypeHash,
		BatchSize:    10,
		Embedding:    &deterministicEmbedder{dim: 8},
	})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}

	docs := []*schema.Document{
		{ID: "doc1", Content: "Valkey is a high-performance key-value store"},
		{ID: "doc2", Content: "Vector search enables semantic similarity queries"},
		{ID: "doc3", Content: "The quick brown fox", MetaData: map[string]any{"category": "example"}},
	}

	ids, err := idx.Store(ctx, docs)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 ids, got %d", len(ids))
	}

	// Verify via HGetAll
	for _, doc := range docs {
		result, err := client.HGetAll(ctx, prefix+doc.ID)
		if err != nil {
			t.Fatalf("HGetAll failed for %s: %v", doc.ID, err)
		}
		if result[defaultReturnFieldContent] != doc.Content {
			t.Fatalf("content mismatch for %s", doc.ID)
		}
		if result[defaultReturnFieldVectorContent] == "" {
			t.Fatalf("vector not set for %s", doc.ID)
		}
	}
	t.Logf("Successfully stored %d hash documents", len(ids))
}

func TestIntegration_Indexer_JSON(t *testing.T) {
	ctx := context.Background()
	client := getTestClient(t)
	defer client.Close()

	prefix := "inttest:indexer:json:"

	t.Cleanup(func() {
		client.CustomCommand(ctx, []string{"DEL", prefix + "doc1", prefix + "doc2"})
	})

	idx, err := NewIndexer(ctx, &IndexerConfig{
		Client:       client,
		KeyPrefix:    prefix,
		DocumentType: DocumentTypeJSON,
		BatchSize:    10,
		Embedding:    &deterministicEmbedder{dim: 8},
	})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}

	docs := []*schema.Document{
		{ID: "doc1", Content: "JSON document storage in Valkey"},
		{ID: "doc2", Content: "Supports nested structures", MetaData: map[string]any{"tag": "test"}},
	}

	ids, err := idx.Store(ctx, docs)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}

	// Verify via JSON.GET
	for _, doc := range docs {
		result, err := client.CustomCommand(ctx, []string{"JSON.GET", prefix + doc.ID, "$." + defaultReturnFieldContent})
		if err != nil {
			t.Fatalf("JSON.GET failed for %s: %v", doc.ID, err)
		}
		if result == nil {
			t.Fatalf("no JSON data for %s", doc.ID)
		}
		t.Logf("  %s: %v", doc.ID, result)
	}
	t.Logf("Successfully stored %d JSON documents", len(ids))
}
