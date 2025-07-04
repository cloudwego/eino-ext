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

package duckduckgo

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

func Test_buildClient(t *testing.T) {
	mockey.PatchConvey("Test buildClient", t, func() {
		ctx := context.Background()

		mockey.PatchConvey("nil config", func() {
			search, err := buildClient(ctx, nil)
			assert.NoError(t, err)

			cli, ok := search.(*client)
			assert.True(t, ok)

			assert.Equal(t, RegionWT, cli.region)
			assert.Equal(t, 10, cli.maxResults)

			assert.NotNil(t, cli.httpCli)
			assert.Equal(t, 30*time.Second, cli.httpCli.Timeout)
		})

		customConfig := &Config{
			Timeout:    15 * time.Second,
			MaxResults: 20,
			Region:     RegionUS,
		}

		search, err := buildClient(ctx, customConfig)
		assert.NoError(t, err)

		cli, ok := search.(*client)
		assert.True(t, ok)

		assert.Equal(t, RegionUS, cli.region)
		assert.Equal(t, 20, cli.maxResults)

		assert.NotNil(t, cli.httpCli)
		assert.Equal(t, 15*time.Second, cli.httpCli.Timeout)
	})
}
