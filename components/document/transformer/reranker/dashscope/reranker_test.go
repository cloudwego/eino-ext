package dashscope

import (
	"context"
	"os"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestReranker(t *testing.T) {

	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	ctx := context.Background()

	docs := makeDocs()

	r := NewReranker(ctx, &RerankerConfig{
		ModelName: "qwen3-rerank",
		ApiKey:    apiKey,
		BaseUrl:   "https://dashscope.aliyuncs.com/api/v1",
		TopN:      nil,
	})

	sortedDocs, err := r.Transform(ctx, docs, WithQuery("什么是文本排序模型"))
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if len(sortedDocs) != 3 {
		t.Fatalf("Expected 3 documents, got %d", len(sortedDocs))
	}

	if sortedDocs[0].ID != "1" {
		t.Fatalf("Expected document 1 to be first, got %s", sortedDocs[0].ID)
	}
	return
}

func makeDocs() []*schema.Document {
	return []*schema.Document{
		{
			ID:      "1",
			Content: "文本排序模型广泛用于搜索引擎和推荐系统中，它们根据文本相关性对候选文本进行排序",
		},
		{
			ID:      "2",
			Content: "量子计算是计算科学的一个前沿领域",
		},
		{
			ID:      "3",
			Content: "预训练语言模型的发展给文本排序模型带来了新的进展",
		},
	}
}
