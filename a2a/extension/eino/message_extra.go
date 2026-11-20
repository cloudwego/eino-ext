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

package eino

import (
	"github.com/cloudwego/eino/schema"
)

const (
	extraKeyOfReferenceTaskIDs = "_a2a_eino_adk_reference_task_ids"
	extraKeyOfMessageID        = "_a2a_eino_adk_message_id"
	extraKeyOfTaskID           = "_a2a_eino_adk_task_id"
	extraKeyOfContextID        = "_a2a_eino_adk_context_id"
	extraKeyOfArtifactID       = "_a2a_eino_adk_artifact_id"
)

func GetReferenceTaskIDs(msg *schema.Message) ([]string, bool) {
	if msg == nil {
		return nil, false
	}
	ids, ok := msg.Extra[extraKeyOfReferenceTaskIDs].([]string)
	return ids, ok
}

func SetReferenceTaskIDs(msg *schema.Message, ids []string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[extraKeyOfReferenceTaskIDs] = ids
}

func GetMessageID(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	id, ok := msg.Extra[extraKeyOfMessageID].(string)
	return id, ok
}

func SetMessageID(msg *schema.Message, id string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[extraKeyOfMessageID] = id
}

func GetTaskID(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	id, ok := msg.Extra[extraKeyOfTaskID].(string)
	return id, ok
}

func SetTaskID(msg *schema.Message, id string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[extraKeyOfTaskID] = id
}

func GetContextID(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	id, ok := msg.Extra[extraKeyOfContextID].(string)
	return id, ok
}

func SetContextID(msg *schema.Message, id string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[extraKeyOfContextID] = id
}

func GetArtifactID(msg *schema.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	id, ok := msg.Extra[extraKeyOfArtifactID].(string)
	return id, ok
}

func SetArtifactID(msg *schema.Message, id string) {
	if msg == nil {
		return
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]interface{})
	}
	msg.Extra[extraKeyOfArtifactID] = id
}
