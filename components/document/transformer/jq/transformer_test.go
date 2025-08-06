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
