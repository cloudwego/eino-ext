package wikipediaclient

import (
	"context"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient(
		WithHTTPClient(&http.Client{Timeout: 15 * time.Second}),
		WithLanguage("en"),
		WithUserAgent("eino"),
	)
	assert.NotNil(t, c)
	c = NewClient(
		WithLanguage("en"),
		WithUserAgent("eino"),
	)
	assert.NotNil(t, c)
}

func TestSearchAndGetPage(t *testing.T) {
	c := NewClient(
		WithHTTPClient(&http.Client{Timeout: 15 * time.Second}),
		WithLanguage("en"),
		WithUserAgent("eino"),
	)

	results, err := c.Search(context.Background(), "bytedance")
	assert.NoError(t, err)
	assert.NotNil(t, results)

	results, err = c.Search(context.Background(), "")
	assert.Error(t, err, ErrInvalidParameters)
	assert.Nil(t, results)

	for _, result := range results {
		pr, err := c.GetPage(context.Background(), result.Title)
		assert.NoError(t, err)
		assert.NotNil(t, pr)
	}

	pr, err := c.GetPage(context.Background(), "xxxxxxxxx")
	assert.Error(t, err, ErrPageNotFound)
	assert.Nil(t, pr)

}
