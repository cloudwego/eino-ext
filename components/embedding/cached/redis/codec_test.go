package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodec_Sonic(t *testing.T) {
	c := &sonicCodec{}
	v := []float64{
		1.0, 2.0, 3.0,
	}

	data, err := c.Marshal(v)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var out []float64
	err = c.Unmarshal(data, &out)
	require.NoError(t, err)
	assert.Equal(t, v, out)
}

func TestCodec_Default(t *testing.T) {
	assert.Equal(t, &sonicCodec{}, defaultCodec)
}
