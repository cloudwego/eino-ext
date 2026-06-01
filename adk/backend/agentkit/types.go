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

package agentkit

type invokeToolRequest struct {
	ToolID           string `json:"ToolId"`
	SessionID        string `json:"SessionId"`
	UserSessionID    string `json:"UserSessionId"`
	OperationPayload string `json:"OperationPayload"`
	OperationType    string `json:"OperationType"`
	Ttl              *int   `json:"Ttl"`
}

type response struct {
	Result struct {
		Result string `json:"result"`
	} `json:"result"`
}

type result struct {
	Data struct {
		Outputs []output `json:"outputs"`
	} `json:"data"`
	Success bool `json:"success"`
}

type output struct {
	OutputType string `json:"output_type"`
	Text       string `json:"text"`
	EName      string `json:"ename"`
	EValue     string `json:"evalue"`
}
