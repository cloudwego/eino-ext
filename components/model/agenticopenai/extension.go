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

package agenticopenai

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/schema"
	"github.com/mitchellh/mapstructure"
)

type ServerToolCallArguments struct {
	WebSearch       *WebSearchArguments       `json:"web_search,omitempty"`
	FileSearch      *FileSearchArguments      `json:"file_search,omitempty"`
	CodeInterpreter *CodeInterpreterArguments `json:"code_interpreter,omitempty"`
	ImageGeneration *ImageGenerationArguments `json:"image_generation,omitempty"`
	Shell           *ShellArguments           `json:"shell,omitempty"`
}

type ServerToolResult struct {
	WebSearch       *WebSearchResult       `json:"web_search,omitempty"`
	FileSearch      *FileSearchResult      `json:"file_search,omitempty"`
	CodeInterpreter *CodeInterpreterResult `json:"code_interpreter,omitempty"`
	ImageGeneration *ImageGenerationResult `json:"image_generation,omitempty"`
	Shell           *ShellResult           `json:"shell,omitempty"`
}

// WebSearchArguments represents the arguments for a web search tool call.
type WebSearchArguments struct {
	// ActionType is the type of action: search, open_page, or find.
	ActionType WebSearchAction `json:"action_type,omitempty"`
	// Search is the search query parameters. Present when ActionType is "search".
	Search *WebSearchQuery `json:"search,omitempty"`
	// OpenPage is the open page parameters. Present when ActionType is "open_page".
	OpenPage *WebSearchOpenPage `json:"open_page,omitempty"`
	// Find is the find in page parameters. Present when ActionType is "find".
	Find *WebSearchFind `json:"find,omitempty"`
}

// WebSearchQuery represents a web search query parameters.
type WebSearchQuery struct {
	// Queries are the search queries.
	Queries []string `json:"queries,omitempty"`
}

// WebSearchOpenPage represents open page parameters.
type WebSearchOpenPage struct {
	// URL is the URL to open.
	URL string `json:"url,omitempty"`
}

// WebSearchFind represents find in page parameters.
type WebSearchFind struct {
	// URL is the URL of the page.
	URL string `json:"url,omitempty"`
	// Pattern is the pattern to find in the page.
	Pattern string `json:"pattern,omitempty"`
}

// FileSearchArguments represents the arguments for a file search tool call.
type FileSearchArguments struct {
	// Queries are the queries used to search for files.
	Queries []string `json:"queries,omitempty"`
}

// WebSearchQueryResult represents the result of a web search query.
type WebSearchQueryResult struct {
	// Sources are the sources returned from the search.
	Sources []*WebSearchQuerySource `json:"sources,omitempty"`
}

// WebSearchQuerySource represents a source returned from the search.
type WebSearchQuerySource struct {
	// URL is the URL of the source.
	URL string `json:"url,omitempty"`
}

// WebSearchResult represents the result of a web search tool call.
type WebSearchResult struct {
	// ActionType is the type of action performed.
	ActionType WebSearchAction `json:"action_type,omitempty"`
	// Search contains the search query result. Present when ActionType is "search".
	Search *WebSearchQueryResult `json:"search,omitempty"`
}

// FileSearchResult represents the result of a file search tool call.
type FileSearchResult struct {
	// Results contains the matched files.
	Results []*FileSearchResultItem `json:"results,omitempty"`
}

// FileSearchResultItem represents a single matched file.
type FileSearchResultItem struct {
	// FileID is the unique ID of the file.
	FileID string `json:"file_id,omitempty"`
	// FileName is the name of the file.
	FileName string `json:"file_name,omitempty"`
	// Score is the relevance score, a value between 0 and 1.
	Score float64 `json:"score,omitempty"`
	// Text is the text content retrieved from the file.
	Text string `json:"text,omitempty"`
	// Attributes contains the metadata key-value pairs attached to the file.
	Attributes map[string]*FileSearchAttribute `json:"attributes,omitempty"`
}

// FileSearchAttribute represents an attribute value, which can be string, float, or bool.
type FileSearchAttribute struct {
	// OfString is set when the attribute value is a string.
	OfString *string `json:"of_string,omitempty"`
	// OfFloat is set when the attribute value is a number.
	OfFloat *float64 `json:"of_float,omitempty"`
	// OfBool is set when the attribute value is a boolean.
	OfBool *bool `json:"of_bool,omitempty"`
}

// CodeInterpreterArguments represents the arguments for a code interpreter tool call.
type CodeInterpreterArguments struct {
}

// CodeInterpreterResult represents the result of a code interpreter tool call.
type CodeInterpreterResult struct {
	// Code is the code that was run.
	Code string `json:"code,omitempty"`
	// ContainerID is the ID of the container used to run the code.
	ContainerID string `json:"container_id,omitempty"`
	// Outputs are the outputs generated by the code interpreter.
	Outputs []*CodeInterpreterOutput `json:"outputs,omitempty"`
}

// CodeInterpreterOutput represents a single output from the code interpreter.
type CodeInterpreterOutput struct {
	// Type is the output type: "logs" or "image".
	Type CodeInterpreterOutputType `json:"type,omitempty"`
	// Logs is set when Type is "logs".
	Logs *CodeInterpreterOutputLogs `json:"logs,omitempty"`
	// Image is set when Type is "image".
	Image *CodeInterpreterOutputImage `json:"image,omitempty"`
}

// CodeInterpreterOutputLogs represents the logs output from the code interpreter.
type CodeInterpreterOutputLogs struct {
	// Logs is the log content from code execution.
	Logs string `json:"logs,omitempty"`
}

// CodeInterpreterOutputImage represents the image output from the code interpreter.
type CodeInterpreterOutputImage struct {
	// URL is the URL of the generated image.
	URL string `json:"url,omitempty"`
}

// ImageGenerationArguments represents the arguments for an image generation tool call.
type ImageGenerationArguments struct {
}

// ImageGenerationResult represents the result of an image generation tool call.
type ImageGenerationResult struct {
	// ImageBase64 is the base64-encoded image data.
	ImageBase64 string `json:"image_base64,omitempty"`
}

// ShellArguments represents the arguments for a shell tool call.
type ShellArguments struct {
	// Action contains the shell commands to execute.
	Action *ShellAction `json:"action,omitempty"`
	// Environment specifies where to run the commands.
	Environment *ShellEnvironment `json:"environment,omitempty"`
	// CreatedBy is the ID of the entity that created this tool call.
	CreatedBy string `json:"created_by,omitempty"`
}

// ShellAction contains the shell commands and execution limits.
type ShellAction struct {
	// Commands are the shell commands to run.
	Commands []string `json:"commands,omitempty"`
	// TimeoutMs is the timeout in milliseconds for the commands.
	TimeoutMs int64 `json:"timeout_ms,omitempty"`
	// MaxOutputLength is the maximum number of characters to return from each command.
	MaxOutputLength int64 `json:"max_output_length,omitempty"`
}

// ShellEnvironment specifies the execution environment for shell commands.
type ShellEnvironment struct {
	// Type is the environment type: "local" or "container_reference".
	Type ShellEnvironmentType `json:"type,omitempty"`
	// Local is set when Type is "local".
	Local *ShellEnvironmentLocal `json:"local,omitempty"`
	// ContainerReference is set when Type is "container_reference".
	ContainerReference *ShellEnvironmentContainerReference `json:"container_reference,omitempty"`
}

// ShellEnvironmentLocal represents a local execution environment for shell commands.
type ShellEnvironmentLocal struct {
	// Skills is an optional list of skills available in the local environment.
	Skills []*ShellEnvironmentLocalSkill `json:"skills,omitempty"`
}

// ShellEnvironmentLocalSkill represents a skill available in a local environment.
type ShellEnvironmentLocalSkill struct {
	// Name is the name of the skill.
	Name string `json:"name,omitempty"`
	// Description is the description of the skill.
	Description string `json:"description,omitempty"`
	// Path is the path to the directory containing the skill.
	Path string `json:"path,omitempty"`
}

// ShellEnvironmentContainerReference identifies a container to run commands in.
type ShellEnvironmentContainerReference struct {
	// ContainerID is the ID of the container.
	ContainerID string `json:"container_id,omitempty"`
}

// ShellResult represents the result of a shell tool call.
type ShellResult struct {
	// MaxOutputLength is the maximum length of output returned per command.
	MaxOutputLength int64 `json:"max_output_length,omitempty"`
	// Outputs contains the output from each command.
	Outputs []*ShellOutputItem `json:"outputs,omitempty"`
	// CreatedBy is the identifier of the actor that created this result.
	CreatedBy string `json:"created_by,omitempty"`
}

// ShellOutputItem represents the output from a single command.
type ShellOutputItem struct {
	// Stdout is the captured standard output.
	Stdout string `json:"stdout,omitempty"`
	// Stderr is the captured standard error.
	Stderr string `json:"stderr,omitempty"`
	// Outcome indicates how the command finished (exit or timeout).
	Outcome *ShellOutputOutcome `json:"outcome,omitempty"`
	// CreatedBy is the identifier of the actor that created this output.
	CreatedBy string `json:"created_by,omitempty"`
}

// ShellOutputOutcome indicates how a shell command finished.
type ShellOutputOutcome struct {
	// Type is the outcome type: "exit" or "timeout".
	Type ShellOutputOutcomeType `json:"type,omitempty"`
	// Exit is set when Type is "exit".
	Exit *ShellOutputOutcomeExit `json:"exit,omitempty"`
}

// ShellOutputOutcomeExit contains exit information for a completed command.
type ShellOutputOutcomeExit struct {
	// ExitCode is the exit code from the shell process.
	ExitCode int64 `json:"exit_code,omitempty"`
}

func getServerToolCallArguments(call *schema.ServerToolCall) (*ServerToolCallArguments, error) {
	if call == nil || call.Arguments == nil {
		return nil, fmt.Errorf("server tool call arguments are nil")
	}
	if args, ok := call.Arguments.(*ServerToolCallArguments); ok {
		return args, nil
	}
	if m, ok := call.Arguments.(map[string]any); ok {
		args := &ServerToolCallArguments{}
		if err := mapstructure.Decode(m, args); err != nil {
			return nil, fmt.Errorf("failed to decode server tool call arguments: %w", err)
		}
		return args, nil
	}
	return nil, fmt.Errorf("unexpected type %T for server tool call arguments", call.Arguments)
}

func getServerToolResult(res *schema.ServerToolResult) (*ServerToolResult, error) {
	if res == nil || res.Result == nil {
		return nil, fmt.Errorf("server tool result is nil")
	}
	if result, ok := res.Result.(*ServerToolResult); ok {
		return result, nil
	}
	if m, ok := res.Result.(map[string]any); ok {
		result := &ServerToolResult{}
		if err := mapstructure.Decode(m, result); err != nil {
			return nil, fmt.Errorf("failed to decode server tool result: %w", err)
		}
		return result, nil
	}
	return nil, fmt.Errorf("unexpected type %T for server tool result", res.Result)
}

func checkExpectedType(expectedType, chunkType reflect.Type) (reflect.Type, error) {
	if expectedType == nil {
		return chunkType, nil
	}
	if expectedType != chunkType {
		return nil, fmt.Errorf("type mismatch, expected '%s', but got '%s'", expectedType, chunkType)
	}
	return expectedType, nil
}

func concatServerToolCallArguments(chunks []*ServerToolCallArguments) (ret *ServerToolCallArguments, err error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no server tool call arguments found")
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}

	var (
		expectedType             reflect.Type
		webSearchArguments       *WebSearchArguments
		fileSearchArguments      *FileSearchArguments
		codeInterpreterArguments *CodeInterpreterArguments
		imageGenerationArguments *ImageGenerationArguments
		shellArguments           []*ShellArguments
	)
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}

		switch {
		case chunk.WebSearch != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.WebSearch))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool call arguments: %w", err)
			}
			if webSearchArguments != nil {
				return nil, fmt.Errorf("cannot concat multiple web search arguments")
			}
			webSearchArguments = chunk.WebSearch

		case chunk.FileSearch != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.FileSearch))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool call arguments: %w", err)
			}
			if fileSearchArguments != nil {
				return nil, fmt.Errorf("cannot concat multiple file search arguments")
			}
			fileSearchArguments = chunk.FileSearch

		case chunk.CodeInterpreter != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.CodeInterpreter))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool call arguments: %w", err)
			}
			if codeInterpreterArguments != nil {
				return nil, fmt.Errorf("cannot concat multiple code interpreter arguments")
			}
			codeInterpreterArguments = chunk.CodeInterpreter

		case chunk.ImageGeneration != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.ImageGeneration))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool call arguments: %w", err)
			}
			if imageGenerationArguments != nil {
				return nil, fmt.Errorf("cannot concat multiple image generation arguments")
			}
			imageGenerationArguments = chunk.ImageGeneration

		case chunk.Shell != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.Shell))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool call arguments: %w", err)
			}
			shellArguments = append(shellArguments, chunk.Shell)
		}
	}

	if webSearchArguments != nil {
		return &ServerToolCallArguments{WebSearch: webSearchArguments}, nil
	}
	if fileSearchArguments != nil {
		return &ServerToolCallArguments{FileSearch: fileSearchArguments}, nil
	}
	if codeInterpreterArguments != nil {
		return &ServerToolCallArguments{CodeInterpreter: codeInterpreterArguments}, nil
	}
	if imageGenerationArguments != nil {
		return &ServerToolCallArguments{ImageGeneration: imageGenerationArguments}, nil
	}
	if len(shellArguments) > 0 {
		return &ServerToolCallArguments{
			Shell: concatShellArguments(shellArguments),
		}, nil
	}

	return nil, fmt.Errorf("no valid server tool call arguments to concat")
}

func concatShellArguments(chunks []*ShellArguments) *ShellArguments {
	ret := &ShellArguments{}
	for _, chunk := range chunks {
		if chunk.Action != nil {
			ret.Action = chunk.Action
		}
		if chunk.Environment != nil {
			ret.Environment = chunk.Environment
		}
		if chunk.CreatedBy != "" {
			ret.CreatedBy = chunk.CreatedBy
		}
	}
	return ret
}

func concatServerToolResult(chunks []*ServerToolResult) (ret *ServerToolResult, err error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no server tool result found")
	}
	if len(chunks) == 1 {
		return chunks[0], nil
	}

	var (
		expectedType           reflect.Type
		webSearchResult        *WebSearchResult
		fileSearchResult       *FileSearchResult
		codeInterpreterResults []*CodeInterpreterResult
		imageGenerationResults []*ImageGenerationResult
		shellResults           []*ShellResult
	)
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		switch {
		case chunk.WebSearch != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.WebSearch))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool result: %w", err)
			}
			if webSearchResult != nil {
				return nil, fmt.Errorf("cannot concat multiple web search results")
			}
			webSearchResult = chunk.WebSearch

		case chunk.FileSearch != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.FileSearch))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool result: %w", err)
			}
			if fileSearchResult != nil {
				return nil, fmt.Errorf("cannot concat multiple file search results")
			}
			fileSearchResult = chunk.FileSearch

		case chunk.CodeInterpreter != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.CodeInterpreter))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool result: %w", err)
			}
			codeInterpreterResults = append(codeInterpreterResults, chunk.CodeInterpreter)

		case chunk.ImageGeneration != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.ImageGeneration))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool result: %w", err)
			}
			imageGenerationResults = append(imageGenerationResults, chunk.ImageGeneration)

		case chunk.Shell != nil:
			expectedType, err = checkExpectedType(expectedType, reflect.TypeOf(chunk.Shell))
			if err != nil {
				return nil, fmt.Errorf("failed to concat server tool result: %w", err)
			}
			shellResults = append(shellResults, chunk.Shell)
		}
	}

	if webSearchResult != nil {
		return &ServerToolResult{WebSearch: webSearchResult}, nil
	}
	if fileSearchResult != nil {
		return &ServerToolResult{FileSearch: fileSearchResult}, nil
	}
	if len(codeInterpreterResults) > 0 {
		return &ServerToolResult{
			CodeInterpreter: concatCodeInterpreterResults(codeInterpreterResults),
		}, nil
	}
	if len(imageGenerationResults) > 0 {
		return &ServerToolResult{
			ImageGeneration: concatImageGenerationResults(imageGenerationResults),
		}, nil
	}
	if len(shellResults) > 0 {
		return &ServerToolResult{
			Shell: concatShellResults(shellResults),
		}, nil
	}

	return nil, fmt.Errorf("no valid server tool result to concat")
}

func concatCodeInterpreterResults(chunks []*CodeInterpreterResult) *CodeInterpreterResult {
	ret := &CodeInterpreterResult{}
	for _, chunk := range chunks {
		ret.Code += chunk.Code
		if chunk.ContainerID != "" {
			ret.ContainerID = chunk.ContainerID
		}
		ret.Outputs = append(ret.Outputs, chunk.Outputs...)
	}
	return ret
}

func concatImageGenerationResults(chunks []*ImageGenerationResult) *ImageGenerationResult {
	ret := &ImageGenerationResult{}
	for _, chunk := range chunks {
		ret.ImageBase64 += chunk.ImageBase64
	}
	return ret
}

func concatShellResults(chunks []*ShellResult) *ShellResult {
	ret := &ShellResult{}
	for _, chunk := range chunks {
		if chunk.MaxOutputLength > 0 {
			ret.MaxOutputLength = chunk.MaxOutputLength
		}
		if chunk.CreatedBy != "" {
			ret.CreatedBy = chunk.CreatedBy
		}
		ret.Outputs = append(ret.Outputs, chunk.Outputs...)
	}
	return ret
}
