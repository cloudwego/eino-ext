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

package models

type PushNotificationConfig struct {
	// The absolute HTTPS webhook URL where the A2A Server should POST task updates.
	// This URL MUST be HTTPS for security.
	URL string `json:"url"`
	// An optional, client-generated opaque token (e.g., a secret, a task-specific identifier, or a nonce).
	// The A2A Server SHOULD include this token in the notification request it sends to the `url`
	// (e.g., in a custom HTTP header like `X-A2A-Notification-Token` or similar).
	// This allows the client's webhook receiver to validate the relevance and authenticity of the notification.
	Token string `json:"token,omitempty"`
	// Authentication details the A2A Server needs to use when calling the client's `url`.
	// The client's webhook endpoint defines these requirements. This tells the A2A Server how to authenticate *itself* to the client's webhook.
	Authentication *AuthenticationInfo `json:"authentication,omitempty"`
}

type GetTaskPushNotificationConfigParams struct {
	PushNotificationConfigID string `json:"pushNotificationConfigID,omitempty"`
}
