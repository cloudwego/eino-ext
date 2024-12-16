// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package langfuse

import (
	"sync"
	"time"
)

const (
	defaultMaxSize = 100
)

func newQueue(maxSize int) *queue {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}
	return &queue{
		data:  make(chan *event, maxSize),
		empty: sync.NewCond(&sync.Mutex{}),
	}
}

type queue struct {
	data       chan *event
	empty      *sync.Cond
	unfinished int
}

func (q *queue) put(value *event) bool {
	q.empty.L.Lock()
	defer q.empty.L.Unlock()
	for {
		select {
		case q.data <- value:
			q.unfinished++
			return true
		default:
			return false
		}
	}
}

func (q *queue) get(timeout time.Duration) (*event, bool) {
	select {
	case v := <-q.data:
		return v, true
	case <-time.After(timeout):
		return nil, false
	}
}

func (q *queue) done() {
	q.empty.L.Lock()
	defer q.empty.L.Unlock()
	q.unfinished--
	if q.unfinished == 0 {
		q.empty.Broadcast()
	}
}

func (q *queue) join() {
	q.empty.L.Lock()
	defer q.empty.L.Unlock()
	for q.unfinished > 0 {
		q.empty.Wait()
	}
}
