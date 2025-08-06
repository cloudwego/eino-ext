package jq

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/invopop/yaml"
)

// --- Test Setup ---

// testGenerateToken is a deterministic version of our custom function for testing.
func testGenerateToken(docID string, author any) (string, error) {
	authorStr, ok := author.(string)
	if !ok {
		authorStr = "unknown"
	}
	return fmt.Sprintf("TOKEN::%s::%s", strings.ToUpper(authorStr), docID), nil
}

// testFuncRegistry is the registry used by all tests.
var testFuncRegistry = map[string]any{
	"generate_user_token": testGenerateToken,
}

// newTestTransformer is a helper to quickly create a transformer from a YAML string.
func newTestTransformer(t *testing.T, yamlConfig string, funcRegistry map[string]any) *TransformerRules {
	t.Helper() // Marks this as a test helper function.

	var cfg Config
	if err := yaml.Unmarshal([]byte(yamlConfig), &cfg); err != nil {
		t.Fatalf("Failed to unmarshal test YAML config: %v", err)
	}

	rules, err := NewTransformerRules(&cfg, funcRegistry)
	if err != nil {
		t.Fatalf("NewTransformerRules failed: %v", err)
	}
	return rules
}

// --- Test Cases ---

func TestIndividualTransform(t *testing.T) {
	config := `
transform: |
  .content = (.content + " (transformed)")
  | .meta_data.status = "processed"
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{
			ID:       "doc-1",
			Content:  "Original",
			MetaData: map[string]any{"status": "new"},
		},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if len(transformed) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(transformed))
	}

	expectedContent := "Original (transformed)"
	if transformed[0].Content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, transformed[0].Content)
	}

	expectedStatus := "processed"
	if status := transformed[0].MetaData["status"]; status != expectedStatus {
		t.Errorf("Expected meta status '%s', got '%v'", expectedStatus, status)
	}
}

func TestJoinAggregation(t *testing.T) {
	config := `
aggregation:
  rules:
    - name: "Aggregate simple prose"
      source_selector: '.meta_data.type == "prose"'
      target_selector: '.meta_data.type == "def"'
      action:
        mode: "join"
        join_separator: " | "
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{ID: "prose-1", Content: "First part", MetaData: map[string]any{"type": "prose"}},
		{ID: "prose-2", Content: "Second part", MetaData: map[string]any{"type": "prose"}},
		{ID: "def-1", Content: "Definition.", MetaData: map[string]any{"type": "def"}},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	targetDoc := transformed[2]
	expectedContent := "Definition.\n\n--- Aggregated Content ---\n\nFirst part | Second part"
	if targetDoc.Content != expectedContent {
		t.Errorf("Expected aggregated content '%s', got '%s'", expectedContent, targetDoc.Content)
	}
}

func TestHierarchicalAggregation(t *testing.T) {
	config := `
aggregation:
  rules:
    - name: "Aggregate hierarchical"
      source_selector: '.meta_data.level != null'
      target_selector: '.meta_data.type == "def"'
      action:
        mode: "hierarchical_by_level"
        level_key: "level"
        join_separator: "\n"
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{ID: "L1", Content: "Level 1", MetaData: map[string]any{"level": 1}},
		{ID: "L2-v1", Content: "Level 2 OLD", MetaData: map[string]any{"level": 2}},
		{ID: "L2-v2", Content: "Level 2 NEW", MetaData: map[string]any{"level": 2}}, // Should overwrite L2-v1
		{ID: "L4", Content: "Level 4", MetaData: map[string]any{"level": 4}},        // Should be skipped
		{ID: "DEF", Content: "Target", MetaData: map[string]any{"type": "def", "level": 3}},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	targetDoc := transformed[4]
	// Expects aggregation in reverse order of levels: 2 then 1. Level 4 is ignored.
	expectedContent := "Level 1\nLevel 2 NEW\nTarget"
	if targetDoc.Content != expectedContent {
		t.Errorf("Expected hierarchical content '%s', got '%s'", expectedContent, targetDoc.Content)
	}
}

func TestCustomFunctionTransform(t *testing.T) {
	config := `
custom_transforms:
  - name: "Generate token"
    selector: '.meta_data.needs_token == true'
    function: "generate_user_token"
    target_key: "auth_token"
    args: [ .id, .meta_data.author ]
`
	rules := newTestTransformer(t, config, testFuncRegistry)

	docs := []*schema.Document{
		{ID: "user-123", MetaData: map[string]any{"needs_token": true, "author": "admin"}},
		{ID: "user-456", MetaData: map[string]any{"needs_token": false, "author": "guest"}},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Check the first document
	doc1 := transformed[0]
	expectedToken := "TOKEN::ADMIN::user-123"
	if token, ok := doc1.MetaData["auth_token"]; !ok || token != expectedToken {
		t.Errorf("Expected token '%s' for doc1, got '%v'", expectedToken, token)
	}

	// Check the second document
	doc2 := transformed[1]
	if _, ok := doc2.MetaData["auth_token"]; ok {
		t.Errorf("Expected no token for doc2, but found one")
	}
}

func TestNewTransformerRules_ErrorCases(t *testing.T) {
	t.Run("Nil Config", func(t *testing.T) {
		_, err := NewTransformerRules(nil, nil)
		if err == nil {
			t.Fatal("Expected an error for nil config, but got nil")
		}
	})

	t.Run("Invalid JQ Syntax", func(t *testing.T) {
		invalidConfig := `transform: ".| |"`
		var cfg Config
		if err := yaml.Unmarshal([]byte(invalidConfig), &cfg); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		_, err := NewTransformerRules(&cfg, nil)
		if err == nil {
			t.Fatal("Expected an error for invalid JQ syntax, but got nil")
		}
	})
}

func TestJqSplit(t *testing.T) {
	config := `
transform: |
  # Split tags by comma into an array
  .meta_data.tags_array = (if .meta_data.tags then .meta_data.tags | split(",") else null end)
  # Split categories by pipe
  | .meta_data.categories_array = (if .meta_data.categories then .meta_data.categories | split("|") else null end)
  # Trim whitespace from each array item
  | .meta_data.tags_array = (if .meta_data.tags_array then [.meta_data.tags_array[] | ltrimstr(" ") | rtrimstr(" ")] else null end)
  | .meta_data.categories_array = (if .meta_data.categories_array then [.meta_data.categories_array[] | ltrimstr(" ") | rtrimstr(" ")] else null end)
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{
			ID: "doc-1",
			MetaData: map[string]any{
				"tags":       "technology, science, programming",
				"categories": "book|reference|documentation",
			},
		},
		{
			ID: "doc-2",
			MetaData: map[string]any{
				"tags": "art, design",
				// No categories field
			},
		},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if len(transformed) != 2 {
		t.Fatalf("Expected 2 documents, got %d", len(transformed))
	}

	// Check first document
	tagsArray, ok := transformed[0].MetaData["tags_array"].([]any)
	if !ok {
		t.Errorf("Expected tags_array to be []any, got %T", transformed[0].MetaData["tags_array"])
	} else if len(tagsArray) != 3 || tagsArray[0] != "technology" || tagsArray[1] != "science" || tagsArray[2] != "programming" {
		t.Errorf("Expected tags_array to be [technology science programming], got %v", tagsArray)
	}

	categoriesArray, ok := transformed[0].MetaData["categories_array"].([]any)
	if !ok {
		t.Errorf("Expected categories_array to be []any, got %T", transformed[0].MetaData["categories_array"])
	} else if len(categoriesArray) != 3 || categoriesArray[0] != "book" || categoriesArray[1] != "reference" || categoriesArray[2] != "documentation" {
		t.Errorf("Expected categories_array to be [book reference documentation], got %v", categoriesArray)
	}

	// Check second document
	tagsArray2, ok := transformed[1].MetaData["tags_array"].([]any)
	if !ok {
		t.Errorf("Expected tags_array to be []any, got %T", transformed[1].MetaData["tags_array"])
	} else if len(tagsArray2) != 2 || tagsArray2[0] != "art" || tagsArray2[1] != "design" {
		t.Errorf("Expected tags_array to be [art design], got %v", tagsArray2)
	}

	// Verify null handling - this field should be null since there was no categories field
	if val, exists := transformed[1].MetaData["categories_array"]; exists && val != nil {
		t.Errorf("Expected categories_array to be nil, got %v", val)
	}
}

func TestJqJoinMetadata(t *testing.T) {
	config := `
transform: |
  # Join first and last name into a full name
  .meta_data.full_name = (if .meta_data.first_name and .meta_data.last_name then 
                         (.meta_data.first_name + " " + .meta_data.last_name) 
                       else null end)
  
  # Join multiple tags into a single string
  | .meta_data.joined_tags = (if .meta_data.tags_array then 
                             (.meta_data.tags_array | join(", ")) 
                            else null end)
  
  # Concatenate fields with custom formatting
  | .meta_data.citation = (if .meta_data.author and .meta_data.title and .meta_data.year then
                          (.meta_data.author + " (" + (.meta_data.year | tostring) + "). " + .meta_data.title)
                         else null end)
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{
			ID: "doc-1",
			MetaData: map[string]any{
				"first_name": "John",
				"last_name":  "Doe",
				"tags_array": []any{"research", "paper", "academic"}, // Changed from []string to []any
				"author":     "Smith, J.",
				"title":      "Understanding Metadata",
				"year":       2023,
			},
		},
		{
			ID: "doc-2",
			MetaData: map[string]any{
				"first_name": "Jane",
				"last_name":  "Smith",
				// Intentionally missing other fields
			},
		},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if len(transformed) != 2 {
		t.Fatalf("Expected 2 documents, got %d", len(transformed))
	}

	// Check first document's full name
	fullName, ok := transformed[0].MetaData["full_name"].(string)
	if !ok {
		t.Errorf("Expected full_name to be string, got %T", transformed[0].MetaData["full_name"])
	} else if fullName != "John Doe" {
		t.Errorf("Expected full_name to be 'John Doe', got '%s'", fullName)
	}

	// Check first document's joined tags
	joinedTags, ok := transformed[0].MetaData["joined_tags"].(string)
	if !ok {
		t.Errorf("Expected joined_tags to be string, got %T", transformed[0].MetaData["joined_tags"])
	} else if joinedTags != "research, paper, academic" {
		t.Errorf("Expected joined_tags to be 'research, paper, academic', got '%s'", joinedTags)
	}

	// Check first document's citation format
	citation, ok := transformed[0].MetaData["citation"].(string)
	if !ok {
		t.Errorf("Expected citation to be string, got %T", transformed[0].MetaData["citation"])
	} else if citation != "Smith, J. (2023). Understanding Metadata" {
		t.Errorf("Expected citation to be 'Smith, J. (2023). Understanding Metadata', got '%s'", citation)
	}

	// Check second document
	fullName2, ok := transformed[1].MetaData["full_name"].(string)
	if !ok {
		t.Errorf("Expected full_name to be string, got %T", transformed[1].MetaData["full_name"])
	} else if fullName2 != "Jane Smith" {
		t.Errorf("Expected full_name to be 'Jane Smith', got '%s'", fullName2)
	}

	// Second document should have null for the other fields
	if val, exists := transformed[1].MetaData["joined_tags"]; exists && val != nil {
		t.Errorf("Expected joined_tags to be nil, got %v", val)
	}

	if val, exists := transformed[1].MetaData["citation"]; exists && val != nil {
		t.Errorf("Expected citation to be nil, got %v", val)
	}
}
