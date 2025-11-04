package openai

import (
	"github.com/openai/openai-go/v2/responses"
)

type BuiltInToolCallOutput struct {
	WebSearch       *responses.ResponseFunctionWebSearch             `json:"web_search,omitempty"`
	FileSearch      *responses.ResponseFileSearchToolCall            `json:"file_search,omitempty"`
	Computer        *responses.ResponseComputerToolCall              `json:"computer,omitempty"`
	ImageGeneration *responses.ResponseOutputItemImageGenerationCall `json:"image_generation,omitempty"`
	CodeInterpreter *responses.ResponseCodeInterpreterToolCall       `json:"code_interpreter,omitempty"`
	LocalShell      *responses.ResponseOutputItemLocalShellCall      `json:"local_shell,omitempty"`
}

type OutputTextAnnotation struct {
	Items []*OutputTextAnnotationItem `json:"items,omitempty"`
}

type OutputTextAnnotationItem struct {
	FileCitation          *responses.ResponseOutputTextAnnotationFileCitation          `json:"file_citation,omitempty"`
	URLCitation           *responses.ResponseOutputTextAnnotationURLCitation           `json:"url_citation,omitempty"`
	ContainerFileCitation *responses.ResponseOutputTextAnnotationContainerFileCitation `json:"container_file_citation,omitempty"`
	FilePath              *responses.ResponseOutputTextAnnotationFilePath              `json:"file_path,omitempty"`
}

type NonStandardContentBlock struct {
	MCPListTools        *responses.ResponseItemMcpListTools        `json:"mcp_list_tools"`
	MCPApprovalRequest  *responses.ResponseItemMcpApprovalRequest  `json:"mcp_approval_request"`
	MCPApprovalResponse *responses.ResponseItemMcpApprovalResponse `json:"mcp_approval_response"`
}
