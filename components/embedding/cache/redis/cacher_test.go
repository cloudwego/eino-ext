/*
 * Copyright 2024 CloudWeGo Authors
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

package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRedisClient struct {
	redis.UniversalClient
	mock.Mock
}

type mockCodec struct {
	codec
	mock.Mock
}

func (m *mockCodec) Marshal(v any) ([]byte, error) {
	args := m.Called(v)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockCodec) Unmarshal(data []byte, v any) error {
	args := m.Called(data, v)
	return args.Error(0)
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

		data, ok, err := c.Get(ctx, key)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, value, data)

		mockRdb.AssertExpectations(t)
	})

	t.Run("Get Not Found", func(t *testing.T) {
		mockRdb := new(mockRedisClient)
		c := NewCacher(mockRdb)

		mockRdb.On("Get", mock.Anything, mock.Anything).Return("", redis.Nil)

		data, ok, err := c.Get(ctx, key)
		assert.NoError(t, err)
		assert.False(t, ok)
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

		data, ok, err := c.Get(ctx, key)
		assert.Error(t, err)
		assert.False(t, ok)
		assert.Nil(t, data)
		assert.Equal(t, getErr, err)

		mockRdb.AssertExpectations(t)
	})

	t.Run("marshal and unmarshal error", func(t *testing.T) {
		mockRdb := new(mockRedisClient)
		mc := new(mockCodec)
		c := NewCacher(mockRdb)
		c.codec = mc

		mockRdb.On("Get", mock.Anything, mock.Anything).Return(string(valueBytes), nil)
		mc.On("Marshal", value).Return(nil, errors.New("marshal error"))
		mc.On("Unmarshal", mock.Anything, mock.Anything).Return(errors.New("unmarshal error"))

		// Simulate marshal error
		err = c.Set(ctx, key, value, expire)
		assert.Error(t, err)
		assert.Equal(t, "marshal error", err.Error())

		// Simulate unmarshal error
		data, ok, err := c.Get(ctx, key)
		assert.Error(t, err)
		assert.False(t, ok)
		assert.Nil(t, data)
		assert.Equal(t, "unmarshal error", err.Error())

		mockRdb.AssertExpectations(t)
		mc.AssertExpectations(t)
	})
}

func TestWithPrefix(t *testing.T) {
	assert.Equal(t, "eino:", NewCacher(nil).prefix)
	assert.Equal(t, "custom:", NewCacher(nil, WithPrefix("custom:")).prefix)
	assert.Equal(t, "custom:", NewCacher(nil, WithPrefix("custom")).prefix)
}
