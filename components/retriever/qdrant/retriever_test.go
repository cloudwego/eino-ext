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
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/google/uuid"
	qdrant "github.com/qdrant/go-client/qdrant"
	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const TestImage string = "qdrant/qdrant:v1.15.1"
const CollectionName string = "test_collection"
const APIKey string = "test-api-key"

func TestRetrieve(t *testing.T) {
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

	err = client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: CollectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     4,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	require.NoError(t, err)

	points := make([]*qdrant.PointStruct, 0)
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("content %d", i)
		metadata := map[string]any{
			"something": "something",
		}

		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewID(uuid.NewString()),
			Vectors: qdrant.NewVectorsDense([]float32{rand.Float32(), rand.Float32(), rand.Float32(), rand.Float32()}),
			Payload: qdrant.NewValueMap(map[string]any{
				defaultContentKey:  content,
				defaultMetadataKey: metadata,
			}),
		})
	}

	_, err = client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: CollectionName,
		Points:         points,
		Wait:           qdrant.PtrOf(true),
	})
	require.NoError(t, err)

	i, err := NewRetriever(ctx, &RetrieverConfig{
		Client:     client,
		Collection: CollectionName,
		Embedding:  &mockEmbeddingQdrant{dims: 4},
		TopK:       5,
	})
	require.NoError(t, err)

	docs, err := i.Retrieve(ctx, "query")
	require.NoError(t, err)
	require.Len(t, docs, 5)
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
