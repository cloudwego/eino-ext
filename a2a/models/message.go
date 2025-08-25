package models

type Role string

const (
	RoleUser  Role = "user"
	RoleAgent      = "agent"
)

type Message struct {
	// Indicates the sender of the message:
	// "user" for messages originating from the A2A Client (acting on behalf of an end-user or system).
	// "agent" for messages originating from the A2A Server (the remote agent).
	Role Role `json:"role"`
	// An array containing the content of the message, broken down into one or more parts.
	// A message MUST contain at least one part.
	// Using multiple parts allows for rich, multi-modal content (e.g., text accompanying an image).
	Parts []Part `json:"parts"`
	// Arbitrary key-value metadata associated with the message.
	// Keys SHOULD be strings; values can be any valid JSON type.
	// Useful for timestamps, source identifiers, language codes, etc.
	Metadata map[string]any `json:"metadata,omitempty"`
	// List of tasks referenced as contextual hint by this message.
	ReferenceTaskIDs []string `json:"referenceTaskIDs,omitempty"`
	// message identifier created by the message creator
	MessageID string `json:"messageId"`
	// task identifier the current message is related to
	TaskID *string `json:"taskId,omitempty"`
	// Context identifier the message is associated with
	ContextID *string `json:"contextId,omitempty"`
}

func (m *Message) EnsureRequiredFields() {
	if m == nil {
		return
	}
	if m.Parts == nil {
		m.Parts = []Part{}
	}
	for i := 0; i < len(m.Parts); i++ {
		if m.Parts[i].Kind == PartKindData && m.Parts[i].Data == nil {
			m.Parts[i].Data = map[string]interface{}{}
		}
	}
}

type MessageSendParams struct {
	// The message to send to the agent. The `role` within this message is typically "user".
	Message Message `json:"message"`
	// Optional: additional configuration to send to the agent`.
	Configuration *MessageSendConfiguration `json:"configuration,omitempty"`
	// Arbitrary metadata for this specific `message/send` request.
	Metadata map[string]any `json:"metadata,omitempty"`
}

type MessageSendConfiguration struct {
	// AcceptedOutputModes specifies accepted output modalities by the client
	//AcceptedOutputModes []string `json:"acceptedOutputModes"` todo: why can client control server's output type...
	// HistoryLength specifies the number of recent messages to be retrieved
	//HistoryLength *int `json:"historyLength"`
	// PushNotificationConfig provides the server for sending asynchronous push notifications about task updates.
	PushNotificationConfig *PushNotificationConfig `json:"pushNotificationConfig"`
	// Blocking specifies if the server should treat the client as a blocking request
	//Blocking *bool `json:"blocking"`  todo: what is the meaning of this
}

type SendMessageResponseUnion struct {
	Message *Message
	Task    *Task
}

type SendMessageStreamingResponseUnion struct {
	Message                 *Message
	Task                    *Task
	TaskStatusUpdateEvent   *TaskStatusUpdateEvent
	TaskArtifactUpdateEvent *TaskArtifactUpdateEvent
}

func (s *SendMessageStreamingResponseUnion) GetTaskID() string {
	if s.Message != nil && s.Message.TaskID != nil {
		return *s.Message.TaskID
	} else if s.Task != nil {
		return s.Task.ID
	} else if s.TaskStatusUpdateEvent != nil {
		return s.TaskStatusUpdateEvent.TaskID
	} else if s.TaskArtifactUpdateEvent != nil {
		return s.TaskArtifactUpdateEvent.TaskID
	}
	return ""
}

type ResponseEvent struct {
	Message                        *Message
	TaskContent                    *TaskContent
	TaskStatusUpdateEventContent   *TaskStatusUpdateEventContent
	TaskArtifactUpdateEventContent *TaskArtifactUpdateEventContent
}

func (r *ResponseEvent) EnsureRequiredFields() {
	if r == nil {
		return
	}
	if r.Message != nil {
		r.Message.EnsureRequiredFields()
	}
	if r.TaskContent != nil {
		r.TaskContent.EnsureRequiredFields()
	}
	if r.TaskStatusUpdateEventContent != nil {
		r.TaskStatusUpdateEventContent.EnsureRequiredFields()
	}
	if r.TaskArtifactUpdateEventContent != nil {
		r.TaskArtifactUpdateEventContent.EnsureRequiredFields()
	}
}
