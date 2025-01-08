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

package log

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type logger struct {
	mu sync.Mutex
	*log.Logger
	logCh chan Message
}

type Message struct {
	Level string
	Msg   string
}

const (
	infoLevel = 1
	warnLevel = 2
	errLevel  = 3
)

var (
	lg     *logger
	lgOnce sync.Once
)

func InitLogger() (logCh chan Message) {
	lgOnce.Do(func() {
		lg = &logger{
			Logger: log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile),
			mu:     sync.Mutex{},
			logCh:  make(chan Message, 1000),
		}
	})
	return lg.logCh
}

func Infof(format string, v ...any) {
	printLog(infoLevel, 3, format, v...)
	sendLogMsg(infoLevel, fmt.Sprintf(format, v...))
}

func Warnf(format string, v ...any) {
	printLog(warnLevel, 3, format, v...)
	sendLogMsg(warnLevel, fmt.Sprintf(format, v...))
}

func Errorf(format string, v ...any) {
	printLog(errLevel, 3, format, v...)
	sendLogMsg(errLevel, fmt.Sprintf(format, v...))
}

func printLog(level int, depth int, format string, v ...any) {
	if lg == nil {
		return
	}

	lg.mu.Lock()
	defer lg.mu.Unlock()

	switch level {
	case infoLevel:
		lg.SetOutput(os.Stdout)
		lg.SetPrefix("[eino devops][INFO] ")
	case warnLevel:
		lg.SetOutput(os.Stderr)
		lg.SetPrefix("[eino devops][WARN] ")
	case errLevel:
		lg.SetOutput(os.Stderr)
		lg.SetPrefix("[eino devops][ERROR] ")
	default:
		lg.SetOutput(os.Stdout)
		lg.SetPrefix("[eino devops][INFO] ")
	}

	lg.Output(depth, fmt.Sprintf(format, v...))
}

func sendLogMsg(level int, msg string) {
	if lg == nil {
		return
	}

	logMsg := fmt.Sprintf("%s %s", time.Now().Format("2006/01/02 15:04:05.999"), msg)
	var message Message
	switch level {
	case infoLevel:
		message = Message{Msg: fmt.Sprintf("[INFO] %s", logMsg), Level: "INFO"}
	case warnLevel:
		message = Message{Msg: fmt.Sprintf("[WARN] %s", logMsg), Level: "WARN"}
	case errLevel:
		message = Message{Msg: fmt.Sprintf("[ERROR] %s", logMsg), Level: "ERROR"}
	default:
		message = Message{Msg: fmt.Sprintf("[INFO] %s", logMsg), Level: "INFO"}

	}

	select {
	case lg.logCh <- message:
	default:
		printLog(warnLevel, 4, "too many log, will drop\nlog=%s", logMsg)
	}
}
