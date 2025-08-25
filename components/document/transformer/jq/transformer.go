package jq

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"github.com/itchyny/gojq"
)

// --- Public Transformer Entrypoint ---

type Transformer struct {
	Rules *TransformerRules
}

// implOptions is used to extract the rules from the generic TransformerOption.
type implOptions struct {
	Rules *TransformerRules
}

func WithRules(Rules *TransformerRules) document.TransformerOption {
	return document.WrapTransformerImplSpecificOptFn(func(o *implOptions) {
		o.Rules = Rules
	})
}

// Transform is the main entry point called by the Eino framework.
func (t *Transformer) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) (documents []*schema.Document, err error) {
	option := implOptions{}
	document.GetTransformerImplSpecificOptions(&option, opts...)

	rules := option.Rules
	if rules == nil {
		rules = t.Rules
	}

	if rules != nil {
		documents, err = rules.Transform(ctx, docs, opts...)
	} else {
		log.Println("No jq transformer rules provided, skipping transformation.")
		documents = docs
	}
	return
}

// --- Configuration Structs with JSON annotations ---
type AggregationAction struct {
	Source         string  `yaml:"source" json:"source"`
	Target         string  `yaml:"target" json:"target"`
	Join           *string `yaml:"join,omitempty" json:"join,omitempty"`
	Hierarchy      *string `yaml:"hierarchy,omitempty" json:"hierarchy,omitempty"`
	HierarchyUntil *int    `yaml:"hierarchy_until,omitempty" json:"hierarchy_until,omitempty"`
	HierarchySkip  []int   `yaml:"hierarchy_skip,omitempty" json:"hierarchy_skip,omitempty"`
	LevelPrefix    *string `yaml:"level_prefix,omitempty" json:"level_prefix,omitempty"`
}

type Aggregation struct {
	Name   string            `yaml:"name" json:"name"`
	Source string            `yaml:"source" json:"source"`
	Target string            `yaml:"target" json:"target"`
	Action AggregationAction `yaml:"action" json:"action"`

	sourceQuery *gojq.Query
	targetQuery *gojq.Query
}

type CustomTransform struct {
	Name     string   `yaml:"name" json:"name"`
	Selector string   `yaml:"selector" json:"selector"`
	Function string   `yaml:"function" json:"function"`
	Target   string   `yaml:"target" json:"target"`
	Args     []string `yaml:"args" json:"args"`

	selectorQuery *gojq.Query
	argQueries    []*gojq.Query
}

type Config struct {
	Transform        string            `yaml:"transform" json:"transform"`
	Filter           string            `yaml:"filter" json:"filter"`
	Aggregations     []Aggregation     `yaml:"aggregations" json:"aggregations"`
	CustomTransforms []CustomTransform `yaml:"custom_transforms" json:"custom_transforms"`
}

// --- Core Logic Implementation ---

// ConfigRules stores the compiled queries and rules for a single config
type ConfigRules struct {
	transformQuery   *gojq.Query
	filterQuery      *gojq.Query
	aggregations     []Aggregation
	customTransforms []CustomTransform
}

type TransformerRules struct {
	configRules      []*ConfigRules
	functionRegistry map[string]any
}

// LeveledDocument represents a document with an associated level
type LeveledDocument struct {
	Level int
	Doc   *schema.Document
}

func NewTransformerRules(cfgs []*Config, funcRegistry map[string]any) (transformer *TransformerRules, err error) {
	if len(cfgs) == 0 {
		err = fmt.Errorf("configurations cannot be nil or empty")
		return
	}

	transformer = &TransformerRules{
		functionRegistry: funcRegistry,
		configRules:      make([]*ConfigRules, 0, len(cfgs)),
	}

	for _, cfg := range cfgs {
		configRule := &ConfigRules{
			aggregations:     cfg.Aggregations,
			customTransforms: cfg.CustomTransforms,
		}

		// Parse transform query if provided
		if cfg.Transform != "" {
			if configRule.transformQuery, err = gojq.Parse(cfg.Transform); err != nil {
				err = fmt.Errorf("failed to parse transform query: %w", err)
				return
			}
		}

		// Parse filter query if provided
		if cfg.Filter != "" {
			if configRule.filterQuery, err = gojq.Parse(cfg.Filter); err != nil {
				err = fmt.Errorf("failed to parse filter query: %w", err)
				return
			}
		}

		// Parse aggregation rules
		for i := range configRule.aggregations {
			rule := &configRule.aggregations[i]
			if rule.sourceQuery, err = gojq.Parse(rule.Source); err != nil {
				err = fmt.Errorf("failed to parse source selector for rule '%s': %w", rule.Name, err)
				return
			}
			if rule.targetQuery, err = gojq.Parse(rule.Target); err != nil {
				err = fmt.Errorf("failed to parse target selector for rule '%s': %w", rule.Name, err)
				return
			}
		}

		// Parse custom transforms
		for i := range configRule.customTransforms {
			rule := &configRule.customTransforms[i]
			if rule.selectorQuery, err = gojq.Parse(rule.Selector); err != nil {
				err = fmt.Errorf("failed to parse selector for transform '%s': %w", rule.Name, err)
				return
			}
			rule.argQueries = make([]*gojq.Query, len(rule.Args))
			for j, argExpr := range rule.Args {
				if rule.argQueries[j], err = gojq.Parse(argExpr); err != nil {
					err = fmt.Errorf("failed to parse arg %d for transform '%s': %w", j, rule.Name, err)
					return
				}
			}
		}

		transformer.configRules = append(transformer.configRules, configRule)
	}
	return
}

func (t *TransformerRules) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) (documents []*schema.Document, err error) {
	documents = docs

	// Process documents through each config in sequence
	for _, configRule := range t.configRules {
		if documents, err = t.applyConfigRules(ctx, configRule, documents); err != nil {
			return
		}
	}

	return
}

func (t *TransformerRules) applyConfigRules(_ context.Context, rules *ConfigRules, docs []*schema.Document) (documents []*schema.Document, err error) {
	// Filter documents if filter query is defined
	filteredDocs := docs
	if rules.filterQuery != nil {
		filteredDocs = make([]*schema.Document, 0, len(docs))
		for _, doc := range docs {
			docAsMap := docToMap(doc)
			var keep bool
			if keep, err = runBoolQuery(rules.filterQuery, docAsMap); err != nil {
				err = fmt.Errorf("document filtering failed: %w", err)
				return
			} else if keep {
				filteredDocs = append(filteredDocs, doc)
			}
		}
	}

	// Prepare buffers for aggregation - track sources for each rule
	joinBuffers := make(map[string][]*schema.Document)
	hierarchicalBuffers := make(map[string][]*LeveledDocument)

	// Process each document in a single pass
	for _, doc := range filteredDocs {
		docAsMap := docToMap(doc)

		// Apply individual transformation
		if rules.transformQuery != nil {
			var transformedMap map[string]any
			if transformedMap, err = runMapQuery(rules.transformQuery, docAsMap); err != nil {
				return
			} else {
				updateDocFromMap(doc, transformedMap)
				// Refresh map in case the transform changed it
				docAsMap = docToMap(doc)
			}
		}

		// Apply aggregation rules - first check if document is a target
		for i := range rules.aggregations {
			rule := &rules.aggregations[i]

			// Check if document is a target for aggregation
			var isTarget bool
			if isTarget, err = runBoolQuery(rule.targetQuery, docAsMap); err != nil {
				err = fmt.Errorf("rule '%s' target check failed: %w", rule.Name, err)
				return
			}

			if isTarget {
				// Apply appropriate aggregation using previously collected sources
				if rule.Action.Hierarchy != nil {
					// Apply hierarchical aggregation
					if err = t.applyHierarchicalAggregation(doc, rule, hierarchicalBuffers[rule.Name]); err != nil {
						return
					}
				} else {
					// Apply join aggregation
					if sourceDocs := joinBuffers[rule.Name]; len(sourceDocs) > 0 {
						// Collect content from source documents (only previous docs)
						contentsToAggregate := []interface{}{}
						for _, sourceDoc := range sourceDocs {
							contentsToAggregate = appendFieldOrContent(contentsToAggregate, rule.Action.Source, sourceDoc)
						}

						// Add current document's content if needed
						contentsToAggregate = appendFieldOrContent(contentsToAggregate, rule.Action.Source, doc)

						// Apply aggregated content to target
						applyAggregatedContent(doc, rule.Action, contentsToAggregate)
					}
				}
			}

			// After processing as a target, check if it's also a source
			var isSource bool
			if isSource, err = runBoolQuery(rule.sourceQuery, docAsMap); err != nil {
				err = fmt.Errorf("rule '%s' source check failed: %w", rule.Name, err)
				return
			}

			if isSource {
				// Add to appropriate buffer for future targets
				if rule.Action.Hierarchy != nil {
					var sourceLevel int
					if sourceLevel, err = getHierarchyValue(doc, *rule.Action.Hierarchy); err != nil {
						return
					}

					// Create a new LeveledDocument
					leveledDoc := &LeveledDocument{
						Level: sourceLevel,
						Doc:   doc,
					}

					hierarchicalBuffers[rule.Name] = append(hierarchicalBuffers[rule.Name], leveledDoc)
				} else {
					joinBuffers[rule.Name] = append(joinBuffers[rule.Name], doc)
				}
			}
		}

		// Apply custom transforms
		for i := range rules.customTransforms {
			rule := &rules.customTransforms[i]
			var isTarget bool
			if isTarget, err = runBoolQuery(rule.selectorQuery, docAsMap); err != nil {
				err = fmt.Errorf("rule '%s' selector check failed: %w", rule.Name, err)
				return
			} else if isTarget {
				if err = t.applyCustomFunction(doc, rule, docAsMap); err != nil {
					return
				}
			}
		}
	}

	documents = filteredDocs
	return
}

// --- Helper functions ---

// Common helper functions for query execution
func runBoolQuery(query *gojq.Query, input map[string]any) (result bool, err error) {
	iter := query.Run(input)
	var v interface{}
	var ok bool
	if v, ok = iter.Next(); !ok {
		result = false
		return
	}
	if e, isErr := v.(error); isErr {
		err = e
		return
	}
	var boolResult bool
	if boolResult, ok = v.(bool); !ok {
		err = fmt.Errorf("query did not return a boolean, got %T", v)
		return
	}
	result = boolResult
	return
}

func runMapQuery(query *gojq.Query, input map[string]any) (result map[string]any, err error) {
	iter := query.Run(input)
	var v interface{}
	var ok bool
	if v, ok = iter.Next(); !ok {
		result = input
		return
	}
	if e, isErr := v.(error); isErr {
		err = fmt.Errorf("query error: %w", e)
		return
	}
	if result, ok = v.(map[string]any); !ok {
		err = fmt.Errorf("query did not return a map")
		return
	}
	return
}

// Update document from transformed map
func updateDocFromMap(doc *schema.Document, transformedMap map[string]any) {
	if newContent, exists := transformedMap["content"].(string); exists {
		doc.Content = newContent
	}
	if newMeta, exists := transformedMap["meta_data"].(map[string]any); exists {
		doc.MetaData = newMeta
	}
}

// Helper function to get level value from document
func getHierarchyValue(doc *schema.Document, hierarchyField string) (level int, err error) {
	hierarchy, ok := doc.MetaData[hierarchyField]
	if !ok {
		err = fmt.Errorf("doc %s missing hierarchy field '%s'", doc.ID, hierarchyField)
		return
	}

	var isInt bool
	if level, isInt = ToInt(hierarchy); !isInt {
		err = fmt.Errorf("hierarchy field for doc %s is not a valid integer", doc.ID)
		return
	}

	return
}

// Apply aggregated content to the target document
func applyAggregatedContent(doc *schema.Document, action AggregationAction, contents []interface{}) {
	if action.Target != "" {
		if action.Join == nil {
			doc.MetaData[action.Target] = contents
		} else {
			doc.MetaData[action.Target] = JoinToStr(contents, *action.Join)
		}
	} else {
		separator := "\t"
		if action.Join != nil {
			separator = *action.Join
		}
		doc.Content = JoinToStr(contents, separator)
	}
}

func appendFieldOrContent(contentsToAggregate []interface{}, sourceField string, doc *schema.Document) []interface{} {
	if sourceField != "" {
		// Extract from specific field
		if fieldValue, ok := doc.MetaData[sourceField]; ok {
			contentsToAggregate = append(contentsToAggregate, fieldValue)
		}
	} else {
		contentsToAggregate = append(contentsToAggregate, doc.Content)
	}
	return contentsToAggregate
}

func (t *TransformerRules) applyCustomFunction(doc *schema.Document, rule *CustomTransform, docAsMap map[string]any) (err error) {
	goFunc, ok := t.functionRegistry[rule.Function]
	if !ok {
		err = fmt.Errorf("custom function '%s' not found", rule.Function)
		return
	}
	var args []reflect.Value
	if args, err = t.extractArgs(rule.argQueries, docAsMap); err != nil {
		err = fmt.Errorf("failed to extract args for '%s': %w", rule.Function, err)
		return
	}
	results := reflect.ValueOf(goFunc).Call(args)
	if len(results) > 1 && !results[1].IsNil() {
		if errResult, ok := results[1].Interface().(error); ok {
			err = fmt.Errorf("custom function '%s' returned an error: %w", rule.Function, errResult)
			return
		}
	}
	doc.MetaData[rule.Target] = results[0].Interface()
	return
}

func (t *TransformerRules) extractArgs(argQueries []*gojq.Query, input map[string]any) (args []reflect.Value, err error) {
	args = make([]reflect.Value, len(argQueries))
	for i, q := range argQueries {
		iter := q.Run(input)
		var v interface{}
		var ok bool
		if v, ok = iter.Next(); !ok {
			err = fmt.Errorf("arg query %d produced no value", i)
			args = nil
			return
		}
		args[i] = reflect.ValueOf(v)
	}
	return
}

// docToMap is now used consistently to create a reliable input for gojq.
func docToMap(doc *schema.Document) (result map[string]any) {
	metaCopy := make(map[string]any, len(doc.MetaData))
	for k, v := range doc.MetaData {
		metaCopy[k] = v
	}
	result = map[string]any{"id": doc.ID, "content": doc.Content, "meta_data": metaCopy}
	return
}

func ToInt(v any) (i int, ok bool) {
	switch val := v.(type) {
	case int:
		i, ok = val, true
	case int32:
		i, ok = int(val), true
	case int64:
		i, ok = int(val), true
	case float32:
		i, ok = int(val), true
	case float64:
		i, ok = int(val), true
	default:
		valueStr := fmt.Sprintf("%v", v)
		var err error
		if i, err = strconv.Atoi(valueStr); err == nil {
			ok = true
		}
	}
	return
}

func JoinToStr(input []interface{}, separator string) (result string) {
	strs := make([]string, len(input))
	for i, v := range input {
		strs[i] = ToStr(v)
	}
	result = strings.Join(strs, separator)
	return
}

func ToStr(input interface{}) (ret string) {
	switch val := input.(type) {
	case string:
		ret = val
	default:
		ret = fmt.Sprintf("%v", input)
	}
	return
}

// Helper function to apply hierarchical aggregation
func (t *TransformerRules) applyHierarchicalAggregation(doc *schema.Document, rule *Aggregation, buffer []*LeveledDocument) (err error) {
	if len(buffer) == 0 {
		return nil
	}

	var targetLevel int
	if targetLevel, err = getHierarchyValue(doc, *rule.Action.Hierarchy); err != nil {
		return
	}

	// We need to collect the most recent document for each level without gaps
	// First sort the buffer from newest to oldest (assuming buffer is in order of appearance)
	// This allows us to only take the most recent document at each level

	// Find the most recent valid document at each level
	mostRecentByLevel := make(map[int]*LeveledDocument)
	for i := len(buffer) - 1; i >= 0; i-- {
		leveledDoc := buffer[i]
		// Only consider levels below the target
		if leveledDoc.Level < targetLevel {
			// Skip levels specified in HierarchySkip
			skipLevel := false
			if rule.Action.HierarchySkip != nil {
				for _, skip := range rule.Action.HierarchySkip {
					if leveledDoc.Level == skip {
						skipLevel = true
						break
					}
				}
			}

			// Skip levels below HierarchyUntil if specified
			if !skipLevel && (rule.Action.HierarchyUntil == nil || leveledDoc.Level >= *rule.Action.HierarchyUntil) {
				// If we haven't seen this level yet, record it
				if _, exists := mostRecentByLevel[leveledDoc.Level]; !exists {
					mostRecentByLevel[leveledDoc.Level] = leveledDoc
				}
			}
		}
	}

	// Now collect documents from consecutive levels without gaps
	contentsToAggregate := []interface{}{}
	joinSeparator := "\t"
	if rule.Action.Join != nil {
		joinSeparator = *rule.Action.Join
	}

	// Start from the level immediately below the target and work backwards
	// until we find a gap
	for level := targetLevel - 1; level >= 1; level-- {
		leveledDoc, exists := mostRecentByLevel[level]
		if !exists {
			// We found a gap, stop collecting
			break
		}

		// Extract content from this level's document
		levelContent := ""
		levelContents := appendFieldOrContent([]interface{}{}, rule.Action.Source, leveledDoc.Doc)

		if len(levelContents) > 0 {
			levelContent = ToStr(levelContents[0])

			// Add level prefix if configured
			if rule.Action.LevelPrefix != nil {
				levelContent = fmt.Sprintf("%s %d: %s", *rule.Action.LevelPrefix, level, levelContent)
			}

			// Add to the beginning since we're going backwards
			contentsToAggregate = append([]interface{}{levelContent}, contentsToAggregate...)
		}
	}

	// Add the target document's content
	targetContents := appendFieldOrContent([]interface{}{}, rule.Action.Source, doc)
	if len(targetContents) > 0 {
		targetContent := ToStr(targetContents[0])

		// Add level prefix if configured
		if rule.Action.LevelPrefix != nil {
			targetContent = fmt.Sprintf("%s %d: %s", *rule.Action.LevelPrefix, targetLevel, targetContent)
		}

		contentsToAggregate = append(contentsToAggregate, targetContent)
	}

	// Apply aggregated content if there's anything to aggregate
	if len(contentsToAggregate) > 0 {
		// Use the same field/content application logic but with simplified handling
		if rule.Action.Target != "" {
			if rule.Action.Join == nil {
				doc.MetaData[rule.Action.Target] = contentsToAggregate
			} else {
				doc.MetaData[rule.Action.Target] = JoinToStr(contentsToAggregate, *rule.Action.Join)
			}
		} else {
			doc.Content = JoinToStr(contentsToAggregate, joinSeparator)
		}
	}

	return nil
}
