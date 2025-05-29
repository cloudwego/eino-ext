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
