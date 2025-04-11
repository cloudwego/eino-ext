/*
 * Copyright 2025 CloudWeGo Authors
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

package cozeloop

import (
	"runtime/debug"

	"github.com/cloudwego/eino-ext/callbacks/cozeloop/internal/consts"
)

func readBuildVersion() string {
	if v, ok := readVersionByGoMod(consts.EinoImportPath); ok {
		return v
	}

	return "unknown_build_info"
}

func readVersionByGoMod(path string) (string, bool) {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}

	for _, dep := range buildInfo.Deps {
		if dep.Path == path {
			if dep.Replace != nil {
				return dep.Replace.Version, true
			} else {
				return dep.Version, true
			}
		}
	}

	return "", false
}
