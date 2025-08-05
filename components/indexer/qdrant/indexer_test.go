/*
 * Copyright 2025 CloudWeGo Authors
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

package qdrant

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const TestImage string = "qdrant/qdrant:v1.15.1"
const CollectionName string = "test_collection"
const APIKey string = "test-api-key"

func TestIndexer(t *testing.T) {
	ctx := context.Background()

	container, err := standaloneQdrant(ctx, APIKey)
	require.NoError(t, err)

	err = container.Start(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6334/tcp")
	require.NoError(t, err)

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                   host,
		Port:                   port.Int(),
		APIKey:                 APIKey,
		UseTLS:                 false,
		SkipCompatibilityCheck: true,
	})
	require.NoError(t, err)

	d1 := &schema.Document{ID: "c60df334-dbbe-49b8-82d8-a2bd668602f6", Content: "asd"}
	d2 := &schema.Document{ID: "7b83aca0-5f6c-4491-8dd4-22e15e9d582e", Content: "qwe", MetaData: map[string]any{
		"mock_field_1": map[string]any{"extra_field_1": "asd"},
		"mock_field_2": int64(123),
	}}
	docs := []*schema.Document{d1, d2}

	i, err := NewIndexer(ctx, &IndexerConfig{
		Client:     client,
		Collection: CollectionName,
		BatchSize:  10,
		Embedding:  &mockEmbeddingQdrant{dims: 4},
		VectorDim:  4,
		Distance:   qdrant.Distance_Cosine,
	})
	require.NoError(t, err)

	ids, err := i.Store(ctx, docs)
	require.NoError(t, err)
	require.Len(t, ids, len(docs))

	count, err := client.Count(ctx, &qdrant.CountPoints{
		CollectionName: CollectionName,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(len(docs)), count)
}

func standaloneQdrant(ctx context.Context, apiKey string) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        TestImage,
		ExposedPorts: []string{"6334/tcp"},
		Env: map[string]string{
			"QDRANT__SERVICE__API_KEY": apiKey,
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("6334/tcp").WithStartupTimeout(5 * time.Second),
		),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
	})

	return container, err
}

type mockEmbeddingQdrant struct {
	err  error
	dims int
}

func (m *mockEmbeddingQdrant) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}

	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, m.dims)
		for j := range vec {
			vec[j] = rand.Float64()
		}
		result[i] = vec
	}
	return result, nil
}
