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
	"testing"
	
	"github.com/bytedance/mockey"
	"github.com/bytedance/sonic"
	"github.com/smartystreets/goconvey/convey"
)

// Helper functions for tests to create valid thoughtData JSON
func createThoughtJSON(thought string, thoughtNumber, totalThoughts int, nextThoughtNeeded bool) string {
	td := thoughtData{
		Thought:           thought,
		ThoughtNumber:     thoughtNumber,
		TotalThoughts:     totalThoughts,
		NextThoughtNeeded: nextThoughtNeeded,
	}
	data, _ := sonic.MarshalString(td)
	return data
}

func createBranchThoughtJSON(thought string, thoughtNumber, totalThoughts, branchFrom int, branchID string, nextThoughtNeeded bool) string {
	td := thoughtData{
		Thought:           thought,
		ThoughtNumber:     thoughtNumber,
		TotalThoughts:     totalThoughts,
		BranchFromThought: branchFrom,
		BranchID:          branchID,
		NextThoughtNeeded: nextThoughtNeeded,
	}
	data, _ := sonic.MarshalString(td)
	return data
}

func TestNewThinkingServer(t *testing.T) {
	convey.Convey("Test NewThinkingServer", t, func() {
		server := newThinkingServer()
		convey.So(server, convey.ShouldNotBeNil)
		convey.So(server.thoughtHistory, convey.ShouldNotBeNil)
		convey.So(server.branches, convey.ShouldNotBeNil)
		convey.So(len(server.thoughtHistory), convey.ShouldEqual, 0)
		convey.So(len(server.branches), convey.ShouldEqual, 0)
	})
}

func TestValidate(t *testing.T) {
	server := newThinkingServer()
	
	convey.Convey("Test validate with valid input", t, func() {
		validJSON := createThoughtJSON("This is a test thought", 1, 3, true)
		td, err := server.validate(validJSON)
		convey.So(err, convey.ShouldBeNil)
		convey.So(td, convey.ShouldNotBeNil)
		convey.So(td.Thought, convey.ShouldEqual, "This is a test thought")
		convey.So(td.ThoughtNumber, convey.ShouldEqual, 1)
		convey.So(td.TotalThoughts, convey.ShouldEqual, 3)
		convey.So(td.NextThoughtNeeded, convey.ShouldBeTrue)
	})
	
	convey.Convey("Test validate with invalid JSON", t, func() {
		invalidJSON := "{"
		td, err := server.validate(invalidJSON)
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(td, convey.ShouldBeNil)
	})
	
	convey.Convey("Test validate with missing required fields", t, func() {
		// Missing thought
		missingThought := `{"thought_number": 1, "total_thoughts": 3, "next_thought_needed": true}`
		td, err := server.validate(missingThought)
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(td, convey.ShouldNotBeNil)
		
		// Invalid thought number
		invalidThoughtNumber := `{"thought": "Test", "thought_number": 0, "total_thoughts": 3, "next_thought_needed": true}`
		td, err = server.validate(invalidThoughtNumber)
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(td, convey.ShouldNotBeNil)
		
		// Invalid total thoughts
		invalidTotalThoughts := `{"thought": "Test", "thought_number": 1, "total_thoughts": 0, "next_thought_needed": true}`
		td, err = server.validate(invalidTotalThoughts)
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(td, convey.ShouldNotBeNil)
	})
}

func TestFormatThought(t *testing.T) {
	server := newThinkingServer()
	
	convey.Convey("Test formatThought with regular thought", t, func() {
		td := &thoughtData{
			Thought:           "This is a regular thought",
			ThoughtNumber:     1,
			TotalThoughts:     3,
			NextThoughtNeeded: true,
		}
		formatted := server.formatThought(td)
		convey.So(formatted, convey.ShouldContainSubstring, "💭 Thought 1/3")
		convey.So(formatted, convey.ShouldContainSubstring, "This is a regular thought")
	})
	
	convey.Convey("Test formatThought with revision thought", t, func() {
		td := &thoughtData{
			Thought:           "This is a revision thought",
			ThoughtNumber:     2,
			TotalThoughts:     3,
			IsRevision:        true,
			RevisesThought:    1,
			NextThoughtNeeded: true,
		}
		formatted := server.formatThought(td)
		convey.So(formatted, convey.ShouldContainSubstring, "🔄 Revision 2/3")
		convey.So(formatted, convey.ShouldContainSubstring, "revising thought 1")
		convey.So(formatted, convey.ShouldContainSubstring, "This is a revision thought")
	})
	
	convey.Convey("Test formatThought with branch thought", t, func() {
		td := &thoughtData{
			Thought:           "This is a branch thought",
			ThoughtNumber:     2,
			TotalThoughts:     4,
			BranchFromThought: 1,
			BranchID:          "branch-1",
			NextThoughtNeeded: true,
		}
		formatted := server.formatThought(td)
		convey.So(formatted, convey.ShouldContainSubstring, "🌿 Branch 2/4")
		convey.So(formatted, convey.ShouldContainSubstring, "from thought 1")
		convey.So(formatted, convey.ShouldContainSubstring, "ID: branch-1")
		convey.So(formatted, convey.ShouldContainSubstring, "This is a branch thought")
	})
}

func TestProcessThought(t *testing.T) {
	mockey.PatchConvey("Test ProcessThought", t, func() {
		ctx := context.Background()
		server := newThinkingServer()
		
		mockey.PatchConvey("Test process valid thought", func() {
			jsonString := createThoughtJSON("First thought", 1, 3, true)
			result, err := server.processThought(ctx, &thoughtData{Thought: jsonString})
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result.Content, convey.ShouldContainSubstring, "First thought")
			convey.So(result.ThoughtNumber, convey.ShouldEqual, 1)
			convey.So(result.TotalThoughts, convey.ShouldEqual, 3)
			convey.So(result.NextThoughtNeeded, convey.ShouldBeTrue)
			convey.So(result.ThoughtHistoryLength, convey.ShouldEqual, 1)
		})
		
		mockey.PatchConvey("Test process invalid thought", func() {
			invalidJSON := "{"
			result, err := server.processThought(ctx, &thoughtData{Thought: invalidJSON})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldBeNil)
		})
		
		mockey.PatchConvey("Test thought with thought number > total thoughts", func() {
			// Create a thought where the thought number is greater than total thoughts
			jsonString := createThoughtJSON("Exceeding thought", 5, 3, true)
			result, err := server.processThought(ctx, &thoughtData{Thought: jsonString})
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result.ThoughtNumber, convey.ShouldEqual, 5)
			convey.So(result.TotalThoughts, convey.ShouldEqual, 5) // Should be adjusted to match thought number
		})
		
		mockey.PatchConvey("Test branching thought", func() {
			// First add a regular thought
			jsonString1 := createThoughtJSON("First thought", 1, 3, true)
			_, err := server.processThought(ctx, &thoughtData{Thought: jsonString1})
			convey.So(err, convey.ShouldBeNil)
			
			// Then add a branch thought
			jsonString2 := createBranchThoughtJSON("Branch thought", 2, 4, 1, "test-branch", true)
			result, err := server.processThought(ctx, &thoughtData{Thought: jsonString2})
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result.ThoughtHistoryLength, convey.ShouldEqual, 2)
			convey.So(len(result.Branches), convey.ShouldEqual, 1)
			convey.So(result.Branches[0], convey.ShouldEqual, "test-branch")
		})
	})
}

func TestNewTool(t *testing.T) {
	mockey.PatchConvey("Test NewTool", t, func() {
		// Mock the InferTool function to test a success case
		tool, err := NewTool()
		convey.So(err, convey.ShouldBeNil)
		convey.So(tool, convey.ShouldNotBeNil) // Because our mock returns nil
		
		// Test failure case
		mockey.Mock(newThinkingServer).Return(nil).Build()
		tool, err = NewTool()
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(err.Error(), convey.ShouldContainSubstring, "failed to create thinking server")
		convey.So(tool, convey.ShouldBeNil)
	})
}

func TestHelperFunctions(t *testing.T) {
	convey.Convey("Test padEnd function", t, func() {
		convey.So(padEnd("test", 8), convey.ShouldEqual, "test    ")
		convey.So(padEnd("test", 4), convey.ShouldEqual, "test")
		convey.So(padEnd("test", 2), convey.ShouldEqual, "test")
	})
	
	convey.Convey("Test max function", t, func() {
		convey.So(max(5, 3), convey.ShouldEqual, 5)
		convey.So(max(3, 5), convey.ShouldEqual, 5)
		convey.So(max(5, 5), convey.ShouldEqual, 5)
	})
	
	convey.Convey("Test getKeys function", t, func() {
		m := map[string][]*thoughtData{
			"key1": {},
			"key2": {},
		}
		keys := getKeys(m)
		convey.So(len(keys), convey.ShouldEqual, 2)
		convey.So(keys, convey.ShouldContain, "key1")
		convey.So(keys, convey.ShouldContain, "key2")
	})
}
