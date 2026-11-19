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
	"net"
	"testing"
	"time"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	qdrant "github.com/qdrant/go-client/qdrant"
	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
)

const CollectionName string = "test_collection"

func TestNewRetrieverValidation(t *testing.T) {
	PatchConvey("TestNewRetrieverValidation", t, func() {
		mockEmbedding := &mockEmbeddingQdrant{dims: 4}

		Convey("Given nil config", func() {
			retriever, err := NewRetriever(context.Background(), nil)

			Convey("Then it should return an error", func() {
				So(err, ShouldNotBeNil)
				So(retriever, ShouldBeNil)
				So(err.Error(), ShouldContainSubstring, "config is nil")
			})
		})

		Convey("Given config with nil embedding", func() {
			retriever, err := NewRetriever(context.Background(), &Config{
				Client:     nil,
				Collection: CollectionName,
				Embedding:  nil,
			})

			Convey("Then it should return an error", func() {
				So(err, ShouldNotBeNil)
				So(retriever, ShouldBeNil)
				So(err.Error(), ShouldContainSubstring, "embedding not provided")
			})
		})

		Convey("Given config with empty collection", func() {
			retriever, err := NewRetriever(context.Background(), &Config{
				Client:     nil,
				Collection: "",
				Embedding:  mockEmbedding,
			})

			Convey("Then it should return an error", func() {
				So(err, ShouldNotBeNil)
				So(retriever, ShouldBeNil)
				So(err.Error(), ShouldContainSubstring, "collection not provided")
			})
		})

		Convey("Given config with nil client", func() {
			retriever, err := NewRetriever(context.Background(), &Config{
				Client:     nil,
				Collection: CollectionName,
				Embedding:  mockEmbedding,
			})

			Convey("Then it should return an error", func() {
				So(err, ShouldNotBeNil)
				So(retriever, ShouldBeNil)
				So(err.Error(), ShouldContainSubstring, "client not provided")
			})
		})
	})
}

func TestRetrieverRetrieve(t *testing.T) {
	PatchConvey("TestRetrieverRetrieve", t, func() {
		ctx := context.Background()
		mockEmbedding := &mockEmbeddingQdrant{dims: 4}
		mockClient := &qdrant.Client{}

		var queryCalled bool

		Mock((*qdrant.Client).Query).To(func(c *qdrant.Client, ctx context.Context, req *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
			queryCalled = true

			return []*qdrant.ScoredPoint{
				{
					Id:    qdrant.NewID("fba95545-ef38-4880-bf4a-b98174554103"),
					Score: 0.95,
					Payload: map[string]*qdrant.Value{
						defaultContentKey: qdrant.NewValueString("Test content 1"),
					},
				},
			}, nil
		}).Build()

		Convey("Given a valid retriever", func() {
			retriever, err := NewRetriever(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Embedding:  mockEmbedding,
				TopK:       5,
			})

			Convey("Then the retriever should be created successfully", func() {
				So(err, ShouldBeNil)
				So(retriever, ShouldNotBeNil)
			})

			Convey("When retrieving documents", func() {
				docs, err := retriever.Retrieve(ctx, "test query")

				Convey("Then the retrieve operation should succeed", func() {
					So(err, ShouldBeNil)
					So(docs, ShouldNotBeNil)
					So(len(docs), ShouldEqual, 1)
				})

				Convey("And the document should have correct content", func() {
					So(docs[0].Content, ShouldEqual, "Test content 1")
					So(docs[0].ID, ShouldEqual, "fba95545-ef38-4880-bf4a-b98174554103")
				})

				Convey("And the mock Query method should be called", func() {
					So(queryCalled, ShouldBeTrue)
				})
			})
		})
	})
}

func TestRetrieverRetrieveWithError(t *testing.T) {
	PatchConvey("TestRetrieverRetrieveWithError", t, func() {
		ctx := context.Background()
		mockEmbedding := &mockEmbeddingQdrant{
			err:  fmt.Errorf("embedding failed"),
			dims: 4,
		}
		mockClient := &qdrant.Client{}

		Convey("Given a retriever with failing embedding", func() {
			retriever, err := NewRetriever(ctx, &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Embedding:  mockEmbedding,
				TopK:       5,
			})

			Convey("Then the retriever should be created successfully", func() {
				So(err, ShouldBeNil)
				So(retriever, ShouldNotBeNil)
			})

			Convey("When retrieving documents", func() {
				docs, err := retriever.Retrieve(ctx, "test query")

				Convey("Then the retrieve operation should fail", func() {
					So(err, ShouldNotBeNil)
					So(docs, ShouldBeNil)
					So(err.Error(), ShouldContainSubstring, "embedding failed")
				})
			})
		})
	})
}

type captureQdrantQuery struct {
	qdrant.UnimplementedPointsServer
	requests chan *qdrant.QueryPoints
}

func (s *captureQdrantQuery) Query(_ context.Context, request *qdrant.QueryPoints) (*qdrant.QueryResponse, error) {
	s.requests <- request
	return &qdrant.QueryResponse{}, nil
}

func TestRetrieverScoreThreshold(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	requests := make(chan *qdrant.QueryPoints, 4)
	qdrant.RegisterPointsServer(server, &captureQdrantQuery{requests: requests})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                   "127.0.0.1",
		Port:                   listener.Addr().(*net.TCPAddr).Port,
		SkipCompatibilityCheck: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	for _, tc := range []struct {
		name           string
		defaultScore   *float64
		requestedScore *float64
		want           *float32
	}{
		{name: "no threshold"},
		{name: "configured default", defaultScore: qdrant.PtrOf(0.25), want: qdrant.PtrOf(float32(0.25))},
		{name: "per-call threshold", requestedScore: qdrant.PtrOf(0.75), want: qdrant.PtrOf(float32(0.75))},
		{name: "zero overrides the default", defaultScore: qdrant.PtrOf(0.25), requestedScore: qdrant.PtrOf(0.0), want: qdrant.PtrOf(float32(0))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			r, err := NewRetriever(ctx, &Config{
				Client:         client,
				Collection:     CollectionName,
				Embedding:      &mockEmbeddingQdrant{dims: 4},
				ScoreThreshold: tc.defaultScore,
			})
			if err != nil {
				t.Fatal(err)
			}
			runnable, err := compose.NewChain[string, []*schema.Document]().AppendRetriever(r).Compile(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var opts []compose.Option
			if tc.requestedScore != nil {
				opts = append(opts, compose.WithRetrieverOption(retriever.WithScoreThreshold(*tc.requestedScore)))
			}
			if _, err := runnable.Invoke(ctx, "test query", opts...); err != nil {
				t.Fatal(err)
			}

			select {
			case request := <-requests:
				if tc.want == nil {
					if request.ScoreThreshold != nil {
						t.Fatalf("score_threshold = %v, want absent", *request.ScoreThreshold)
					}
				} else if request.ScoreThreshold == nil || *request.ScoreThreshold != *tc.want {
					t.Fatalf("score_threshold = %v, present = %t; want %v",
						request.GetScoreThreshold(), request.ScoreThreshold != nil, *tc.want)
				}
			case <-ctx.Done():
				t.Fatal("no Qdrant query received")
			}
		})
	}
}

func TestRetrieverTypeAndCallbacks(t *testing.T) {
	PatchConvey("TestRetrieverTypeAndCallbacks", t, func() {
		mockEmbedding := &mockEmbeddingQdrant{dims: 4}
		mockClient := &qdrant.Client{}

		Convey("Given a valid retriever", func() {
			retriever, err := NewRetriever(context.Background(), &Config{
				Client:     mockClient,
				Collection: CollectionName,
				Embedding:  mockEmbedding,
				TopK:       5,
			})

			Convey("Then the retriever should be created successfully", func() {
				So(err, ShouldBeNil)
				So(retriever, ShouldNotBeNil)
			})

			Convey("And GetType should return 'Qdrant'", func() {
				So(retriever.GetType(), ShouldEqual, "Qdrant")
			})

			Convey("And IsCallbacksEnabled should return true", func() {
				So(retriever.IsCallbacksEnabled(), ShouldBeTrue)
			})
		})
	})
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
