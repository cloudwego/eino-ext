package jq

import (
	"context"
	"fmt"
	"reflect"
	"slices"
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

// Transform is the main entry point called by the Eino framework.
func (t *Transformer) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) (documents []*schema.Document, err error) {
	option := implOptions{}
	document.GetTransformerImplSpecificOptions(&option, opts...)

	rules := option.Rules
	if rules == nil {
		rules = t.Rules
	}

	if rules == nil {
		err = fmt.Errorf("jq transformer rules not provided in options and no default rules available")
		return
	}

	documents, err = rules.Transform(ctx, docs, opts...)
	return
}

// --- Configuration Structs with JSON annotations ---
type AggregationAction struct {
	SourceField   string `yaml:"source_field" json:"source_field"`
	TargetField   string `yaml:"target_field" json:"target_field"`
	Mode          string `yaml:"mode" json:"mode"`
	JoinSeparator string `yaml:"join_separator" json:"join_separator"`
	LevelKey      string `yaml:"level_key" json:"level_key"`
}

type AggregationRule struct {
	Name           string            `yaml:"name" json:"name"`
	SourceSelector string            `yaml:"source_selector" json:"source_selector"`
	TargetSelector string            `yaml:"target_selector" json:"target_selector"`
	Action         AggregationAction `yaml:"action" json:"action"`
	sourceQuery    *gojq.Query
	targetQuery    *gojq.Query
}

type CustomTransform struct {
	Name          string   `yaml:"name" json:"name"`
	Selector      string   `yaml:"selector" json:"selector"`
	Function      string   `yaml:"function" json:"function"`
	TargetKey     string   `yaml:"target_key" json:"target_key"`
	Args          []string `yaml:"args" json:"args"`
	selectorQuery *gojq.Query
	argQueries    []*gojq.Query
}

type Config struct {
	Transform   string `yaml:"transform" json:"transform"`
	Aggregation struct {
		Rules []AggregationRule `yaml:"rules" json:"rules"`
	} `yaml:"aggregation" json:"aggregation"`
	CustomTransforms []CustomTransform `yaml:"custom_transforms" json:"custom_transforms"`
}

// --- Core Logic Implementation ---

type TransformerRules struct {
	transformQuery   *gojq.Query
	aggregationRules []AggregationRule
	customTransforms []CustomTransform
	functionRegistry map[string]any
}

func NewTransformerRules(cfg *Config, funcRegistry map[string]any) (transformer *TransformerRules, err error) {
	if cfg == nil {
		err = fmt.Errorf("configuration cannot be nil")
		return
	}

	transformer = &TransformerRules{
		functionRegistry: funcRegistry,
		aggregationRules: cfg.Aggregation.Rules,
		customTransforms: cfg.CustomTransforms,
	}

	// Simple parse, no special options needed.
	if cfg.Transform != "" {
		transformer.transformQuery, err = gojq.Parse(cfg.Transform)
		if err != nil {
			err = fmt.Errorf("failed to parse transform query: %w", err)
			return
		}
	}
	for i := range transformer.aggregationRules {
		rule := &transformer.aggregationRules[i]
		rule.sourceQuery, err = gojq.Parse(rule.SourceSelector)
		if err != nil { /* ... handle error ... */
		}
		rule.targetQuery, err = gojq.Parse(rule.TargetSelector)
		if err != nil { /* ... handle error ... */
		}
	}
	for i := range transformer.customTransforms {
		rule := &transformer.customTransforms[i]
		rule.selectorQuery, err = gojq.Parse(rule.Selector)
		if err != nil { /* ... handle error ... */
		}
		rule.argQueries = make([]*gojq.Query, len(rule.Args))
		for j, argExpr := range rule.Args {
			rule.argQueries[j], err = gojq.Parse(argExpr)
			if err != nil { /* ... handle error ... */
			}
		}
	}
	return
}

func (t *TransformerRules) Transform(ctx context.Context, docs []*schema.Document, opts ...document.TransformerOption) (documents []*schema.Document, err error) {
	documents = docs
	joinBuffers := make(map[string][]string)
	hierarchicalBuffers := make(map[string]map[int]*schema.Document)

	for _, doc := range docs {
		// Consistently convert the doc to a map at the beginning of the loop.
		docAsMap := docToMap(doc)

		// Part 1: Individual Transformation
		if t.transformQuery != nil {
			var transformedMap map[string]any
			transformedMap, err = t.applyIndividualTransform(docAsMap)
			if err != nil {
				documents = nil
				return
			}
			// Update the document from the transformed map before proceeding.
			if newContent, exists := transformedMap["content"].(string); exists {
				doc.Content = newContent
			}
			if newMeta, exists := transformedMap["meta_data"].(map[string]any); exists {
				doc.MetaData = newMeta
			}
			// Refresh docAsMap in case the transform changed it.
			docAsMap = docToMap(doc)
		}

		// Part 2: Stateful Aggregation
		for i := range t.aggregationRules {
			rule := &t.aggregationRules[i]
			switch rule.Action.Mode {
			case "hierarchical_by_level":
				err = t.handleHierarchicalAggregation(doc, docAsMap, rule, hierarchicalBuffers)
			default:
				err = t.handleJoinAggregation(doc, docAsMap, rule, joinBuffers)
			}
			if err != nil {
				documents = nil
				return
			}
		}

		// Part 3: Custom Go Functions
		for i := range t.customTransforms {
			rule := &t.customTransforms[i]
			var isTarget bool
			isTarget, err = checkCondition(rule.selectorQuery, docAsMap)
			if err != nil {
				err = fmt.Errorf("rule '%s' selector check failed: %w", rule.Name, err)
				documents = nil
				return
			}
			if isTarget {
				if err = t.applyCustomFunction(doc, rule, docAsMap); err != nil {
					documents = nil
					return
				}
			}
		}
	}
	return
}

// --- Handlers & Helpers ---

func (t *TransformerRules) applyIndividualTransform(input map[string]any) (result map[string]any, err error) {
	iter := t.transformQuery.Run(input)
	v, ok := iter.Next()
	if !ok {
		result = input // No change, return the original map
		return
	}
	if e, isErr := v.(error); isErr {
		err = fmt.Errorf("query error: %w", e)
		return
	}
	result, ok = v.(map[string]any)
	if !ok {
		err = fmt.Errorf("query did not return a map")
	}
	return
}

func (t *TransformerRules) handleJoinAggregation(doc *schema.Document, docAsMap map[string]any, rule *AggregationRule, buffers map[string][]string) (err error) {
	isTarget, err := checkCondition(rule.targetQuery, docAsMap)
	if err != nil {
		err = fmt.Errorf("rule '%s' target check failed: %w", rule.Name, err)
		return
	}
	if isTarget {
		if sourceContents := buffers[rule.Name]; len(sourceContents) > 0 {
			aggregatedContent := strings.Join(sourceContents, rule.Action.JoinSeparator)
			doc.Content += "\n\n--- Aggregated Content ---\n\n" + aggregatedContent
			buffers[rule.Name] = nil
		}
	}
	isSource, err := checkCondition(rule.sourceQuery, docAsMap)
	if err != nil {
		err = fmt.Errorf("rule '%s' source check failed: %w", rule.Name, err)
		return
	}
	if isSource {
		buffers[rule.Name] = append(buffers[rule.Name], doc.Content)
	}
	return
}

func (t *TransformerRules) handleHierarchicalAggregation(doc *schema.Document, docAsMap map[string]any, rule *AggregationRule, buffers map[string]map[int]*schema.Document) (err error) {
	isTarget, err := checkCondition(rule.targetQuery, docAsMap)
	if err != nil {
		err = fmt.Errorf("rule '%s' target check failed: %w", rule.Name, err)
		return
	}
	if isTarget {
		buffer := buffers[rule.Name]
		if len(buffer) == 0 {
			return
		}
		targetLevelVal, ok := doc.MetaData[rule.Action.LevelKey]
		if !ok {
			err = fmt.Errorf("target doc %s missing level_key '%s'", doc.ID, rule.Action.LevelKey)
			return
		}
		var targetLevel int
		targetLevel, ok = toInt(targetLevelVal)
		if !ok {
			err = fmt.Errorf("level_key for doc %s is not a valid integer", doc.ID)
			return
		}

		contentsToAggregate := []string{doc.Content}
		for i := targetLevel - 1; i >= 1; i-- {
			if sourceDoc, ok := buffer[i]; ok {
				contentsToAggregate = append(contentsToAggregate, sourceDoc.Content)
			}
		}

		if len(contentsToAggregate) > 1 {
			slices.Reverse(contentsToAggregate)
			doc.Content = strings.Join(contentsToAggregate, rule.Action.JoinSeparator)
		}
		delete(buffers, rule.Name)
	}
	isSource, err := checkCondition(rule.sourceQuery, docAsMap)
	if err != nil {
		err = fmt.Errorf("rule '%s' source check failed: %w", rule.Name, err)
		return
	}
	if isSource {
		sourceLevelVal, ok := doc.MetaData[rule.Action.LevelKey]
		if !ok {
			err = fmt.Errorf("source doc %s missing level_key '%s'", doc.ID, rule.Action.LevelKey)
			return
		}
		var sourceLevel int
		sourceLevel, ok = toInt(sourceLevelVal)
		if !ok {
			err = fmt.Errorf("level_key for doc %s is not a valid integer", doc.ID)
			return
		}
		if buffers[rule.Name] == nil {
			buffers[rule.Name] = make(map[int]*schema.Document)
		}
		buffers[rule.Name][sourceLevel] = doc
	}
	return
}

func (t *TransformerRules) applyCustomFunction(doc *schema.Document, rule *CustomTransform, docAsMap map[string]any) (err error) {
	goFunc, ok := t.functionRegistry[rule.Function]
	if !ok {
		err = fmt.Errorf("custom function '%s' not found", rule.Function)
		return
	}
	var args []reflect.Value
	args, err = t.extractArgs(rule.argQueries, docAsMap)
	if err != nil {
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
	doc.MetaData[rule.TargetKey] = results[0].Interface()
	return
}

func (t *TransformerRules) extractArgs(argQueries []*gojq.Query, input map[string]any) (args []reflect.Value, err error) {
	args = make([]reflect.Value, len(argQueries))
	for i, q := range argQueries {
		iter := q.Run(input)
		v, ok := iter.Next()
		if !ok {
			err = fmt.Errorf("arg query %d produced no value", i)
			args = nil
			return
		}
		args[i] = reflect.ValueOf(v)
	}
	return
}

func checkCondition(query *gojq.Query, input map[string]any) (isMatch bool, err error) {
	iter := query.Run(input)
	v, ok := iter.Next()
	if !ok {
		isMatch = false
		return
	}
	if e, isErr := v.(error); isErr {
		err = e
		return
	}
	result, ok := v.(bool)
	if !ok {
		err = fmt.Errorf("selector query did not return a boolean, got %T", v)
		return
	}
	isMatch = result
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

func toInt(v any) (i int, ok bool) {
	switch val := v.(type) {
	case int:
		i, ok = val, true
	case int64:
		i, ok = int(val), true
	case float64:
		i, ok = int(val), true
	}
	return
}
