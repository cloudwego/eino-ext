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

package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	qri "github.com/cloudwego/eino-ext/components/indexer/qdrant"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	qdrant "github.com/qdrant/go-client/qdrant"
)

func main() {
	ctx := context.Background()
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		panic(err)
	}

	dim := 1024
	batch := 10

	dense := make([][]float64, batch)
	for i := 0; i < batch; i++ {
		vec := make([]float64, dim)
		for j := 0; j < dim; j++ {
			vec[j] = rand.Float64()
		}
		dense[i] = vec
	}

	indexer, err := qri.NewIndexer(ctx, &qri.Config{
		Client:     client,
		Collection: "eino_collection",
		VectorDim:  dim,
		Distance:   qdrant.Distance_Cosine,
		BatchSize:  10,
		Embedding:  &mockEmbedding{dense},
	})
	if err != nil {
		panic(err)
	}

	contents := `1. Eiffel Tower: Located in Paris, France, it is one of the most famous landmarks in the world, designed by Gustave Eiffel and built in 1889.
2. The Great Wall: Located in China, it is one of the Seven Wonders of the World, built from the Qin Dynasty to the Ming Dynasty, with a total length of over 20000 kilometers.
3. Grand Canyon National Park: Located in Arizona, USA, it is famous for its deep canyons and magnificent scenery, which are cut by the Colorado River.
4. The Colosseum: Located in Rome, Italy, built between 70-80 AD, it was the largest circular arena in the ancient Roman Empire.
5. Taj Mahal: Located in Agra, India, it was completed by Mughal Emperor Shah Jahan in 1653 to commemorate his wife and is one of the New Seven Wonders of the World.
6. Sydney Opera House: Located in Sydney Harbour, Australia, it is one of the most iconic buildings of the 20th century, renowned for its unique sailboat design.
7. Louvre Museum: Located in Paris, France, it is one of the largest museums in the world with a rich collection, including Leonardo da Vinci's Mona Lisa and Greece's Venus de Milo.
8. Niagara Falls: located at the border of the United States and Canada, consisting of three main waterfalls, its spectacular scenery attracts millions of tourists every year.
9. St. Sophia Cathedral: located in Istanbul, Türkiye, originally built in 537 A.D., it used to be an Orthodox cathedral and mosque, and now it is a museum.
10. Machu Picchu: an ancient Inca site located on the plateau of the Andes Mountains in Peru, one of the New Seven Wonders of the World, with an altitude of over 2400 meters.`

	var docs []*schema.Document
	for _, str := range strings.Split(contents, "\n") {
		docs = append(docs, &schema.Document{
			ID:      uuid.NewString(),
			Content: str,
		})
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		panic(err)
	}

	fmt.Println(ids)
}

type mockEmbedding struct {
	dense [][]float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.dense, nil
}
