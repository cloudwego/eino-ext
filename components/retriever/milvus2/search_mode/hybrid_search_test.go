package search_mode_test

import (
	"context"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/smartystreets/goconvey/convey"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
)

func TestHybridSearchTopK(t *testing.T) {
	PatchConvey("Test Hybrid Search TopK Default", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}

		// Mock Reranker
		mockReranker := milvusclient.NewRRFReranker()

		// Hybrid mode with no explicit TopK in SubRequest
		hybridMode := search_mode.NewHybrid(
			mockReranker,
			&search_mode.SubRequest{
				VectorField: "vector",
				MetricType:  milvus2.L2,
				// TopK is unset (0)
			},
		)

		// We just need a config object to pass to BuildHybridSearchOption
		// No need for a full Retriever instance since we are testing the SearchMode method directly
		config := &milvus2.RetrieverConfig{
			Collection:  "test_collection",
			VectorField: "vector",
			TopK:        50, // Global TopK > 10
			SearchMode:  hybridMode,
		}

		convey.Convey("should use global TopK when SubRequest TopK is missing", func() {
			Mock(GetMethod(mockClient, "HybridSearch")).Return([]milvusclient.ResultSet{}, nil).Build()

			queryVector := make([]float32, 128)
			opt, err := hybridMode.BuildHybridSearchOption(ctx, config, queryVector, "query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)

			var capturedLimit int
			Mock(milvusclient.NewAnnRequest).To(func(fieldName string, limit int, vectors ...entity.Vector) *milvusclient.AnnRequest {
				capturedLimit = limit
				return &milvusclient.AnnRequest{}
			}).Build()

			Mock((*milvusclient.AnnRequest).WithSearchParam).To(func(r *milvusclient.AnnRequest, key string, value string) *milvusclient.AnnRequest {
				return r
			}).Build()

			_, _ = hybridMode.BuildHybridSearchOption(ctx, config, queryVector, "query")

			convey.So(capturedLimit, convey.ShouldEqual, 50)
		})

		convey.Convey("should use explicit TopK when SubRequest TopK is set", func() {
			hybridMode.SubRequests[0].TopK = 5

			var capturedLimit int
			Mock(milvusclient.NewAnnRequest).To(func(fieldName string, limit int, vectors ...entity.Vector) *milvusclient.AnnRequest {
				capturedLimit = limit
				return &milvusclient.AnnRequest{}
			}).Build()

			Mock((*milvusclient.AnnRequest).WithSearchParam).To(func(r *milvusclient.AnnRequest, key string, value string) *milvusclient.AnnRequest {
				return r
			}).Build()

			queryVector := make([]float32, 128)
			_, _ = hybridMode.BuildHybridSearchOption(ctx, config, queryVector, "query")

			convey.So(capturedLimit, convey.ShouldEqual, 5)
		})
	})
}
