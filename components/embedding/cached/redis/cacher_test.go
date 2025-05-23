package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/cached"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRedisClient struct {
	redis.UniversalClient
	mock.Mock
}

var _ redis.UniversalClient = (*mockRedisClient)(nil)

func (m *mockRedisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal(args.String(0))
	cmd.SetErr(args.Error(1))
	return cmd
}

func (m *mockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	cmd := redis.NewStringCmd(ctx)
	cmd.SetVal(args.String(0))
	cmd.SetErr(args.Error(1))
	return cmd
}

func TestCacher(t *testing.T) {
	ctx := context.Background()
	key := "test_key"
	value := []float64{1.1, 2.2, 3.3}
	expire := time.Second * 10

	valueBytes, err := defaultCodec.Marshal(value)
	require.NoError(t, err)

	t.Run("Set and Get", func(t *testing.T) {
		mockRdb := new(mockRedisClient)
		c := NewCacher(mockRdb)

		mockRdb.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("OK", nil)
		mockRdb.On("Get", mock.Anything, mock.Anything).Return(string(valueBytes), nil)

		err = c.Set(ctx, key, value, expire)
		assert.NoError(t, err)

		data, err := c.Get(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, value, data)

		mockRdb.AssertExpectations(t)
	})

	t.Run("Get Not Found", func(t *testing.T) {
		mockRdb := new(mockRedisClient)
		c := NewCacher(mockRdb)

		mockRdb.On("Get", mock.Anything, mock.Anything).Return("", redis.Nil)

		data, err := c.Get(ctx, key)
		assert.Error(t, err)
		assert.Equal(t, cached.ErrNotFound, err)
		assert.Nil(t, data)

		mockRdb.AssertExpectations(t)
	})

	t.Run("Get and Set Error", func(t *testing.T) {
		mockRdb := new(mockRedisClient)
		c := NewCacher(mockRdb)
		setErr := errors.New("set error")
		getErr := errors.New("get error")

		mockRdb.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", setErr)
		mockRdb.On("Get", mock.Anything, mock.Anything).Return("", getErr)

		err = c.Set(ctx, key, value, expire)
		assert.Error(t, err)
		assert.Equal(t, setErr, err)

		data, err := c.Get(ctx, key)
		assert.Error(t, err)
		assert.Nil(t, data)
		assert.Equal(t, getErr, err)

		mockRdb.AssertExpectations(t)
	})
}

func TestWithPrefix(t *testing.T) {
	assert.Equal(t, "eino:", NewCacher(nil).prefix)
	assert.Equal(t, "custom:", NewCacher(nil, WithPrefix("custom:")).prefix)
	assert.Equal(t, "custom:", NewCacher(nil, WithPrefix("custom")).prefix)
}
