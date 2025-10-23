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

package sequentialthinking

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// Tool name and description constants
// Inspired by @modelcontextprotocol/sequentialthinking, it guides LLM through a series of questions to help them think through problems step-by-step.
const (
	toolName = "sequentialthinking"
	toolDesc = `A detailed tool for dynamic and reflective problem-solving through thoughts.
This tool helps analyze problems through a flexible thinking process that can adapt and evolve.
Each thought can build on, question, or revise previous insights as understanding deepens.

When to use this tool:
- Breaking down complex problems into steps
- Planning and design with room for revision
- Analysis that might need course correction
- Problems where the full scope might not be clear initially
- Problems that require a multi-step solution
- Tasks that need to maintain context over multiple steps
- Situations where irrelevant information needs to be filtered out

Key features:
- You can adjust total_thoughts up or down as you progress
- You can question or revise previous thoughts
- You can add more thoughts even after reaching what seemed like the end
- You can express uncertainty and explore alternative approaches
- Not every thought needs to build linearly - you can branch or backtrack
- Generates a solution hypothesis
- Verifies the hypothesis based on the Chain of Thought steps
- Repeats the process until satisfied
- Provides a correct answer

Parameters explained:
- thought: Your current thinking step, which can include:
* Regular analytical steps
* Revisions of previous thoughts
* Questions about previous decisions
* Realizations about needing more analysis
* Changes in approach
* Hypothesis generation
* Hypothesis verification
- next_thought_needed: True if you need more thinking, even if at what seemed like the end
- thought_number: Current number in sequence (can go beyond initial total if needed)
- total_thoughts: Current estimate of thoughts needed (can be adjusted up/down)
- is_revision: A boolean indicating if this thought revises previous thinking
- revises_thought: If is_revision is true, which thought number is being reconsidered
- branch_from_thought: If branching, which thought number is the branching point
- branch_id: Identifier for the current branch (if any)
- needs_more_thoughts: If reaching end but realizing more thoughts needed

You should:
1. Start with an initial estimate of needed thoughts, but be ready to adjust
2. Feel free to question or revise previous thoughts
3. Don't hesitate to add more thoughts if needed, even at the "end"
4. Express uncertainty when present
5. Mark thoughts that revise previous thinking or branch into new paths
6. Ignore information that is irrelevant to the current step
7. Generate a solution hypothesis when appropriate
8. Verify the hypothesis based on the Chain of Thought steps
9. Repeat the process until satisfied with the solution
10. Provide a single, ideally correct answer as the final output
11. Only set next_thought_needed to false when truly done and a satisfactory answer is reached
`
)

// ThoughtRequest represents a single step in the sequential thinking process.
// It captures the thought content and metadata about the thinking process.
type ThoughtRequest struct {
	Thought           string `json:"thought" jsonschema:"required" jsonschema_description:"Your current thinking step"`
	ThoughtNumber     int    `json:"thought_number" jsonschema:"required" jsonschema_description:"Current thought number"`
	TotalThoughts     int    `json:"total_thoughts" jsonschema:"required" jsonschema_description:"Estimated total thoughts needed"`
	IsRevision        bool   `json:"is_revision,omitempty" jsonschema_description:"Whether this revises previous thinking"`
	RevisesThought    int    `json:"revises_thought,omitempty" jsonschema_description:"Which thought is being reconsidered"`
	BranchFromThought int    `json:"branch_from_thought,omitempty" jsonschema_description:"Branching point thought number"`
	BranchID          string `json:"branch_id,omitempty" jsonschema_description:"Branch identifier"`
	NeedsMoreThoughts bool   `json:"needs_more_thoughts,omitempty" jsonschema_description:"If more thoughts are needed"`
	NextThoughtNeeded bool   `json:"next_thought_needed" jsonschema:"required" jsonschema_description:"Whether another thought step is needed"`
}

// ThoughtResult represents the formatted output of processing a thought.
// It contains the content to display and metadata about the thinking state.
type ThoughtResult struct {
	Content              string   `json:"content" jsonschema:"required" jsonschema_description:"Your current thinking step"`
	ThoughtNumber        int      `json:"thought_number" jsonschema:"required" jsonschema_description:"Current thought number"`
	TotalThoughts        int      `json:"total_thoughts" jsonschema:"required" jsonschema_description:"Estimated total thoughts needed"`
	NextThoughtNeeded    bool     `json:"next_thought_needed" jsonschema:"required" jsonschema_description:"Which thought is needed"`
	Branches             []string `json:"branches" jsonschema_description:"Branch identifier"`
	ThoughtHistoryLength int      `json:"thought_history_length" jsonschema_description:"Length of thoughts history needed"`
}

// thinkingServer maintains the state of the sequential thinking process.
// It stores the history of thoughts and manages branches.
type thinkingServer struct {
	thoughtHistory []*ThoughtRequest
	branches       map[string][]*ThoughtRequest
}

// newThinkingServer creates a new instance of thinkingServer with initialized fields.
// Returns: A pointer to the newly created thinkingServer
func newThinkingServer() *thinkingServer {
	return &thinkingServer{
		thoughtHistory: make([]*ThoughtRequest, 0),
		branches:       make(map[string][]*ThoughtRequest),
	}
}

// validate checks the ThoughtRequest for validity and consistency.
// Parameters:
//   - input: The thought request to validate
//
// Returns:
//   - td: The unmarshalled ThoughtRequest, or nil if an error occurred
//   - err: An error if validation fails, or nil if validation succeeds
func (t *thinkingServer) validate(req *ThoughtRequest) *ThoughtRequest {
	if req.ThoughtNumber < 1 {
		req.Thought = "Thought number must be greater than 0"
		req.IsRevision = true
		req.RevisesThought = req.ThoughtNumber
	}
	if req.TotalThoughts < 1 {
		req.Thought = "Total thoughts must be greater than 0"
		req.IsRevision = true
		req.RevisesThought = req.ThoughtNumber
	}
	if req.ThoughtNumber > req.TotalThoughts {
		req.Thought = "Thought number cannot exceed total thoughts"
		req.IsRevision = true
		req.RevisesThought = req.ThoughtNumber
	}
	if req.Thought == "" {
		req.Thought = "The Parameter's thought should not empty"
		req.IsRevision = true
		req.RevisesThought = req.ThoughtNumber
	}
	return req
}

// formatThought creates a formatted string representation of a thought.
// Parameters:
//   - td: The ThoughtRequest to format
//
// Returns: A string with the formatted thought, including decorative borders and metadata
func (t *thinkingServer) formatThought(td *ThoughtRequest) string {
	var prefix, content string
	if td.IsRevision {
		prefix = "🔄 Revision"
		content = fmt.Sprintf(" (revising thought %d)", td.RevisesThought)
	} else if td.BranchFromThought > 0 {
		prefix = "🌿 Branch"
		content = fmt.Sprintf(" (from thought %d, ID: %s)", td.BranchFromThought, td.BranchID)
	} else {
		prefix = "💭 Thought"
		content = ""
	}

	header := fmt.Sprintf("%s %d/%d%s", prefix, td.ThoughtNumber, td.TotalThoughts, content)
	border := strings.Repeat("-", max(len(header), len(td.Thought))+4)

	return fmt.Sprintf(`
┌%s┐
│ %s │
├%s┤
│ %s │
└%s┘`, border, padEnd(header, len(border)-2), border, padEnd(td.Thought, len(border)-2), border)
}

// processThought processes a thought input, validates it, adds it to the history,
// and returns a formatted result.
// Parameters:
//   - ctx: The context for the operation
//   - td: The ThoughtRequest to process
//
// Returns:
//   - result: The processed thought result
//   - err: An error if processing fails
func (t *thinkingServer) processThought(_ context.Context, td *ThoughtRequest) (*ThoughtResult, error) {
	validated := t.validate(td)
	if validated.ThoughtNumber > validated.TotalThoughts {
		validated.TotalThoughts = validated.ThoughtNumber
	}

	t.thoughtHistory = append(t.thoughtHistory, validated)

	if validated.BranchID != "" {
		t.branches[validated.BranchID] = append(t.branches[validated.BranchID], validated)
	}

	thought := t.formatThought(validated)

	return &ThoughtResult{
		Content:              thought,
		ThoughtNumber:        validated.ThoughtNumber,
		TotalThoughts:        validated.TotalThoughts,
		NextThoughtNeeded:    validated.NextThoughtNeeded,
		Branches:             getKeys(t.branches),
		ThoughtHistoryLength: len(t.thoughtHistory),
	}, nil
}

// NewTool creates a new sequential thinking tool instance.
// Returns:
//   - tool: An invokable tool interface
//   - err: An error if tool creation fails
func NewTool() (tool.InvokableTool, error) {
	thinking := newThinkingServer()
	if thinking == nil {
		return nil, errors.New("failed to create thinking server")
	}

	thinkingTool, err := utils.InferTool(toolName, toolDesc, thinking.processThought)
	if err != nil {
		return nil, fmt.Errorf("failed to infer tool: %w", err)
	}

	return thinkingTool, nil
}
