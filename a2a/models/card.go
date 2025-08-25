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

import (
	"github.com/go-openapi/spec"
)

type AgentProvider struct {
	// Name of the organization or entity.
	Organization string `json:"organization"`
	// URL for the provider's organization website or relevant contact page.
	URL string `json:"url"`
}

type AgentCapabilities struct {
	// If `true`, the agent supports `message/stream` and `tasks/resubscribe` for real-time
	// updates via Server-Sent Events (SSE). Default: `false`.
	Streaming bool `json:"streaming,omitempty"`
	// If `true`, the agent supports `tasks/pushNotificationConfig/set` and `tasks/pushNotificationConfig/get`
	// for asynchronous task updates via webhooks. Default: `false`.
	PushNotifications bool `json:"pushNotifications,omitempty"`
	// If `true`, the agent may include a detailed history of status changes
	// within the `Task` object (future enhancement; specific mechanism TBD). Default: `false`.
	StateTransitionHistory bool `json:"stateTransitionHistory,omitempty"`
}

// AgentSkill defines a specific skill or capability offered by an agent
type AgentSkill struct {
	// ID is the unique identifier for the skill
	ID string `json:"id"`
	// Name is the human-readable name of the skill
	Name string `json:"name"`
	// Description is an optional description of the skill
	Description *string `json:"description"`
	// Tags is an optional list of tags associated with the skill for categorization
	Tags []string `json:"tags"`
	// Examples is an optional list of example inputs or use cases for the skill
	Examples []string `json:"examples,omitempty"`
	// InputModes is an optional list of input modes supported by this skill
	InputModes []string `json:"inputModes,omitempty"`
	// OutputModes is an optional list of output modes supported by this skill
	OutputModes []string `json:"outputModes,omitempty"`
}

// AgentCard conveys key information about an A2A Server:
// - Overall identity and descriptive details.
// - Service endpoint URL.
// - Supported A2A protocol capabilities (streaming, push notifications).
// - Authentication requirements.
// - Default input/output content types (MIME types).
// - A list of specific skills the agent offers.
type AgentCard struct {
	/**
	 * The version of the A2A protocol this agent supports.
	 * @default "0.2.5"
	 */
	ProtocolVersion string `json:"protocolVersion"`
	/**
	 * Human readable name of the agent.
	 * Example: "Recipe Agent"
	 */
	Name string `json:"name"`
	/**
	 * A human-readable description of the agent. Used to assist users and
	 * other agents in understanding what the agent can do.
	 * Example: "Agent that helps users with recipes and cooking."
	 */
	Description string `json:"description"`
	/**
	 * A URL to the address the agent is hosted at. This represents the
	 * preferred endpoint as declared by the agent.
	 */
	URL string `json:"url"`
	/**
	 * The transport of the preferred endpoint. If empty, defaults to JSONRPC.
	 */
	PreferredTransport string `json:"preferredTransport,omitempty"`
	/**
	 * Announcement of additional supported transports. Client can use any of
	 * the supported transports.
	 */
	// AdditionalInterfaces AgentInterface[] `json:"additionalInterfaces,omitempty"` todo: support?
	/** A URL to an icon for the agent. */
	IconUrl string `json:"iconUrl,omitempty"`
	/** The service provider of the agent */
	Provider *AgentProvider `json:"provider,omitempty"`
	/**
	 * The version of the agent - format is up to the provider.
	 * @TJS-examples ["1.0.0"]
	 */
	Version string `json:"version"`
	/**
	 * A URL to the address the agent is hosted at. This represents the
	 * preferred endpoint as declared by the agent.
	 */
	DocumentationURL string `json:"documentationUrl,omitempty"`
	/** Optional capabilities supported by the agent. */
	Capabilities AgentCapabilities `json:"capabilities"`
	/** Security scheme details used for authenticating with this agent. */
	SecuritySchemes map[string]*spec.SecurityScheme `json:"securitySchemes,omitempty"`
	/** Security requirements for contacting the agent. */
	Security map[string][]string `json:"security,omitempty"`
	/**
	 * The set of interaction modes that the agent supports across all skills. This can be overridden per-skill.
	 * Supported media types for input.
	 */
	DefaultInputModes []string `json:"defaultInputModes"`
	/** Supported media types for output. */
	DefaultOutputModes []string `json:"defaultOutputModes"`
	/** Skills are a unit of capability that an agent can perform. */
	Skills []AgentSkill `json:"skills"`
	/**
	 * true if the agent supports providing an extended agent card when the user is authenticated.
	 * Defaults to false if not specified.
	 */
	SupportsAuthenticatedExtendedCard bool `json:"supportsAuthenticatedExtendedCard,omitempty"`
}
