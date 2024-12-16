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
	"errors"
	"net/http"
	"sync"
	"time"
)

func newTaskManager(
	threads int,
	cli *http.Client,
	host string,
	maxTaskQueueSize int,
	flushAt int,
	flushInterval time.Duration,
	sampleRate float64,
	logMessage string,
	maskFunc func(string) string,
	sdkName string,
	sdkVersion string,
	sdkIntegration string,
	publicKey string,
	secretKey string,
	maxRetry uint64,
) *taskManager {
	langfuseCli := newClient(cli, host, publicKey, secretKey, sdkVersion)
	q := newQueue(maxTaskQueueSize)
	if threads < 1 {
		threads = 1
	}
	wg := &sync.WaitGroup{}
	for i := 0; i < threads; i++ {
		newIngestionConsumer(langfuseCli, q, flushAt, flushInterval, sampleRate, logMessage, maskFunc, sdkName, sdkVersion, sdkIntegration, publicKey, maxRetry, wg).run()
	}

	return &taskManager{q: q, mediaWG: wg}
}

type taskManager struct {
	q       *queue
	mediaWG *sync.WaitGroup
}

func (t *taskManager) push(e *event) error {
	e.TimeStamp = time.Now()
	success := t.q.put(e)
	if !success {
		return errors.New("event send queue is full")
	}
	return nil
}

func (t *taskManager) flush() {
	t.q.join()
	t.mediaWG.Wait()
}
