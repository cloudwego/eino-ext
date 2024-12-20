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

package safego

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

func TestGo(t *testing.T) {
	PatchConvey("TestGo", t, func() {
		c := int64(0)
		fn := func() { atomic.AddInt64(&c, 1) }
		Go(context.Background(), fn)
		time.Sleep(1 * time.Millisecond)
		assert.Equal(t, atomic.LoadInt64(&c), int64(1))
	})
}
