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

package langfuse

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cenkalti/backoff/v4"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultFlushAt       int = 15
	defaultFlushInterval     = time.Millisecond * 500
	defaultMaxRetry          = 3

	maxEventSizeBytes = 1_000_000
	maxBatchSizeBytes = 2_500_000

	ingestionPath    = "/api/public/ingestion"
	getUploadURLPath = "/api/public/media"
	patchMediaPath   = "/api/public/media/%s"

	mediaString = "@@@langfuseMedia:type=%s|id=%s|source=%s@@@"
)

func newIngestionConsumer(
	cli *client,
	q *queue,
	flushAt int,
	flushInterval time.Duration,
	sampleRate float64,
	logMessage string,
	maskFunc func(string) string,
	sdkName string,
	sdkVersion string,
	sdkIntegration string,
	publicKey string,
	maxRetry uint64,
	mediaWG *sync.WaitGroup,
) *ingestionConsumer {
	if flushAt <= 0 {
		flushAt = defaultFlushAt
	}
	if flushInterval <= 0 {
		flushInterval = defaultFlushInterval
	}
	if maxRetry <= 0 {
		maxRetry = defaultMaxRetry
	}

	return &ingestionConsumer{
		cli:           cli,
		eventQueue:    q,
		flushAt:       flushAt,
		flushInterval: flushInterval,
		sampleRate:    sampleRate,
		logMessage:    logMessage,
		maskFunc:      maskFunc,
		mediaWG:       mediaWG,

		sdkName:        sdkName,
		sdkVersion:     sdkVersion,
		sdkIntegration: sdkIntegration,
		publicKey:      publicKey,

		closed:   atomic.Bool{},
		maxRetry: maxRetry,
	}
}

type ingestionConsumer struct {
	cli           *client
	eventQueue    *queue
	flushAt       int
	flushInterval time.Duration
	sampleRate    float64
	logMessage    string
	maskFunc      func(string) string
	mediaWG       *sync.WaitGroup
	// batch metadata
	sdkName        string
	sdkVersion     string
	sdkIntegration string
	publicKey      string

	closed   atomic.Bool
	maxRetry uint64
}

func (i *ingestionConsumer) run() {
	go func() {
		defer func() {
			e := recover()
			if e != nil {
				log.Printf("ingest consumer panic: %v", e)
			}
		}()
		for !i.closed.Load() {
			batch := i.next()
			if len(batch) == 0 {
				continue
			}

			err := i.upload(batch)
			if err != nil {
				log.Printf("ingest consumer upload error: %v", err)
			}

			for range batch {
				i.eventQueue.done()
			}
		}
	}()
}

func (i *ingestionConsumer) next() []*event {
	var events []*event
	startTime := time.Now()
	totalSize := 0
	for len(events) < i.flushAt {
		elapsed := time.Since(startTime)
		if elapsed >= i.flushInterval {
			break
		}
		ev, ok := i.eventQueue.get(i.flushInterval - elapsed)
		if !ok {
			break
		}

		// sample
		if !i.deterministicSample(ev.Body.getTraceID()) {
			i.eventQueue.done()
			continue
		}

		// handle multi-modal data
		if ev.Body.Generation != nil {
			var err error
			if len(ev.Body.Generation.InMessages) > 0 {
				_, nMessages := i.convMedias(ev.Body.Generation.InMessages, ev.Body.getTraceID(), ev.Body.getObservationID(), fieldTypeInput)
				ev.Body.Generation.Input, err = marshalMessages(nMessages)
				if err != nil {
					i.eventQueue.done()
					log.Printf("ingest consumer error, marshal model input fail: %v", err)
					continue
				}
			}
			if ev.Body.Generation.OutMessage != nil {
				_, nMessages := i.convMedias([]*schema.Message{ev.Body.Generation.OutMessage}, ev.Body.getTraceID(), ev.Body.getObservationID(), fieldTypeOutput)
				ev.Body.Generation.Output, err = marshalMessage(nMessages[0])
				if err != nil {
					i.eventQueue.done()
					log.Printf("ingest consumer error, marshal model output fail: %v", err)
					continue
				}
			}
		}

		if i.maskFunc != nil {
			if len(ev.Body.getOutput()) > 0 {
				ev.Body.setOutput(i.maskFunc(ev.Body.getOutput()))
			}
			if len(ev.Body.getInput()) > 0 {
				ev.Body.setInput(i.maskFunc(ev.Body.getInput()))
			}
		}

		size := i.truncate(ev)

		// check for serialization errors
		_, err := sonic.MarshalString(ev)
		if err != nil {
			log.Printf("marshal event error: %v, skipping: %s", err, ev.Type)
			i.eventQueue.done()
			continue
		}

		totalSize += size
		events = append(events, ev)
		if totalSize >= maxBatchSizeBytes {
			break
		}
	}

	return events
}

func (i *ingestionConsumer) deterministicSample(traceID string) bool {
	if i.sampleRate <= 0 || i.sampleRate >= 1 || len(traceID) == 0 {
		return true
	}
	hasher := sha256.New()
	hasher.Write([]byte(traceID))
	hashString := hex.EncodeToString(hasher.Sum(nil))
	hashInt, err := strconv.ParseInt(hashString[:8], 16, 64)
	if err != nil {
		log.Printf("Failed to convert trace ID hash[%s] to integer: %v", hashString[:8], err)
		return true
	}
	normalized := float64(hashInt) / float64(0xFFFFFFFF)
	return normalized < i.sampleRate
}

func (i *ingestionConsumer) truncate(ev *event) int {
	type lenAndClear struct {
		len   int
		clear func()
	}

	metadataLen := 0
	metadata, err := sonic.MarshalString(ev.Body.getMetadata())
	if err != nil {
		log.Printf("failed to marshal metadata: %v", err)
	} else {
		metadataLen = len(metadata)
	}

	sumSize := metadataLen + len(ev.Body.getInput()) + len(ev.Body.getOutput())
	if sumSize <= maxEventSizeBytes {
		return sumSize
	}

	clearList := make([]*lenAndClear, 0, 3)
	clearList = append(clearList, &lenAndClear{
		len:   len(ev.Body.getInput()),
		clear: func() { ev.Body.setInput(i.logMessage) },
	})
	clearList = append(clearList, &lenAndClear{
		len:   len(ev.Body.getOutput()),
		clear: func() { ev.Body.setOutput(i.logMessage) },
	})
	clearList = append(clearList, &lenAndClear{
		len:   metadataLen,
		clear: func() { ev.Body.setMetadata(i.logMessage) },
	})

	sort.Slice(clearList, func(i, j int) bool { return clearList[i].len > clearList[j].len })
	for _, c := range clearList {
		if c.len == 0 {
			break
		}
		c.clear()
		sumSize -= c.len
		if sumSize <= maxEventSizeBytes {
			break
		}
	}
	return sumSize
}

func (i *ingestionConsumer) upload(batch []*event) error {
	err := i.langfuseBackOffRequest(func() error {
		return i.cli.batchIngestion(batch, map[string]string{
			"batch_size":      strconv.Itoa(len(batch)),
			"sdk_integration": i.sdkName,
			"sdk_name":        i.sdkVersion,
			"sdk_version":     i.sdkIntegration,
			"public_key":      i.publicKey,
		})
	})
	if err != nil {
		return fmt.Errorf("upload event error: %v", err)
	}
	return nil
}

func (i *ingestionConsumer) langfuseMediaBackOffRequest(fn func() error) error {
	return backoff.Retry(func() error {
		err := fn()
		if err != nil {
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(backoff.WithMultiplier(2), backoff.WithInitialInterval(time.Second)), i.maxRetry))
}

func (i *ingestionConsumer) langfuseBackOffRequest(fn func() error) error {
	return backoff.Retry(func() error {
		err := fn()
		if err != nil {
			apiErr := &apiError{}
			if errors.As(err, &apiErr) {
				if apiErr.Status < 500 && apiErr.Status >= 400 && apiErr.Status != 429 {
					return nil
				}
			}
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(backoff.WithMultiplier(2), backoff.WithInitialInterval(time.Second)), i.maxRetry))
}

func (i *ingestionConsumer) convMedias(messages []*schema.Message, traceID, observationID string, field fieldType) ([]*media, []*schema.Message) {
	var medias []*media
	nMessages := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		nMessage := *message
		nMessages = append(nMessages, &nMessage)
		mc := make([]schema.ChatMessagePart, len(nMessage.MultiContent))
		copy(mc, nMessage.MultiContent)
		nMessage.MultiContent = mc
		for j := range nMessage.MultiContent {
			if nMessage.MultiContent[j].Type == schema.ChatMessagePartTypeImageURL &&
				nMessage.MultiContent[j].ImageURL != nil {
				m, err := i.tryProcessMediaFromBase64(nMessage.MultiContent[j].ImageURL.URL, traceID, observationID, field)
				if err != nil {
					log.Printf("failed to process media from image: %v", err)
					continue
				}
				if m != nil {
					nMessage.MultiContent[j].ImageURL = &schema.ChatMessageImageURL{
						URL:      fmt.Sprintf(mediaString, m.contentType, m.mediaID, m.source),
						URI:      nMessage.MultiContent[j].ImageURL.URI,
						Detail:   nMessage.MultiContent[j].ImageURL.Detail,
						MIMEType: nMessage.MultiContent[j].ImageURL.MIMEType,
						Extra:    nMessage.MultiContent[j].ImageURL.Extra,
					}

					medias = append(medias, m)
				}
			} else if nMessage.MultiContent[j].Type == schema.ChatMessagePartTypeAudioURL &&
				nMessage.MultiContent[j].AudioURL != nil {
				m, err := i.tryProcessMediaFromBase64(nMessage.MultiContent[j].AudioURL.URL, traceID, observationID, field)
				if err != nil {
					log.Printf("failed to process media from audio: %v", err)
					continue
				}
				if m != nil {
					nMessage.MultiContent[j].AudioURL = &schema.ChatMessageAudioURL{
						URL:      fmt.Sprintf(mediaString, m.contentType, m.mediaID, m.source),
						URI:      nMessage.MultiContent[j].AudioURL.URI,
						MIMEType: nMessage.MultiContent[j].AudioURL.MIMEType,
						Extra:    nMessage.MultiContent[j].AudioURL.Extra,
					}
					medias = append(medias, m)
				}
			} else if nMessage.MultiContent[j].Type == schema.ChatMessagePartTypeVideoURL &&
				nMessage.MultiContent[j].VideoURL != nil {
				m, err := i.tryProcessMediaFromBase64(nMessage.MultiContent[j].VideoURL.URL, traceID, observationID, field)
				if err != nil {
					log.Printf("failed to process media from video: %v", err)
					continue
				}

				if m != nil {
					nMessage.MultiContent[j].VideoURL = &schema.ChatMessageVideoURL{
						URL:      fmt.Sprintf(mediaString, m.contentType, m.mediaID, m.source),
						URI:      nMessage.MultiContent[j].VideoURL.URI,
						MIMEType: nMessage.MultiContent[j].VideoURL.MIMEType,
						Extra:    nMessage.MultiContent[j].VideoURL.Extra,
					}

					medias = append(medias, m)
				}
			}
		}
	}
	return medias, nMessages
}

func (i *ingestionConsumer) tryProcessMediaFromBase64(data string, traceID, observationID string, field fieldType) (*media, error) {
	m := tryNewMediaFromBase64(data)
	if m == nil {
		return nil, nil
	}
	var mediaID string
	var uploadURL string
	err := i.langfuseMediaBackOffRequest(func() error {
		var err error
		mediaID, uploadURL, err = i.cli.getUploadURL(m, traceID, observationID, field)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("process media[%s] error: %w", mediaID, err)
	}
	m.mediaID = mediaID
	if len(uploadURL) <= 0 {
		return m, nil
	}
	i.mediaWG.Add(1)
	go func() {
		defer func() {
			e := recover()
			i.mediaWG.Done()
			if e != nil {
				log.Printf("process media[%s] upload panic: %v", mediaID, e)
			}
		}()
		uploadStartTime := time.Now()
		var code int
		var message string
		err_ := i.langfuseMediaBackOffRequest(func() error {
			var backOffErr error
			code, message, backOffErr = i.cli.uploadMedia(m, uploadURL)
			if backOffErr != nil {
				return backOffErr
			}
			return nil
		})
		if err_ != nil {
			log.Printf("process media[%s] upload error: %v", mediaID, err_)
			return
		}
		err_ = i.langfuseMediaBackOffRequest(func() error {
			return i.cli.patchMedia(mediaID, time.Now(), code, message, time.Since(uploadStartTime).Milliseconds())
		})
		if err_ != nil {
			log.Printf("process media[%s] patch error: %v", mediaID, err_)
		}
	}()
	return m, nil
}
