# JQ Document Transformer

A powerful document transformation component for Eino that leverages [gojq](https://github.com/itchyny/gojq) (Go implementation of [jq](https://stedolan.github.io/jq/)) to manipulate and aggregate documents.

## Features

This transformer provides several powerful capabilities for document processing:

1. **Individual Transformation** - Transform individual documents using jq expressions
2. **Join Aggregation** - Combine multiple source documents into a target document
3. **Hierarchical Aggregation** - Combine documents based on hierarchical levels
4. **Custom Function Transforms** - Apply custom Go functions to documents

## Configuration

The transformer is configured using YAML (or equivalent JSON):

```yaml
# Individual transformation
transform: |
  .content = (.content + " (transformed)")
  | .meta_data.status = "processed"

# Aggregation rules
aggregation:
  rules:
    - name: "Rule name"
      source_selector: '.meta_data.type == "source_type"'
      target_selector: '.meta_data.type == "target_type"'
      action:
        mode: "join"  # or "hierarchical_by_level"
        join_separator: " | "
        level_key: "level"  # only for hierarchical mode

# Custom function transforms
custom_transforms:
  - name: "Custom transform name"
    selector: '.meta_data.needs_transform == true'
    function: "registered_function_name"
    target_key: "result_field"
    args: [ .id, .meta_data.some_field ]
```

## Usage Examples

### Individual Transformation

Transform document content and metadata using jq expressions:

```yaml
transform: |
  .content = (.content + " (processed)")
  | .meta_data.status = "complete"
```

### Join Aggregation

Combine content from multiple source documents into target documents:

```yaml
aggregation:
  rules:
    - name: "Combine prose fragments"
      source_selector: '.meta_data.type == "prose"'
      target_selector: '.meta_data.type == "summary"'
      action:
        mode: "join"
        join_separator: "\n\n"
```

This aggregation will find all documents with `meta_data.type="prose"` and combine their content with the specified separator, then append it to documents with `meta_data.type="summary"`.

### Hierarchical Aggregation

Build document content based on hierarchical levels:

```yaml
aggregation:
  rules:
    - name: "Build hierarchical document"
      source_selector: '.meta_data.chapter != null'
      target_selector: '.meta_data.type == "full_document"'
      action:
        mode: "hierarchical_by_level"
        level_key: "chapter"
        join_separator: "\n\n"
```

In this example, source documents with a `chapter` metadata field will be organized hierarchically, and their content will be combined (in reverse level order) into target documents with `meta_data.type="full_document"`. 

The target document must also have a level value in its metadata for proper placement in the hierarchy.

### Custom Function Transforms

Apply registered Go functions to transform documents:

```yaml
custom_transforms:
  - name: "Generate authentication token"
    selector: '.meta_data.requires_auth == true'
    function: "generate_auth_token"
    target_key: "auth_token"
    args: [ .id, .meta_data.user_id ]
```

This example will call a registered Go function named `generate_auth_token` for documents where `meta_data.requires_auth=true`, passing the document ID and user ID as arguments. The result will be stored in the document's metadata under `auth_token`.

## Function Registry

To use custom functions, you must register them when creating the transformer. Example:

```go
funcRegistry := map[string]any{
    "generate_auth_token": myTokenGenerator,
    "compute_checksum": myChecksumFunction,
}

rules, err := jq.NewTransformerRules(&config, funcRegistry)
```

Custom functions must follow this signature pattern:
- Return a value and optionally an error
- Accept the number of arguments defined in your transform configuration

## Processing Order

Documents are processed in the following order:

1. Individual transformations are applied to each document
2. Aggregation rules collect and combine content according to selectors
3. Custom functions are executed on matching documents

This order allows transformed documents to participate in aggregations and custom transforms.