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

package openaigo

import "github.com/openai/openai-go/v3/shared"

type ReasoningEffort string

const (
	ReasoningEffortNone    ReasoningEffort = "none"
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
	ReasoningEffortXHigh   ReasoningEffort = "xhigh"
)

type ReasoningSummary string

const (
	ReasoningSummaryAuto     ReasoningSummary = "auto"
	ReasoningSummaryConcise  ReasoningSummary = "concise"
	ReasoningSummaryDetailed ReasoningSummary = "detailed"
)

// Reasoning config for Responses API reasoning models.
// Maps to openai-go's shared.ReasoningParam.
type Reasoning struct {
	Effort  ReasoningEffort  `json:"effort,omitempty"`
	Summary ReasoningSummary `json:"summary,omitempty"`
}

func (r *Reasoning) toSDK() shared.ReasoningParam {
	if r == nil {
		return shared.ReasoningParam{}
	}
	return shared.ReasoningParam{
		Effort:  shared.ReasoningEffort(r.Effort),
		Summary: shared.ReasoningSummary(r.Summary),
	}
}
