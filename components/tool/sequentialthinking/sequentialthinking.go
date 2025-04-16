package sequentialthinking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	
	"github.com/bytedance/sonic"
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

// thoughtData represents a single step in the sequential thinking process.
// It captures the thought content and metadata about the thinking process.
type thoughtData struct {
	Thought           string `json:"thought" jsonschema:"required,description=Your current thinking step"`
	ThoughtNumber     int    `json:"thought_number" jsonschema:"required,description=Current thought number"`
	TotalThoughts     int    `json:"total_thoughts" jsonschema:"required,description=Estimated total thoughts needed"`
	IsRevision        bool   `json:"is_revision,omitempty" jsonschema:"description=Whether this revises previous thinking"`
	RevisesThought    int    `json:"revises_thought,omitempty" jsonschema:"description=Which thought is being reconsidered"`
	BranchFromThought int    `json:"branch_from_thought,omitempty" jsonschema:"description=Branching point thought number"`
	BranchID          string `json:"branch_id,omitempty" jsonschema:"description=Branch identifier"`
	NeedsMoreThoughts bool   `json:"needs_more_thoughts,omitempty" jsonschema:"description=If more thoughts are needed"`
	NextThoughtNeeded bool   `json:"next_thought_needed" jsonschema:"required,description=Whether another thought step is needed"`
}

// thoughtResult represents the formatted output of processing a thought.
// It contains the content to display and metadata about the thinking state.
type thoughtResult struct {
	Content              string   `json:"content" jsonschema:"required,description=Your current thinking step"`
	ThoughtNumber        int      `json:"thought_number" jsonschema:"required,description=Current thought number"`
	TotalThoughts        int      `json:"total_thoughts" jsonschema:"required,description=Estimated total thoughts needed"`
	NextThoughtNeeded    bool     `json:"next_thought_needed" jsonschema:"required,description=Which thought is needed"`
	Branches             []string `json:"branches" jsonschema:"description=Branch identifier"`
	ThoughtHistoryLength int      `json:"thought_history_length" jsonschema:"description=Length of thoughts history needed"`
}

// thinkingServer maintains the state of the sequential thinking process.
// It stores the history of thoughts and manages branches.
type thinkingServer struct {
	thoughtHistory []*thoughtData
	branches       map[string][]*thoughtData
}

// newThinkingServer creates a new instance of thinkingServer with initialized fields.
// Returns: A pointer to the newly created thinkingServer
func newThinkingServer() *thinkingServer {
	return &thinkingServer{
		thoughtHistory: make([]*thoughtData, 0),
		branches:       make(map[string][]*thoughtData),
	}
}

// validate checks if the provided JSON input can be unmarshaled into a thoughtData struct
// and performs basic validation on the fields.
// Parameters:
//   - input: The JSON string to validate
// Returns:
//   - td: The unmarshalled thoughtData, or nil if an error occurred
//   - err: An error if validation fails, or nil if validation succeeds
func (t *thinkingServer) validate(input string) (td *thoughtData, err error) {
	if err = sonic.Unmarshal([]byte(input), &td); err != nil {
		return
	}
	if td.ThoughtNumber < 1 {
		td.Thought = "Thought number must be greater than 0"
		td.IsRevision = true
		td.RevisesThought = td.ThoughtNumber
	}
	if td.TotalThoughts < 1 {
		td.Thought = "Total thoughts must be greater than 0"
		td.IsRevision = true
		td.RevisesThought = td.ThoughtNumber
	}
	if td.ThoughtNumber > td.TotalThoughts {
		td.Thought = "Thought number cannot exceed total thoughts"
		td.IsRevision = true
		td.RevisesThought = td.ThoughtNumber
	}
	if td.Thought == "" {
		td.Thought = "The Parameter's thought should not empty"
		td.IsRevision = true
		td.RevisesThought = td.ThoughtNumber
	}
	return
}

// formatThought creates a formatted string representation of a thought.
// Parameters:
//   - td: The thoughtData to format
// Returns: A string with the formatted thought, including decorative borders and metadata
func (t *thinkingServer) formatThought(td *thoughtData) string {
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
//   - td: The thoughtData to process
// Returns:
//   - result: The processed thought result
//   - err: An error if processing fails
func (t *thinkingServer) processThought(_ context.Context, td *thoughtData) (*thoughtResult, error) {
	validated, err := t.validate(td.Thought)
	if err != nil {
		return nil, err
	}
	if validated.ThoughtNumber > validated.TotalThoughts {
		validated.TotalThoughts = validated.ThoughtNumber
	}
	
	t.thoughtHistory = append(t.thoughtHistory, validated)
	
	if validated.BranchID != "" {
		t.branches[validated.BranchID] = append(t.branches[validated.BranchID], validated)
	}
	
	thought := t.formatThought(validated)
	
	return &thoughtResult{
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
