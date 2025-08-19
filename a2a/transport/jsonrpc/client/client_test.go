/*
 * Copyright 2025 CloudWeGo Authors
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

package client

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	// missing URL
	cli, err := NewClient()
	assert.True(t, errors.Is(err, errNoURL))
	// specify URL
	testURL := "http://127.0.0.1/testing"
	cli, err = NewClient(WithURL(testURL))
	assert.Nil(t, err)
	assert.NotNil(t, cli)
	assert.Equal(t, testURL, cli.remotePeer.Address())
	assert.NotNil(t, cli.hdl)
	// specify URL and TransportHandler
	testHdl := &testClientTransportHandler{}
	cli, err = NewClient(WithURL(testURL), WithTransportHandler(testHdl))
	assert.Nil(t, err)
	assert.NotNil(t, cli)
	assert.Equal(t, testHdl, cli.hdl)
}
