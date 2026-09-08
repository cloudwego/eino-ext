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
	"compress/gzip"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"
)

type exportDiagnosticsContextKey struct{}

type exportDiagnostics struct {
	mu             sync.Mutex
	protobufBytes  int64
	gzipBytes      int64
	bodyMeasured   bool
	bodyMeasureErr error
	attempts       []exportAttemptDiagnostics
}

type exportAttemptDiagnostics struct {
	total       time.Duration
	dns         time.Duration
	connect     time.Duration
	tls         time.Duration
	write       time.Duration
	waitHeaders time.Duration
	reused      bool
	statusCode  int
}

type exportAttemptRecorder struct {
	mu           sync.Mutex
	start        time.Time
	dnsStart     time.Time
	connectStart time.Time
	tlsStart     time.Time
	gotConn      time.Time
	wroteRequest time.Time
	gotFirstByte time.Time
	dns          time.Duration
	connect      time.Duration
	tls          time.Duration
	reused       bool
}

type diagnosticRoundTripper struct {
	base http.RoundTripper
}

func newDiagnosticHTTPClient(configured *http.Client, timeout time.Duration) *http.Client {
	client := &http.Client{Timeout: timeout}
	if configured != nil {
		clientCopy := *configured
		client = &clientCopy
	}
	base := client.Transport
	if base == nil {
		base = newDefaultDiagnosticTransport()
	}
	client.Transport = &diagnosticRoundTripper{base: base}
	return client
}

func newDefaultDiagnosticTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

func (t *diagnosticRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	diagnostics, _ := request.Context().Value(exportDiagnosticsContextKey{}).(*exportDiagnostics)
	if diagnostics == nil {
		return t.base.RoundTrip(request)
	}

	diagnostics.measureBody(request)
	recorder := &exportAttemptRecorder{start: time.Now()}
	request = request.Clone(httptrace.WithClientTrace(request.Context(), recorder.clientTrace()))
	response, err := t.base.RoundTrip(request)
	diagnostics.addAttempt(recorder.finish(response))
	return response, err
}

func (r *exportAttemptRecorder) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.dnsStart = time.Now()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if !r.dnsStart.IsZero() {
				r.dns = time.Since(r.dnsStart)
			}
		},
		ConnectStart: func(_, _ string) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if !r.connectStart.IsZero() {
				r.connect = time.Since(r.connectStart)
			}
		},
		TLSHandshakeStart: func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if !r.tlsStart.IsZero() {
				r.tls = time.Since(r.tlsStart)
			}
		},
		GotConn: func(info httptrace.GotConnInfo) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.gotConn = time.Now()
			r.reused = info.Reused
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.wroteRequest = time.Now()
		},
		GotFirstResponseByte: func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.gotFirstByte = time.Now()
		},
	}
}

func (r *exportAttemptRecorder) finish(response *http.Response) exportAttemptDiagnostics {
	r.mu.Lock()
	defer r.mu.Unlock()
	finished := time.Now()
	attempt := exportAttemptDiagnostics{
		total:   finished.Sub(r.start),
		dns:     r.dns,
		connect: r.connect,
		tls:     r.tls,
		reused:  r.reused,
	}
	if response != nil {
		attempt.statusCode = response.StatusCode
	}
	if !r.gotConn.IsZero() && !r.wroteRequest.IsZero() {
		attempt.write = r.wroteRequest.Sub(r.gotConn)
	}
	if !r.wroteRequest.IsZero() {
		until := r.gotFirstByte
		if until.IsZero() {
			until = finished
		}
		attempt.waitHeaders = until.Sub(r.wroteRequest)
	}
	return attempt
}

func (d *exportDiagnostics) measureBody(request *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.bodyMeasured {
		return
	}
	d.bodyMeasured = true

	if request.GetBody == nil {
		d.bodyMeasureErr = fmt.Errorf("request body cannot be replayed")
		return
	}
	body, err := request.GetBody()
	if err != nil {
		d.bodyMeasureErr = err
		return
	}
	defer body.Close()

	if request.Header.Get("Content-Encoding") != "gzip" {
		d.protobufBytes, d.bodyMeasureErr = io.Copy(io.Discard, body)
		d.gzipBytes = d.protobufBytes
		return
	}

	compressed := &countingReader{reader: body}
	gzipReader, err := gzip.NewReader(compressed)
	if err != nil {
		d.bodyMeasureErr = err
		return
	}
	d.protobufBytes, err = io.Copy(io.Discard, gzipReader)
	closeErr := gzipReader.Close()
	d.gzipBytes = compressed.bytes
	d.bodyMeasureErr = errors.Join(err, closeErr)
}

func (d *exportDiagnostics) addAttempt(attempt exportAttemptDiagnostics) {
	d.mu.Lock()
	d.attempts = append(d.attempts, attempt)
	d.mu.Unlock()
}

func (d *exportDiagnostics) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()

	var builder strings.Builder
	fmt.Fprintf(
		&builder,
		"export diagnostics: protobuf_bytes=%d gzip_bytes=%d attempts=%d",
		d.protobufBytes,
		d.gzipBytes,
		len(d.attempts),
	)
	if d.bodyMeasureErr != nil {
		fmt.Fprintf(&builder, " body_measure_error=%q", d.bodyMeasureErr)
	}
	for index, attempt := range d.attempts {
		fmt.Fprintf(
			&builder,
			" attempt_%d={status=%d reused=%t total=%s dns=%s connect=%s tls=%s write=%s wait_headers=%s}",
			index+1,
			attempt.statusCode,
			attempt.reused,
			attempt.total,
			attempt.dns,
			attempt.connect,
			attempt.tls,
			attempt.write,
			attempt.waitHeaders,
		)
	}
	return builder.String()
}

type countingReader struct {
	reader io.Reader
	bytes  int64
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	read, err := r.reader.Read(buffer)
	r.bytes += int64(read)
	return read, err
}
