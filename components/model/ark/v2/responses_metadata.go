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

package ark

const (
	keyResponsesItemIndex    = "ark-responses-item-index"
	keyResponsesSection      = "ark-responses-section"
	keyResponsesContentIndex = "ark-responses-content-index"
	keyResponsesSummaryIndex = "ark-responses-summary-index"
	keyResponsesItemType     = "ark-responses-item-type"
	keyResponsesContentType  = "ark-responses-content-type"
	keyResponsesItemID       = "ark-responses-item-id"
	keyResponsesItemStatus   = "ark-responses-item-status"
	keyResponsesItemRole     = "ark-responses-item-role"
	keyResponsesPartial      = "ark-responses-partial"
	keyResponsesNamespace    = "ark-responses-namespace"
	keyResponsesAnnotations  = "ark-responses-annotations"
	// SDK values without an Eino data mapping remain observable as atomic JSON.
	// They are never decoded into additional request fields.
	keyResponsesItemData    = "ark-responses-item-data"
	keyResponsesContentData = "ark-responses-content-data"
)
