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

package service

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/cloudwego/eino-ext/devops/internal/utils/log"
	"github.com/cloudwego/eino/compose"
)

const (
	einoIdentify            = "github.com/cloudwego/eino/"
	devIdentify             = "devops/internal/model.(*Graph).Compile"
	graphCompileCallerDepth = 5
)

type globalDevGraphCompileCallback struct {
	onFinish func(ctx context.Context, graphInfo *compose.GraphInfo)
}

func NewGlobalDevGraphCompileCallback() compose.GraphCompileCallback {
	onFinish := func(ctx context.Context, graphInfo *compose.GraphInfo) {
		if graphInfo == nil {
			return
		}

		frame := getCompileFrame(graphCompileCallerDepth)
		if strings.Contains(frame.Function, devIdentify) {
			return
		}

		graphName := graphInfo.Name
		if graphName == "" {
			graphName = genGraphName(frame)
		}

		_, err := ContainerSVC.AddGraphInfo(graphName, graphInfo)
		if err != nil {
			log.Errorf(err.Error())
		}
	}

	return &globalDevGraphCompileCallback{
		onFinish: onFinish,
	}
}

func (d globalDevGraphCompileCallback) OnFinish(ctx context.Context, graphInfo *compose.GraphInfo) {
	d.onFinish(ctx, graphInfo)
}

func getCompileFrame(startSkip int) runtime.Frame {
	maxStep := 15
	var frame runtime.Frame
	for i := 0; i < maxStep; i++ {
		skip := startSkip + i

		pcs := make([]uintptr, 1)
		_ = runtime.Callers(skip, pcs)
		frame, _ = runtime.CallersFrames(pcs).Next()

		if !strings.Contains(frame.Function, einoIdentify) {
			break
		}
	}

	return frame
}

func genGraphName(frame runtime.Frame) string {
	file := strings.TrimSuffix(frame.File, ".go")
	lastSlashIdx := strings.LastIndex(file, "/")
	if lastSlashIdx != -1 {
		file = file[lastSlashIdx+1:]
	}

	fun := frame.Function
	lastDotIdx := strings.LastIndex(fun, ".")
	if lastDotIdx != -1 {
		fun = fun[lastDotIdx+1:]
	}

	// process pattern functions
	if strings.Contains(frame.Function, "[") && strings.Contains(fun, "]") {
		re := regexp.MustCompile(`\.([^.]+)\[`)
		matches := re.FindStringSubmatch(frame.Function)
		if len(matches) > 1 {
			fun = matches[1]
		}
	}

	return fmt.Sprintf("%s.%s:%d", file, fun, frame.Line)
}
