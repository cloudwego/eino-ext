package milvus

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-proto/go-api/v2/msgpb"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

func TestIndexerConfig_check(t *testing.T) {
	type fields struct {
		Client              client.Client
		Collection          string
		PartitionNum        int64
		Description         string
		Dim                 int64
		SharedNum           int32
		ConsistencyLevel    ConsistencyLevel
		EnableDynamicSchema bool
		MetricType          MetricType
		Embedding           embedding.Embedder
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "valid config",
			fields: fields{
				Client:              &mockClient{},
				Collection:          "test_collection",
				PartitionNum:        1,
				Description:         "test description",
				Dim:                 128,
				SharedNum:           1,
				ConsistencyLevel:    defaultConsistencyLevel,
				EnableDynamicSchema: false,
				MetricType:          HAMMING,
				Embedding:           &mockEmbedding{},
			},
			wantErr: false,
		},
		{
			name: "invalid dim",
			fields: fields{
				Client:              &mockClient{},
				Collection:          "test_collection",
				PartitionNum:        1,
				Description:         "test description",
				Dim:                 7,
				SharedNum:           1,
				ConsistencyLevel:    defaultConsistencyLevel,
				EnableDynamicSchema: false,
				MetricType:          HAMMING,
				Embedding:           &mockEmbedding{},
			},
			wantErr: true,
		},
		{
			name: "invalid dim",
			fields: fields{
				Client:              &mockClient{},
				Collection:          "test_collection",
				PartitionNum:        1,
				Description:         "test description",
				Dim:                 -1,
				SharedNum:           1,
				ConsistencyLevel:    defaultConsistencyLevel,
				EnableDynamicSchema: false,
				MetricType:          HAMMING,
				Embedding:           &mockEmbedding{},
			},
			wantErr: false,
		},
		{
			name: "missing client",
			fields: fields{
				Client:              nil,
				Collection:          "test_collection",
				PartitionNum:        1,
				Description:         "test description",
				Dim:                 128,
				SharedNum:           1,
				ConsistencyLevel:    defaultConsistencyLevel,
				EnableDynamicSchema: false,
				MetricType:          HAMMING,
				Embedding:           &mockEmbedding{},
			},
			wantErr: true,
		},
		{
			name: "missing embedding",
			fields: fields{
				Client:              &mockClient{},
				Collection:          "test_collection",
				PartitionNum:        1,
				Description:         "test description",
				Dim:                 128,
				SharedNum:           1,
				ConsistencyLevel:    defaultConsistencyLevel,
				EnableDynamicSchema: false,
				MetricType:          HAMMING,
				Embedding:           nil,
			},
			wantErr: true,
		},
		{
			name: "missing collection",
			fields: fields{
				Client:              &mockClient{},
				Collection:          "",
				PartitionNum:        0,
				Description:         "",
				Dim:                 0,
				SharedNum:           0,
				ConsistencyLevel:    0,
				EnableDynamicSchema: false,
				MetricType:          "",
				Embedding:           &mockEmbedding{},
			},
			wantErr: false,
		},
		{
			name: "invalid consistency level",
			fields: fields{
				Client:              &mockClient{},
				Collection:          "test_collection",
				PartitionNum:        1,
				Description:         "test description",
				Dim:                 128,
				SharedNum:           1,
				ConsistencyLevel:    10,
				EnableDynamicSchema: false,
				MetricType:          HAMMING,
				Embedding:           &mockEmbedding{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &IndexerConfig{
				Client:              tt.fields.Client,
				Collection:          tt.fields.Collection,
				PartitionNum:        tt.fields.PartitionNum,
				Description:         tt.fields.Description,
				Dim:                 tt.fields.Dim,
				SharedNum:           tt.fields.SharedNum,
				ConsistencyLevel:    tt.fields.ConsistencyLevel,
				EnableDynamicSchema: tt.fields.EnableDynamicSchema,
				MetricType:          tt.fields.MetricType,
				Embedding:           tt.fields.Embedding,
			}
			if err := i.check(); (err != nil) != tt.wantErr {
				t.Errorf("check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewIndexer(t *testing.T) {
	type args struct {
		ctx  context.Context
		conf *IndexerConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *Indexer
		wantErr bool
	}{
		{
			name: "new indexer successfully",
			args: args{
				ctx: context.Background(),
				conf: &IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        0,
					Description:         defaultDescription,
					Dim:                 0,
					SharedNum:           0,
					ConsistencyLevel:    0,
					EnableDynamicSchema: false,
					MetricType:          "",
					Embedding:           &mockEmbedding{},
				},
			},
			want: &Indexer{
				config: IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        0,
					Description:         defaultDescription,
					Dim:                 81920,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           &mockEmbedding{},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid dim",
			args: args{
				ctx: context.Background(),
				conf: &IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        1,
					Description:         defaultDescription,
					Dim:                 7,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           &mockEmbedding{},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid dim",
			args: args{
				ctx: context.Background(),
				conf: &IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        1,
					Description:         defaultDescription,
					Dim:                 -1,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           &mockEmbedding{},
				},
			},
			want: &Indexer{
				config: IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        0,
					Description:         defaultDescription,
					Dim:                 81920,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           &mockEmbedding{},
				},
			},
			wantErr: false,
		},
		{
			name: "missing client",
			args: args{
				ctx: context.Background(),
				conf: &IndexerConfig{
					Client:              nil,
					Collection:          "test_collection",
					PartitionNum:        1,
					Description:         "test description",
					Dim:                 128,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           &mockEmbedding{},
				},
			},
			wantErr: true,
		},
		{
			name: "missing embedding",
			args: args{
				ctx: context.Background(),
				conf: &IndexerConfig{
					Client:              &mockClient{},
					Collection:          "test_collection",
					PartitionNum:        1,
					Description:         "test description",
					Dim:                 128,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           nil,
				},
			},
			wantErr: true,
		},
		{
			name: "missing collection",
			args: args{
				ctx: context.Background(),
				conf: &IndexerConfig{
					Client:              &mockClient{},
					Collection:          "",
					PartitionNum:        0,
					Description:         "",
					Dim:                 0,
					SharedNum:           0,
					ConsistencyLevel:    0,
					EnableDynamicSchema: false,
					MetricType:          "",
					Embedding:           &mockEmbedding{},
				},
			},
			want: &Indexer{
				config: IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        0,
					Description:         defaultDescription,
					Dim:                 81920,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           &mockEmbedding{},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid consistency level",
			args: args{
				ctx: context.Background(),
				conf: &IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        1,
					Description:         defaultDescription,
					Dim:                 128,
					SharedNum:           1,
					ConsistencyLevel:    10,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           &mockEmbedding{},
				},
			},
			want: &Indexer{
				config: IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        0,
					Description:         defaultDescription,
					Dim:                 128,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           &mockEmbedding{},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewIndexer(tt.args.ctx, tt.args.conf)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewIndexer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewIndexer() got = %v,\n want %v", got, tt.want)
			}
		})
	}
}

func TestIndexer_Store(t *testing.T) {
	type fields struct {
		config IndexerConfig
	}
	type args struct {
		ctx  context.Context
		docs []*schema.Document
		opts []indexer.Option
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantIds []string
		wantErr bool
	}{
		{
			name: "store documents successfully",
			fields: fields{
				config: IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        1,
					Description:         defaultDescription,
					Dim:                 128,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding: &mockEmbedding{
						err:         nil,
						cnt:         0,
						sizeForCall: []int{2},
						dims:        1024,
					},
				},
			},
			args: args{
				ctx: context.Background(),
				docs: []*schema.Document{
					{ID: "1", Content: "test content 1"},
					{ID: "2", Content: "test content 2"},
				},
				opts: nil,
			},
			wantIds: []string{"1", "2"},
			wantErr: false,
		},
		{
			name: "embedding not provided",
			fields: fields{
				config: IndexerConfig{
					Client:              &mockClient{},
					Collection:          "test_collection",
					PartitionNum:        1,
					Description:         "test description",
					Dim:                 128,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding:           nil,
				},
			},
			args: args{
				ctx: context.Background(),
				docs: []*schema.Document{
					{ID: "1", Content: "test content 1"},
				},
				opts: nil,
			},
			wantIds: nil,
			wantErr: true,
		},
		{
			name: "embedding result length not match",
			fields: fields{
				config: IndexerConfig{
					Client:              &mockClient{},
					Collection:          defaultCollection,
					PartitionNum:        1,
					Description:         defaultDescription,
					Dim:                 128,
					SharedNum:           1,
					ConsistencyLevel:    defaultConsistencyLevel,
					EnableDynamicSchema: false,
					MetricType:          HAMMING,
					Embedding: &mockEmbedding{
						err:         nil,
						cnt:         0,
						sizeForCall: []int{2},
						dims:        0,
					},
				},
			},
			args: args{
				ctx: context.Background(),
				docs: []*schema.Document{
					{ID: "1", Content: "test content 1"},
				},
				opts: nil,
			},
			wantIds: nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &Indexer{
				config: tt.fields.config,
			}
			gotIds, err := i.Store(tt.args.ctx, tt.args.docs, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Store() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotIds, tt.wantIds) {
				t.Errorf("Store() gotIds = %v, want %v", gotIds, tt.wantIds)
			}
		})
	}
}

type mockClient struct {
}

func (m *mockClient) Close() error {
	panic("implement me")
}

func (m *mockClient) UsingDatabase(ctx context.Context, dbName string) error {
	panic("implement me")
}

func (m *mockClient) ListDatabases(ctx context.Context) ([]entity.Database, error) {
	panic("implement me")
}

func (m *mockClient) CreateDatabase(ctx context.Context, dbName string, opts ...client.CreateDatabaseOption) error {
	panic("implement me")
}

func (m *mockClient) DropDatabase(ctx context.Context, dbName string, opts ...client.DropDatabaseOption) error {
	panic("implement me")
}

func (m *mockClient) AlterDatabase(ctx context.Context, dbName string, attrs ...entity.DatabaseAttribute) error {
	panic("implement me")
}

func (m *mockClient) DescribeDatabase(ctx context.Context, dbName string) (*entity.Database, error) {
	panic("implement me")
}

func (m *mockClient) NewCollection(ctx context.Context, collName string, dimension int64, opts ...client.CreateCollectionOption) error {
	panic("implement me")
}

func (m *mockClient) ListCollections(ctx context.Context, opts ...client.ListCollectionOption) ([]*entity.Collection, error) {
	panic("implement me")
}

func (m *mockClient) CreateCollection(ctx context.Context, schema *entity.Schema, shardsNum int32, opts ...client.CreateCollectionOption) error {
	return nil
}

func (m *mockClient) DescribeCollection(ctx context.Context, collName string) (*entity.Collection, error) {
	if collName != defaultCollection {
		return nil, client.ErrClientNotReady
	}
	return &entity.Collection{
		ID:   0,
		Name: defaultCollection,
		Schema: &entity.Schema{
			CollectionName: collName,
			Description:    defaultDescription,
			AutoID:         false,
			Fields: []*entity.Field{
				{
					ID:              0,
					Name:            defaultCollectionID,
					PrimaryKey:      false,
					AutoID:          false,
					Description:     defaultDescription,
					DataType:        entity.FieldTypeVarChar,
					TypeParams:      nil,
					IndexParams:     nil,
					IsDynamic:       false,
					IsPartitionKey:  false,
					IsClusteringKey: false,
					ElementType:     0,
				},
				{
					ID:              1,
					Name:            defaultCollectionVector,
					PrimaryKey:      false,
					AutoID:          false,
					Description:     defaultCollectionVectorDesc,
					DataType:        entity.FieldTypeBinaryVector,
					TypeParams:      nil,
					IndexParams:     nil,
					IsDynamic:       false,
					IsPartitionKey:  false,
					IsClusteringKey: false,
					ElementType:     0,
				},
				{
					ID:              2,
					Name:            defaultCollectionContent,
					PrimaryKey:      false,
					AutoID:          false,
					Description:     defaultCollectionContentDesc,
					DataType:        entity.FieldTypeVarChar,
					TypeParams:      nil,
					IndexParams:     nil,
					IsDynamic:       false,
					IsPartitionKey:  false,
					IsClusteringKey: false,
					ElementType:     0,
				},
				{
					ID:              3,
					Name:            defaultCollectionMetadata,
					PrimaryKey:      false,
					AutoID:          false,
					Description:     defaultCollectionMetadataDesc,
					DataType:        entity.FieldTypeJSON,
					TypeParams:      nil,
					IndexParams:     nil,
					IsDynamic:       false,
					IsPartitionKey:  false,
					IsClusteringKey: false,
					ElementType:     0,
				},
			},
			EnableDynamicField: false,
		},
		PhysicalChannels: nil,
		VirtualChannels:  nil,
		Loaded:           false,
		ConsistencyLevel: entity.DefaultConsistencyLevel,
		ShardNum:         1,
		Properties:       nil,
	}, nil
}

func (m *mockClient) DropCollection(ctx context.Context, collName string, opts ...client.DropCollectionOption) error {
	panic("implement me")
}

func (m *mockClient) GetCollectionStatistics(ctx context.Context, collName string) (map[string]string, error) {
	panic("implement me")
}

func (m *mockClient) LoadCollection(ctx context.Context, collName string, async bool, opts ...client.LoadCollectionOption) error {
	if collName != defaultCollection {
		return client.ErrClientNotReady
	}
	return nil
}

func (m *mockClient) ReleaseCollection(ctx context.Context, collName string, opts ...client.ReleaseCollectionOption) error {
	panic("implement me")
}

func (m *mockClient) HasCollection(ctx context.Context, collName string) (bool, error) {
	if collName == defaultCollection {
		return true, nil
	}
	return false, nil
}

func (m *mockClient) RenameCollection(ctx context.Context, collName, newName string) error {

	panic("implement me")
}

func (m *mockClient) AlterCollection(ctx context.Context, collName string, attrs ...entity.CollectionAttribute) error {
	panic("implement me")
}

func (m *mockClient) CreateAlias(ctx context.Context, collName string, alias string) error {

	panic("implement me")
}

func (m *mockClient) DropAlias(ctx context.Context, alias string) error {

	panic("implement me")
}

func (m *mockClient) AlterAlias(ctx context.Context, collName string, alias string) error {

	panic("implement me")
}

func (m *mockClient) GetReplicas(ctx context.Context, collName string) ([]*entity.ReplicaGroup, error) {
	panic("implement me")
}

func (m *mockClient) BackupRBAC(ctx context.Context) (*entity.RBACMeta, error) {

	panic("implement me")
}

func (m *mockClient) RestoreRBAC(ctx context.Context, meta *entity.RBACMeta) error {

	panic("implement me")
}

func (m *mockClient) CreateCredential(ctx context.Context, username string, password string) error {

	panic("implement me")
}

func (m *mockClient) UpdateCredential(ctx context.Context, username string, oldPassword string, newPassword string) error {

	panic("implement me")
}

func (m *mockClient) DeleteCredential(ctx context.Context, username string) error {

	panic("implement me")
}

func (m *mockClient) ListCredUsers(ctx context.Context) ([]string, error) {

	panic("implement me")
}

func (m *mockClient) CreatePartition(ctx context.Context, collName string, partitionName string, opts ...client.CreatePartitionOption) error {

	panic("implement me")
}

func (m *mockClient) DropPartition(ctx context.Context, collName string, partitionName string, opts ...client.DropPartitionOption) error {

	panic("implement me")
}

func (m *mockClient) ShowPartitions(ctx context.Context, collName string) ([]*entity.Partition, error) {

	panic("implement me")
}

func (m *mockClient) HasPartition(ctx context.Context, collName string, partitionName string) (bool, error) {

	panic("implement me")
}

func (m *mockClient) LoadPartitions(ctx context.Context, collName string, partitionNames []string, async bool, opts ...client.LoadPartitionsOption) error {

	panic("implement me")
}

func (m *mockClient) ReleasePartitions(ctx context.Context, collName string, partitionNames []string, opts ...client.ReleasePartitionsOption) error {

	panic("implement me")
}

func (m *mockClient) GetPersistentSegmentInfo(ctx context.Context, collName string) ([]*entity.Segment, error) {

	panic("implement me")
}

func (m *mockClient) CreateIndex(ctx context.Context, collName string, fieldName string, idx entity.Index, async bool, opts ...client.IndexOption) error {
	if collName != defaultCollection && fieldName != defaultCollectionVector {
		return client.ErrClientNotReady
	}
	return nil
}

func (m *mockClient) DescribeIndex(ctx context.Context, collName string, fieldName string, opts ...client.IndexOption) ([]entity.Index, error) {
	if collName != defaultCollection && fieldName != defaultCollectionVector {
		return nil, client.ErrClientNotReady
	}
	return []entity.Index{}, nil
}

func (m *mockClient) DropIndex(ctx context.Context, collName string, fieldName string, opts ...client.IndexOption) error {

	panic("implement me")
}

func (m *mockClient) GetIndexState(ctx context.Context, collName string, fieldName string, opts ...client.IndexOption) (entity.IndexState, error) {

	panic("implement me")
}

func (m *mockClient) AlterIndex(ctx context.Context, collName, indexName string, opts ...client.IndexOption) error {

	panic("implement me")
}

func (m *mockClient) GetIndexBuildProgress(ctx context.Context, collName string, fieldName string, opts ...client.IndexOption) (total, indexed int64, err error) {

	panic("implement me")
}

func (m *mockClient) Insert(ctx context.Context, collName string, partitionName string, columns ...entity.Column) (entity.Column, error) {

	panic("implement me")
}

func (m *mockClient) Flush(ctx context.Context, collName string, async bool, opts ...client.FlushOption) error {
	if collName != defaultCollection {
		return client.ErrClientNotReady
	}
	return nil
}

func (m *mockClient) FlushV2(ctx context.Context, collName string, async bool, opts ...client.FlushOption) ([]int64, []int64, int64, map[string]msgpb.MsgPosition, error) {

	panic("implement me")
}

func (m *mockClient) DeleteByPks(ctx context.Context, collName string, partitionName string, ids entity.Column) error {

	panic("implement me")
}

func (m *mockClient) Delete(ctx context.Context, collName string, partitionName string, expr string) error {

	panic("implement me")
}

func (m *mockClient) Upsert(ctx context.Context, collName string, partitionName string, columns ...entity.Column) (entity.Column, error) {

	panic("implement me")
}

func (m *mockClient) Search(ctx context.Context, collName string, partitions []string, expr string, outputFields []string, vectors []entity.Vector, vectorField string, metricType entity.MetricType, topK int, sp entity.SearchParam, opts ...client.SearchQueryOptionFunc) ([]client.SearchResult, error) {

	panic("implement me")
}

func (m *mockClient) QueryByPks(ctx context.Context, collectionName string, partitionNames []string, ids entity.Column, outputFields []string, opts ...client.SearchQueryOptionFunc) (client.ResultSet, error) {

	panic("implement me")
}

func (m *mockClient) Query(ctx context.Context, collectionName string, partitionNames []string, expr string, outputFields []string, opts ...client.SearchQueryOptionFunc) (client.ResultSet, error) {

	panic("implement me")
}

func (m *mockClient) Get(ctx context.Context, collectionName string, ids entity.Column, opts ...client.GetOption) (client.ResultSet, error) {

	panic("implement me")
}

func (m *mockClient) QueryIterator(ctx context.Context, opt *client.QueryIteratorOption) (*client.QueryIterator, error) {

	panic("implement me")
}

func (m *mockClient) CalcDistance(ctx context.Context, collName string, partitions []string, metricType entity.MetricType, opLeft, opRight entity.Column) (entity.Column, error) {

	panic("implement me")
}

func (m *mockClient) CreateCollectionByRow(ctx context.Context, row entity.Row, shardNum int32) error {

	panic("implement me")
}

func (m *mockClient) InsertByRows(ctx context.Context, collName string, paritionName string, rows []entity.Row) (entity.Column, error) {

	panic("implement me")
}

func (m *mockClient) InsertRows(ctx context.Context, collName string, partitionName string, rows []interface{}) (entity.Column, error) {
	if collName != defaultCollection || partitionName != "" {
		return nil, client.ErrClientNotReady
	}
	return entity.NewColumnVarChar("id", []string{"1", "2"}), nil
}

func (m *mockClient) ManualCompaction(ctx context.Context, collName string, toleranceDuration time.Duration) (int64, error) {

	panic("implement me")
}

func (m *mockClient) GetCompactionState(ctx context.Context, id int64) (entity.CompactionState, error) {

	panic("implement me")
}

func (m *mockClient) GetCompactionStateWithPlans(ctx context.Context, id int64) (entity.CompactionState, []entity.CompactionPlan, error) {

	panic("implement me")
}

func (m *mockClient) BulkInsert(ctx context.Context, collName string, partitionName string, files []string, opts ...client.BulkInsertOption) (int64, error) {

	panic("implement me")
}

func (m *mockClient) GetBulkInsertState(ctx context.Context, taskID int64) (*entity.BulkInsertTaskState, error) {

	panic("implement me")
}

func (m *mockClient) ListBulkInsertTasks(ctx context.Context, collName string, limit int64) ([]*entity.BulkInsertTaskState, error) {

	panic("implement me")
}

func (m *mockClient) CreateRole(ctx context.Context, name string) error {

	panic("implement me")
}

func (m *mockClient) DropRole(ctx context.Context, name string) error {

	panic("implement me")
}

func (m *mockClient) AddUserRole(ctx context.Context, username string, role string) error {

	panic("implement me")
}

func (m *mockClient) RemoveUserRole(ctx context.Context, username string, role string) error {

	panic("implement me")
}

func (m *mockClient) ListRoles(ctx context.Context) ([]entity.Role, error) {

	panic("implement me")
}

func (m *mockClient) ListUsers(ctx context.Context) ([]entity.User, error) {

	panic("implement me")
}

func (m *mockClient) DescribeUser(ctx context.Context, username string) (entity.UserDescription, error) {

	panic("implement me")
}

func (m *mockClient) DescribeUsers(ctx context.Context) ([]entity.UserDescription, error) {

	panic("implement me")
}

func (m *mockClient) ListGrant(ctx context.Context, role string, object string, objectName string, dbName string) ([]entity.RoleGrants, error) {

	panic("implement me")
}

func (m *mockClient) ListGrants(ctx context.Context, role string, dbName string) ([]entity.RoleGrants, error) {

	panic("implement me")
}

func (m *mockClient) Grant(ctx context.Context, role string, objectType entity.PriviledgeObjectType, object string, privilege string, options ...entity.OperatePrivilegeOption) error {

	panic("implement me")
}

func (m *mockClient) Revoke(ctx context.Context, role string, objectType entity.PriviledgeObjectType, object string, privilege string, options ...entity.OperatePrivilegeOption) error {

	panic("implement me")
}

func (m *mockClient) GetLoadingProgress(ctx context.Context, collectionName string, partitionNames []string) (int64, error) {

	panic("implement me")
}

func (m *mockClient) GetLoadState(ctx context.Context, collectionName string, partitionNames []string) (entity.LoadState, error) {
	if collectionName != defaultCollection {
		return entity.LoadStateNotExist, nil
	}
	return entity.LoadStateNotLoad, nil
}

func (m *mockClient) ListResourceGroups(ctx context.Context) ([]string, error) {

	panic("implement me")
}

func (m *mockClient) CreateResourceGroup(ctx context.Context, rgName string, opts ...client.CreateResourceGroupOption) error {

	panic("implement me")
}

func (m *mockClient) UpdateResourceGroups(ctx context.Context, opts ...client.UpdateResourceGroupsOption) error {

	panic("implement me")
}

func (m *mockClient) DescribeResourceGroup(ctx context.Context, rgName string) (*entity.ResourceGroup, error) {

	panic("implement me")
}

func (m *mockClient) DropResourceGroup(ctx context.Context, rgName string) error {

	panic("implement me")
}

func (m *mockClient) TransferNode(ctx context.Context, sourceRg, targetRg string, nodesNum int32) error {

	panic("implement me")
}

func (m *mockClient) TransferReplica(ctx context.Context, sourceRg, targetRg string, collectionName string, replicaNum int64) error {

	panic("implement me")
}

func (m *mockClient) GetVersion(ctx context.Context) (string, error) {

	panic("implement me")
}

func (m *mockClient) CheckHealth(ctx context.Context) (*entity.MilvusState, error) {

	panic("implement me")
}

func (m *mockClient) ReplicateMessage(ctx context.Context, channelName string, beginTs, endTs uint64, msgsBytes [][]byte, startPositions, endPositions []*msgpb.MsgPosition, opts ...client.ReplicateMessageOption) (*entity.MessageInfo, error) {

	panic("implement me")
}

func (m *mockClient) HybridSearch(ctx context.Context, collName string, partitions []string, limit int, outputFields []string, reranker client.Reranker, subRequests []*client.ANNSearchRequest, opts ...client.SearchQueryOptionFunc) ([]client.SearchResult, error) {

	panic("implement me")
}

type mockEmbedding struct {
	err         error
	cnt         int
	sizeForCall []int
	dims        int
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.cnt > len(m.sizeForCall) {
		log.Fatal("unexpected")
	}

	if m.err != nil {
		return nil, m.err
	}

	slice := make([]float64, m.dims)
	for i := range slice {
		slice[i] = 1.1
	}

	r := make([][]float64, m.sizeForCall[m.cnt])
	m.cnt++
	for i := range r {
		r[i] = slice
	}

	return r, nil
}
