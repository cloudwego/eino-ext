package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/a2a/models"
)

func TestInMemoryEventQueue(t *testing.T) {
	eq := newInMemoryEventQueue()
	ctx := context.Background()
	assert.NoError(t, eq.Reset(ctx, "1"))
	assert.NoError(t, eq.Push(ctx, "1", &models.SendMessageStreamingResponseUnion{Message: &models.Message{Role: "1"}}, nil))
	assert.NoError(t, eq.Push(ctx, "1", &models.SendMessageStreamingResponseUnion{Message: &models.Message{Role: "2"}}, nil))
	assert.NoError(t, eq.Push(ctx, "1", nil, fmt.Errorf("test error")))
	assert.NoError(t, eq.Push(ctx, "1", &models.SendMessageStreamingResponseUnion{Message: &models.Message{Role: "3"}}, nil))
	assert.NoError(t, eq.Close(ctx, "1"))

	e, taskErr, closed, err := eq.Pop(ctx, "1")
	assert.NoError(t, err)
	assert.False(t, closed)
	assert.Nil(t, taskErr)
	assert.Equal(t, models.Role("1"), e.Message.Role)
	e, taskErr, closed, err = eq.Pop(ctx, "1")
	assert.NoError(t, err)
	assert.False(t, closed)
	assert.Nil(t, taskErr)
	assert.Equal(t, models.Role("2"), e.Message.Role)
	e, taskErr, closed, err = eq.Pop(ctx, "1")
	assert.NoError(t, err)
	assert.False(t, closed)
	assert.ErrorContains(t, taskErr, "test error")
	assert.Nil(t, e)
	e, taskErr, closed, err = eq.Pop(ctx, "1")
	assert.NoError(t, err)
	assert.False(t, closed)
	assert.Nil(t, taskErr)
	assert.Equal(t, models.Role("3"), e.Message.Role)
}
