/*
 * Copyright 2026 CloudWeGo Authors
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

package langfuse

import (
	"context"
	"testing"
	"time"
)

func TestAsyncGroupRecoversPanicAndBecomesIdle(t *testing.T) {
	group := newAsyncGroup()
	if !group.run(func() { panic("async boom") }) {
		t.Fatal("expected async callback to start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := group.wait(ctx, true); err != nil {
		t.Fatalf("async group did not become idle after a recovered panic: %v", err)
	}
}

func TestLossReporterRecoversReportPanicAndStops(t *testing.T) {
	reporter := newLossReporter(time.Hour, func(map[string]uint64, time.Duration) {
		panic("report boom")
	})
	reporter.Add(lossQueueFull, 1)

	stopped := make(chan struct{})
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("Stop propagated panic: %v", recovered)
			}
			close(stopped)
		}()
		reporter.Stop()
	}()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("loss reporter Stop blocked after a recovered panic")
	}
}
