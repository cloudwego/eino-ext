package jq

import (
	"context"
	"fmt"
	"reflect"
	"slices"
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

	rules, err := NewTransformerRules([]*Config{&cfg}, funcRegistry)
	if err != nil {
		t.Fatalf("NewTransformerRules failed: %v", err)
	}
	return rules
}

// newMultiConfigTestTransformer creates a transformer with multiple configs
func newMultiConfigTestTransformer(t *testing.T, yamlConfigs []string, funcRegistry map[string]any) *TransformerRules {
	t.Helper()

	configs := make([]*Config, len(yamlConfigs))
	for i, yamlConfig := range yamlConfigs {
		var cfg Config
		if err := yaml.Unmarshal([]byte(yamlConfig), &cfg); err != nil {
			t.Fatalf("Failed to unmarshal test YAML config %d: %v", i, err)
		}
		configs[i] = &cfg
	}

	rules, err := NewTransformerRules(configs, funcRegistry)
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
aggregations:  
  - name: "Aggregate simple prose"
    source: '.meta_data.type == "prose"'
    target: '.meta_data.type == "def"'
    action:
      join: " | "
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
	expectedContent := "First part | Second part | Definition."
	if targetDoc.Content != expectedContent {
		t.Errorf("Expected aggregated content '%s', got '%s'", expectedContent, targetDoc.Content)
	}
}

func TestHierarchicalAggregation(t *testing.T) {
	config := `
aggregations:
- name: "Aggregate hierarchical"
  source: '.meta_data.level != null'
  target: '.meta_data.type == "def"'
  action:
    hierarchy: "level"
    join: "\n"
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

func TestHierarchicalAggregationToField(t *testing.T) {
	config := `
aggregations:
- name: "Aggregate hierarchical"
  source: '.meta_data.level != null'
  target: '.meta_data.type == "def"'
  action:
    hierarchy: "level"
    target: "contents"
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
	expectedContent := []interface{}{"Level 1", "Level 2 NEW", "Target"}
	if !reflect.DeepEqual(targetDoc.MetaData["contents"], expectedContent) {
		t.Errorf("Expected hierarchical content '%v', got '%v'", expectedContent, targetDoc.MetaData["contents"])
	}
}

func TestCustomFunctionTransform(t *testing.T) {
	config := `
custom_transforms:
  - name: "Generate token"
    selector: '.meta_data.needs_token == true'
    function: "generate_user_token"
    target: "auth_token"
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
		_, err := NewTransformerRules([]*Config{&cfg}, nil)
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

func TestDocumentFiltering(t *testing.T) {
	config := `
filter: |
  .meta_data.type == "include"
transform: |
  .content = (.content + " (transformed)")
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{
			ID:       "doc-1",
			Content:  "Keep this",
			MetaData: map[string]any{"type": "include"},
		},
		{
			ID:       "doc-2",
			Content:  "Filter this out",
			MetaData: map[string]any{"type": "exclude"},
		},
		{
			ID:       "doc-3",
			Content:  "Keep this too",
			MetaData: map[string]any{"type": "include"},
		},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Should only have 2 documents after filtering
	if len(transformed) != 2 {
		t.Fatalf("Expected 2 documents after filtering, got %d", len(transformed))
	}

	// Verify IDs of remaining documents
	ids := []string{transformed[0].ID, transformed[1].ID}
	expectedIDs := []string{"doc-1", "doc-3"}

	for _, id := range expectedIDs {
		if !slices.Contains(ids, id) {
			t.Errorf("Expected document %s to be kept, but it was filtered out", id)
		}
	}

	// Verify transformation was applied to remaining documents
	for _, doc := range transformed {
		if !strings.HasSuffix(doc.Content, " (transformed)") {
			t.Errorf("Transformation not applied to document %s", doc.ID)
		}
	}
}

func TestMultipleConfigurations(t *testing.T) {
	config1 := `
transform: |
  .meta_data.stage = "first_transform"
  | .content = (.content + " (first)")
`
	config2 := `
transform: |
  .meta_data.stage = "second_transform"
  | .content = (.content + " (second)")
`
	rules := newMultiConfigTestTransformer(t, []string{config1, config2}, nil)

	docs := []*schema.Document{
		{
			ID:       "doc-1",
			Content:  "Original",
			MetaData: map[string]any{},
		},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if len(transformed) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(transformed))
	}

	// Document should be processed by both configs in sequence
	expectedContent := "Original (first) (second)"
	if transformed[0].Content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, transformed[0].Content)
	}

	// The metadata should reflect the last transformation
	expectedStage := "second_transform"
	if stage := transformed[0].MetaData["stage"]; stage != expectedStage {
		t.Errorf("Expected meta stage '%s', got '%v'", expectedStage, stage)
	}
}

func TestMultipleFilterConfigurations(t *testing.T) {
	config1 := `
filter: |
  .meta_data.score >= 50
transform: |
  .meta_data.passed_first = true
`
	config2 := `
filter: |
  .meta_data.category == "important"
transform: |
  .meta_data.passed_second = true
`
	rules := newMultiConfigTestTransformer(t, []string{config1, config2}, nil)

	docs := []*schema.Document{
		{
			ID:       "doc-1", // Will pass both filters
			Content:  "Important high score",
			MetaData: map[string]any{"score": 80, "category": "important"},
		},
		{
			ID:       "doc-2", // Will pass first filter only
			Content:  "High score but not important",
			MetaData: map[string]any{"score": 75, "category": "normal"},
		},
		{
			ID:       "doc-3", // Will be filtered out by first filter
			Content:  "Important but low score",
			MetaData: map[string]any{"score": 30, "category": "important"},
		},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Only doc-1 should pass both filters and have both flags
	if len(transformed) != 1 {
		t.Fatalf("Expected 1 document after filtering, got %d", len(transformed))
	}

	if transformed[0].ID != "doc-1" {
		t.Errorf("Expected doc-1 to pass both filters, got %s", transformed[0].ID)
	}

	if !transformed[0].MetaData["passed_first"].(bool) {
		t.Errorf("Expected doc-1 to have passed_first=true")
	}

	if !transformed[0].MetaData["passed_second"].(bool) {
		t.Errorf("Expected doc-1 to have passed_second=true")
	}
}

func TestAggregationWithFields(t *testing.T) {
	// Test with both source and target
	config := `
aggregations:
- name: "Aggregate metadata fields"
  source: '.meta_data.type == "source"'
  target: '.meta_data.type == "target"'
  action:
    source: "extract_me"
    target: "aggregated_data"
    join: ", "
- name: "Aggregate as array"
  source: '.meta_data.type == "array_source"'
  target: '.meta_data.type == "array_target"' 
  action:
    source: "extract_me"
    target: "aggregated_array"
    mode: "join"
    # No join - should result in array storage
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{ID: "source-1", MetaData: map[string]any{"type": "source", "extract_me": "Value One"}},
		{ID: "source-2", MetaData: map[string]any{"type": "source", "extract_me": "Value Two"}},
		{ID: "target-1", Content: "Target doc", MetaData: map[string]any{"type": "target"}},

		{ID: "array-source-1", MetaData: map[string]any{"type": "array_source", "extract_me": "Array One"}},
		{ID: "array-source-2", MetaData: map[string]any{"type": "array_source", "extract_me": "Array Two"}},
		{ID: "array-target", Content: "Array target", MetaData: map[string]any{"type": "array_target"}},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Check string aggregation
	targetDoc := transformed[2] // The target-1 document

	// Verify the metadata field contains the aggregated values
	if aggregated, ok := targetDoc.MetaData["aggregated_data"].(string); !ok {
		t.Errorf("Expected aggregated_data to be a string, got %T", targetDoc.MetaData["aggregated_data"])
	} else if aggregated != "Value One, Value Two" {
		t.Errorf("Expected aggregated value 'Value One, Value Two', got '%s'", aggregated)
	}

	// Verify the original content wasn't modified
	if targetDoc.Content != "Target doc" {
		t.Errorf("Expected content to remain 'Target doc', got '%s'", targetDoc.Content)
	}

	// Check array aggregation
	arrayTargetDoc := transformed[5] // The array-target document

	// Verify the metadata field contains the array
	if aggregatedArray, ok := arrayTargetDoc.MetaData["aggregated_array"].([]any); !ok {
		t.Errorf("Expected aggregated_array to be []any, got %T", arrayTargetDoc.MetaData["aggregated_array"])
	} else if len(aggregatedArray) != 2 {
		t.Errorf("Expected array with 2 items, got %d", len(aggregatedArray))
	} else if aggregatedArray[0] != "Array One" || aggregatedArray[1] != "Array Two" {
		t.Errorf("Expected array with ['Array One', 'Array Two'], got %v", aggregatedArray)
	}
}

func TestJqStringMapping(t *testing.T) {
	config := `
transform: |
  # Define mapping of country codes to country names
  .meta_data.country_name = (
    if .meta_data.country_code then
      .meta_data.country_code | {
        "US": "United States",
        "UK": "United Kingdom",
        "DE": "Germany",
        "FR": "France",
        "JP": "Japan",
        "CN": "China",
        "AU": "Australia"
      }[.] // "Unknown Country"  # Default value if not found
    else
      null
    end
  )
  
  # Another example: map categories to departments
  | .meta_data.department = (
    if .meta_data.category then
      .meta_data.category | {
        "books": "Literature",
        "electronics": "Technology",
        "clothing": "Fashion",
        "food": "Grocery",
        "toys": "Children"
      }[.] // .  # Keep original if not mapped (using the current value as fallback)
    else
      null
    end
  )
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{
			ID: "doc-1",
			MetaData: map[string]any{
				"country_code": "US",
				"category":     "books",
			},
		},
		{
			ID: "doc-2",
			MetaData: map[string]any{
				"country_code": "DE",
				"category":     "electronics",
			},
		},
		{
			ID: "doc-3",
			MetaData: map[string]any{
				"country_code": "XYZ",              // Not in mapping
				"category":     "unknown_category", // Not in mapping
			},
		},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if len(transformed) != 3 {
		t.Fatalf("Expected 3 documents, got %d", len(transformed))
	}

	// Check first document
	if countryName, ok := transformed[0].MetaData["country_name"].(string); !ok || countryName != "United States" {
		t.Errorf("Expected country_name to be 'United States', got %v", transformed[0].MetaData["country_name"])
	}
	if department, ok := transformed[0].MetaData["department"].(string); !ok || department != "Literature" {
		t.Errorf("Expected department to be 'Literature', got %v", transformed[0].MetaData["department"])
	}

	// Check second document
	if countryName, ok := transformed[1].MetaData["country_name"].(string); !ok || countryName != "Germany" {
		t.Errorf("Expected country_name to be 'Germany', got %v", transformed[1].MetaData["country_name"])
	}
	if department, ok := transformed[1].MetaData["department"].(string); !ok || department != "Technology" {
		t.Errorf("Expected department to be 'Technology', got %v", transformed[1].MetaData["department"])
	}

	// Check third document with unknown values
	if countryName, ok := transformed[2].MetaData["country_name"].(string); !ok || countryName != "Unknown Country" {
		t.Errorf("Expected country_name to be 'Unknown Country', got %v", transformed[2].MetaData["country_name"])
	}
	if department, ok := transformed[2].MetaData["department"].(string); !ok || department != "unknown_category" {
		t.Errorf("Expected department to be 'unknown_category', got %v", transformed[2].MetaData["department"])
	}
}

func TestJqArrayMapping(t *testing.T) {
	config := `
transform: |
  # 1. Map simple string array - capitalize each item
  .meta_data.tags_upper = (
    if .meta_data.tags_array then
      [.meta_data.tags_array[] | ascii_upcase]
    else
      null
    end
  )
  
  # 2. Transform object array - add attributes to each object
  | .meta_data.products_with_tax = (
    if .meta_data.products then
      [.meta_data.products[] | {
        name: .name,
        price: .price,
        tax: (.price * 0.1),
        total: (.price * 1.1)
      }]
    else
      null
    end
  )
  
  # 3. Selective mapping - only map items matching a condition
  | .meta_data.expensive_products = (
    if .meta_data.products then
      [.meta_data.products[] | select(.price > 50) | .name]
    else
      null
    end
  )
  
  # 4. Transform array with index - add position to each item
  | .meta_data.indexed_tags = (
    if .meta_data.tags_array then
      [range(0; (.meta_data.tags_array | length)) as $i | {
        index: $i,
        value: .meta_data.tags_array[$i],
        position: ($i + 1)
      }]
    else
      null
    end
  )
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{
			ID: "doc-1",
			MetaData: map[string]any{
				"tags_array": []any{"technology", "science", "programming"},
				"products": []any{
					map[string]any{"name": "Laptop", "price": 1200.0},
					map[string]any{"name": "Mouse", "price": 25.0},
					map[string]any{"name": "Monitor", "price": 300.0},
				},
			},
		},
		{
			ID: "doc-2",
			MetaData: map[string]any{
				"tags_array": []any{"art", "design"},
				// No products
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

	// Check string array capitalization
	tagsUpper, ok := transformed[0].MetaData["tags_upper"].([]any)
	if !ok {
		t.Errorf("Expected tags_upper to be []any, got %T", transformed[0].MetaData["tags_upper"])
	} else if len(tagsUpper) != 3 || tagsUpper[0] != "TECHNOLOGY" || tagsUpper[1] != "SCIENCE" || tagsUpper[2] != "PROGRAMMING" {
		t.Errorf("Expected uppercase tags, got %v", tagsUpper)
	}

	// Check object array transformation
	products, ok := transformed[0].MetaData["products_with_tax"].([]any)
	if !ok {
		t.Errorf("Expected products_with_tax to be []any, got %T", transformed[0].MetaData["products_with_tax"])
	} else {
		// Check first product
		product, ok := products[0].(map[string]any)
		if !ok {
			t.Errorf("Expected product to be map[string]any, got %T", products[0])
		} else {
			if product["name"] != "Laptop" {
				t.Errorf("Expected product name 'Laptop', got %v", product["name"])
			}
			if price, ok := product["price"].(float64); !ok || price != 1200.0 {
				t.Errorf("Expected product price 1200.0 (float64), got %v (%T)", product["price"], product["price"])
			}
			if tax, ok := product["tax"].(float64); !ok || tax != 120.0 {
				t.Errorf("Expected product tax 120.0 (float64), got %v (%T)", product["tax"], product["tax"])
			}
			if total, ok := product["total"].(float64); !ok || total != 1320.0 {
				t.Errorf("Expected product total 1320.0 (float64), got %v (%T)", product["total"], product["total"])
			}
		}
	}

	// Check filtered array
	expensiveProducts, ok := transformed[0].MetaData["expensive_products"].([]any)
	if !ok {
		t.Errorf("Expected expensive_products to be []any, got %T", transformed[0].MetaData["expensive_products"])
	} else if len(expensiveProducts) != 2 || expensiveProducts[0] != "Laptop" || expensiveProducts[1] != "Monitor" {
		t.Errorf("Expected expensive products to be [Laptop Monitor], got %v", expensiveProducts)
	}

	// Check indexed array
	indexedTags, ok := transformed[0].MetaData["indexed_tags"].([]any)
	if !ok {
		t.Errorf("Expected indexed_tags to be []any, got %T", transformed[0].MetaData["indexed_tags"])
	} else if len(indexedTags) != 3 {
		t.Errorf("Expected 3 indexed tags, got %d", len(indexedTags))
	} else {
		firstTag, ok := indexedTags[0].(map[string]any)
		if !ok {
			t.Errorf("Expected indexed tag to be map[string]any, got %T", indexedTags[0])
		} else {
			// Accept either int or float64 for index
			if idx, ok := firstTag["index"].(int); ok {
				if idx != 0 {
					t.Errorf("Expected index 0, got %v", idx)
				}
			} else if idx, ok := firstTag["index"].(float64); ok {
				if idx != 0.0 {
					t.Errorf("Expected index 0.0, got %v", idx)
				}
			} else {
				t.Errorf("Expected index to be int or float64, got %T", firstTag["index"])
			}

			if firstTag["value"] != "technology" {
				t.Errorf("Expected value 'technology', got %v", firstTag["value"])
			}

			// Accept either int or float64 for position
			if pos, ok := firstTag["position"].(int); ok {
				if pos != 1 {
					t.Errorf("Expected position 1, got %v", pos)
				}
			} else if pos, ok := firstTag["position"].(float64); ok {
				if pos != 1.0 {
					t.Errorf("Expected position 1.0, got %v", pos)
				}
			} else {
				t.Errorf("Expected position to be int or float64, got %T", firstTag["position"])
			}
		}
	}

	// Check second document with fewer fields
	tagsUpper2, ok := transformed[1].MetaData["tags_upper"].([]any)
	if !ok {
		t.Errorf("Expected tags_upper to be []any, got %T", transformed[1].MetaData["tags_upper"])
	} else if len(tagsUpper2) != 2 || tagsUpper2[0] != "ART" || tagsUpper2[1] != "DESIGN" {
		t.Errorf("Expected uppercase tags [ART DESIGN], got %v", tagsUpper2)
	}

	// Products field should be null for second document
	if transformed[1].MetaData["products_with_tax"] != nil {
		t.Errorf("Expected products_with_tax to be nil, got %v", transformed[1].MetaData["products_with_tax"])
	}
}

func TestJqCaseInsensitiveMapping(t *testing.T) {
	config := `
transform: |
  # Case-insensitive mapping by converting both keys and lookup value to lowercase
  .meta_data.normalized_category = (
    if .meta_data.category then
      # Convert input to lowercase for case-insensitive comparison
      (.meta_data.category | ascii_downcase) as $lookup |
      # Define mapping with lowercase keys
      {
        "electronics": "Technology Department",
        "books": "Literature Department",
        "clothing": "Fashion Department",
        "food": "Grocery Department"
      }[$lookup] // (.meta_data.category + " Department")  # Default adds "Department" suffix
    else
      null
    end
  )
  
  # Alternative: Case-insensitive lookup with preserved original values in result
  | .meta_data.country_name = (
    if .meta_data.country_code then
      # Store original for later use
      .meta_data.country_code as $original |
      # Convert to lowercase for lookup
      (.meta_data.country_code | ascii_downcase) as $lookup |
      # Lookup table maps lowercase keys to proper case values
      {
        "us": "United States",
        "uk": "United Kingdom",
        "de": "Germany",
        "fr": "France", 
        "jp": "Japan"
      }[$lookup] // ("Unknown: " + $original)  # Default preserves original case
    else
      null
    end
  )
`
	rules := newTestTransformer(t, config, nil)

	docs := []*schema.Document{
		{
			ID: "doc-1",
			MetaData: map[string]any{
				"country_code": "US",    // Uppercase
				"category":     "Books", // Title case
			},
		},
		{
			ID: "doc-2",
			MetaData: map[string]any{
				"country_code": "fr",       // Lowercase
				"category":     "CLOTHING", // All caps
			},
		},
		{
			ID: "doc-3",
			MetaData: map[string]any{
				"country_code": "MX",                 // Not in mapping
				"category":     "Electronics Repair", // Not exact match
			},
		},
	}

	transformed, err := rules.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	if len(transformed) != 3 {
		t.Fatalf("Expected 3 documents, got %d", len(transformed))
	}

	// Check first document - sent "US" and "Books"
	if countryName, ok := transformed[0].MetaData["country_name"].(string); !ok || countryName != "United States" {
		t.Errorf("Expected country_name to be 'United States', got %v", transformed[0].MetaData["country_name"])
	}
	if category, ok := transformed[0].MetaData["normalized_category"].(string); !ok || category != "Literature Department" {
		t.Errorf("Expected normalized_category to be 'Literature Department', got %v", transformed[0].MetaData["normalized_category"])
	}

	// Check second document - sent "fr" and "CLOTHING"
	if countryName, ok := transformed[1].MetaData["country_name"].(string); !ok || countryName != "France" {
		t.Errorf("Expected country_name to be 'France', got %v", transformed[1].MetaData["country_name"])
	}
	if category, ok := transformed[1].MetaData["normalized_category"].(string); !ok || category != "Fashion Department" {
		t.Errorf("Expected normalized_category to be 'Fashion Department', got %v", transformed[1].MetaData["normalized_category"])
	}

	// Check third document - with values not in mapping
	if countryName, ok := transformed[2].MetaData["country_name"].(string); !ok || countryName != "Unknown: MX" {
		t.Errorf("Expected country_name to be 'Unknown: MX', got %v", transformed[2].MetaData["country_name"])
	}
	if category, ok := transformed[2].MetaData["normalized_category"].(string); !ok || category != "Electronics Repair Department" {
		t.Errorf("Expected normalized_category to be 'Electronics Repair Department', got %v", transformed[2].MetaData["normalized_category"])
	}
}
