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

package einodev

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/compose"

	"github.com/cloudwego/eino-ext/devops/internal/model"
	"github.com/cloudwego/eino-ext/devops/internal/service"
	"github.com/cloudwego/eino-ext/devops/internal/utils/log"
)

const (
	einodevBuildIdentify = "einodev/internal/model.GraphInfo.BuildDevGraph"
)

type globalDevGraphCompileCallback struct {
	onFinish func(ctx context.Context, graphInfo *compose.GraphInfo)
}

func newGlobalDevGraphCompileCallback(opts ...DevOption) compose.GraphCompileCallback {
	onFinish := func(ctx context.Context, graphInfo *compose.GraphInfo) {
		if graphInfo == nil || strings.Contains(graphInfo.Key, einodevBuildIdentify) {
			return
		}

		opt := model.GraphOption{
			GenState: graphInfo.GenStateFn,
		}

		graphName := extractLastSegment(graphInfo.Key)

		_, err := service.ContainerSVC.AddGlobalGraphInfo(graphName, graphInfo, opt)
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

func extractLastSegment(str string) string {
	lastSlashIndex := strings.LastIndex(str, "/")

	if lastSlashIndex != -1 {
		return str[lastSlashIndex+1:]
	}
	return str
}
