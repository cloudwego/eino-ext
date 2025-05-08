package pgvector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// mockEmbedder implements a simple mock embedder for testing
type mockEmbedder struct{}

// Generate a simple vector for each text
func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i := range texts {
		// 为每个文本生成一个简单的向量
		vectors[i] = make([]float64, 3)
		for j := range vectors[i] {
			vectors[i][j] = float64(i+j) / 10.0
		}
	}
	return vectors, nil
}

func (m *mockEmbedder) GetDimension() int {
	return 3
}

// testConfig database configuration for testing
var testConfig = &IndexerConfig{
	Host:      "localhost",
	Port:      5432,
	User:      "postgres",
	Password:  "postgres",
	DBName:    "vectorDB",
	SSLMode:   "disable",
	TableName: "test_vectors",
	Dimension: 3,
	Embedding: &mockEmbedder{},
}

func TestNewIndexer(t *testing.T) {
	tests := []struct {
		name    string
		config  *IndexerConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  testConfig,
			wantErr: false,
		},
		{
			name: "missing embedding",
			config: &IndexerConfig{
				Host:      testConfig.Host,
				Port:      testConfig.Port,
				User:      testConfig.User,
				Password:  testConfig.Password,
				DBName:    testConfig.DBName,
				SSLMode:   testConfig.SSLMode,
				TableName: testConfig.TableName,
				Dimension: testConfig.Dimension,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			indexer, err := NewIndexer(ctx, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, indexer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, indexer)
				if indexer != nil {
					defer indexer.Close()
				}
			}
		})
	}
}

func TestIndexer_Store(t *testing.T) {
	// Create indexer
	ctx := context.Background()
	indexer, err := NewIndexer(ctx, testConfig)
	require.NoError(t, err)
	defer indexer.Close()

	// Prepare test documents
	docs := []*schema.Document{
		{
			ID:      "doc1",
			Content: "test document 1",
			MetaData: map[string]interface{}{
				"source": "test",
			},
		},
		{
			ID:      "doc2",
			Content: "test document 2",
			MetaData: map[string]interface{}{
				"source": "test",
			},
		},
	}

	// Test storing documents
	ids, err := indexer.Store(ctx, docs)
	assert.NoError(t, err)
	assert.Len(t, ids, 2)
	assert.Equal(t, "doc1", ids[0])
	assert.Equal(t, "doc2", ids[1])
}

func TestVectorTypeValidation(t *testing.T) {
	tests := []struct {
		name      string
		vecType   VectorType
		dimension int
		wantErr   bool
	}{
		{
			name:      "valid vector type",
			vecType:   VectorTypeVector,
			dimension: 1000,
			wantErr:   false,
		},
		{
			name:      "invalid dimension for vector",
			vecType:   VectorTypeVector,
			dimension: 3000,
			wantErr:   true,
		},
		{
			name:      "valid halfvec type",
			vecType:   VectorTypeHalfvec,
			dimension: 3000,
			wantErr:   false,
		},
		{
			name:      "invalid dimension for halfvec",
			vecType:   VectorTypeHalfvec,
			dimension: 5000,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVectorConfig(tt.vecType, tt.dimension)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSuitableVectorType(t *testing.T) {
	tests := []struct {
		name      string
		dimension int
		want      VectorType
	}{
		{
			name:      "vector type for small dimension",
			dimension: 1000,
			want:      VectorTypeVector,
		},
		{
			name:      "halfvec type for medium dimension",
			dimension: 3000,
			want:      VectorTypeHalfvec,
		},
		{
			name:      "bit type for large dimension",
			dimension: 50000,
			want:      VectorTypeBit,
		},
		{
			name:      "sparsevec type for very large dimension",
			dimension: 100000,
			want:      VectorTypeSparsevec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSuitableVectorType(tt.dimension)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatVector(t *testing.T) {
	indexer := &Indexer{}
	vector := []float64{1.0, 2.0, 3.0}
	expected := "[1.000000,2.000000,3.000000]"

	got := indexer.formatVector(vector)
	assert.Equal(t, expected, got)
}
