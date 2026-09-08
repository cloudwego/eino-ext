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

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestLossReporterAggregatesAllReasonsAndFlushesOnStop(t *testing.T) {
	reports := make(chan map[string]uint64, 1)
	reporter := newLossReporter(time.Hour, func(counts map[string]uint64, _ time.Duration) {
		reports <- counts
	})
	reporter.Add(lossQueueFull, 2)
	reporter.Add(lossValueLimit, 3)
	reporter.Stop()

	counts := <-reports
	if counts[lossQueueFull] != 2 || counts[lossValueLimit] != 3 {
		t.Fatalf("unexpected loss counts: %#v", counts)
	}
}

func TestLossReporterReportsAfterStop(t *testing.T) {
	reports := make(chan map[string]uint64, 1)
	reporter := newLossReporter(time.Hour, func(counts map[string]uint64, _ time.Duration) {
		reports <- counts
	})
	reporter.Stop()
	reporter.Add(lossProcessorStopped, 1)

	counts := <-reports
	if counts[lossProcessorStopped] != 1 {
		t.Fatalf("unexpected loss counts: %#v", counts)
	}
}

func TestLossReporterReportsImmediatelyForNegativeInterval(t *testing.T) {
	reports := make(chan map[string]uint64, 1)
	reporter := newLossReporter(-1, func(counts map[string]uint64, _ time.Duration) {
		reports <- counts
	})
	defer reporter.Stop()
	reporter.Add(lossQueueFull, 1)

	select {
	case counts := <-reports:
		if counts[lossQueueFull] != 1 {
			t.Fatalf("unexpected loss counts: %#v", counts)
		}
	case <-time.After(time.Second):
		t.Fatal("loss was not reported immediately")
	}
}

func TestReportingSamplerCountsDroppedSpans(t *testing.T) {
	reports := make(chan map[string]uint64, 1)
	reporter := newLossReporter(time.Hour, func(counts map[string]uint64, _ time.Duration) {
		reports <- counts
	})
	sampler := reportingSampler{delegate: sdktrace.NeverSample(), losses: reporter}
	result := sampler.ShouldSample(sdktrace.SamplingParameters{ParentContext: context.Background()})
	if result.Decision != sdktrace.Drop {
		t.Fatalf("sampling decision = %v, want drop", result.Decision)
	}
	reporter.Stop()
	if counts := <-reports; counts[lossSampledOut] != 1 {
		t.Fatalf("unexpected loss counts: %#v", counts)
	}
}

func TestValuePreparerCountsTruncation(t *testing.T) {
	reports := make(chan map[string]uint64, 1)
	reporter := newLossReporter(time.Hour, func(counts map[string]uint64, _ time.Duration) {
		reports <- counts
	})
	prepare := newValuePreparer(nil, 3, reporter)
	if got := prepare("abcdef"); len(got) > 3 {
		t.Fatalf("prepared value length = %d", len(got))
	}
	reporter.Stop()
	if counts := <-reports; counts[lossValueLimit] != 1 {
		t.Fatalf("unexpected loss counts: %#v", counts)
	}
}
