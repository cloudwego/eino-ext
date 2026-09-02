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
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultDropLogInterval = time.Minute

const (
	lossQueueFull             = "queue_full_spans"
	lossProcessorStopped      = "processor_stopped_spans"
	lossSampledOut            = "sampled_out_spans"
	lossValueLimit            = "value_limit_truncations"
	lossSpanAttributeBudget   = "span_attribute_budget_drops_or_truncations"
	lossOTelDroppedAttributes = "otel_dropped_attributes"
	lossOTelDroppedEvents     = "otel_dropped_events"
	lossOTelDroppedLinks      = "otel_dropped_links"
	lossMetadataSerialization = "metadata_serialization_failures"
	lossPayloadSerialization  = "payload_serialization_failures"
)

type lossReporter struct {
	mu       sync.Mutex
	counts   map[string]uint64
	interval time.Duration
	report   func(map[string]uint64, time.Duration)
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopped  bool
	stopOnce sync.Once
}

func newLossReporter(interval time.Duration, report func(map[string]uint64, time.Duration)) *lossReporter {
	if interval == 0 {
		interval = defaultDropLogInterval
	}
	if report == nil {
		report = logTelemetryLosses
	}
	r := &lossReporter{
		counts:   make(map[string]uint64),
		interval: interval,
		report:   report,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				recoveredPanic("telemetry loss reporter", recovered)
			}
		}()
		r.run()
	}()
	return r
}

func (r *lossReporter) Add(reason string, count uint64) {
	if r == nil || count == 0 {
		return
	}
	r.mu.Lock()
	if r.interval < 0 {
		r.mu.Unlock()
		r.report(map[string]uint64{reason: count}, r.interval)
		return
	}
	if !r.stopped {
		r.counts[reason] += count
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	r.report(map[string]uint64{reason: count}, r.interval)
}

func (r *lossReporter) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		r.mu.Lock()
		r.stopped = true
		r.mu.Unlock()
		close(r.stopCh)
		<-r.doneCh
	})
}

func (r *lossReporter) run() {
	defer close(r.doneCh)
	if r.interval < 0 {
		<-r.stopCh
		return
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.flush()
		case <-r.stopCh:
			r.flush()
			return
		}
	}
}

func (r *lossReporter) flush() {
	r.mu.Lock()
	if len(r.counts) == 0 {
		r.mu.Unlock()
		return
	}
	counts := r.counts
	r.counts = make(map[string]uint64)
	r.mu.Unlock()
	r.report(counts, r.interval)
}

func logTelemetryLosses(counts map[string]uint64, interval time.Duration) {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+formatUint(counts[key]))
	}
	log.Printf("langfuse callback discarded telemetry since previous report (interval %s): %s", interval, strings.Join(parts, " "))
}

func formatUint(value uint64) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = digits[value%10]
		value /= 10
	}
	return string(buffer[index:])
}
