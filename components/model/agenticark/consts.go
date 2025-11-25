/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticark

const implType = "Ark"

type WebSearchAction string

const (
	WebSearchActionSearch WebSearchAction = "search"
)

type ServerToolName string

const (
	ServerToolNameWebSearch ServerToolName = "web_search"
)

type TextAnnotationType string

const (
	TextAnnotationTypeURLCitation TextAnnotationType = "url_citation"
	TextAnnotationTypeDocCitation TextAnnotationType = "doc_citation"
)

type ThinkingType string

const (
	ThinkingTypeAuto     ThinkingType = "auto"
	ThinkingTypeEnabled  ThinkingType = "enabled"
	ThinkingTypeDisabled ThinkingType = "disabled"
)

type ResponseStatus string

const (
	ResponseStatusInProgress ResponseStatus = "in_progress"
	ResponseStatusCompleted  ResponseStatus = "completed"
	ResponseStatusIncomplete ResponseStatus = "incomplete"
	ResponseStatusFailed     ResponseStatus = "failed"
)

type ServiceTier string

const (
	ServiceTierAuto    ServiceTier = "auto"
	ServiceTierDefault ServiceTier = "default"
)
