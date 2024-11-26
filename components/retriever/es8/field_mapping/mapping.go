package field_mapping

import (
	"fmt"

	"code.byted.org/flow/eino-ext/components/retriever/es8/internal"
	"code.byted.org/flow/eino/schema"
)

// GetDefaultVectorFieldKeyContent get default es key for Document.Content
func GetDefaultVectorFieldKeyContent() FieldName {
	return defaultVectorFieldKeyContent
}

// GetDefaultVectorFieldKey generate default vector field name from its field name
func GetDefaultVectorFieldKey(fieldName string) FieldName {
	return FieldName(fmt.Sprintf("vector_%s", fieldName))
}

// GetExtraDataFields get data fields from *schema.Document
func GetExtraDataFields(doc *schema.Document) (fields map[string]interface{}, ok bool) {
	if doc == nil || doc.MetaData == nil {
		return nil, false
	}

	fields, ok = doc.MetaData[internal.DocExtraKeyEsFields].(map[string]interface{})

	return fields, ok
}

type FieldKV struct {
	// FieldNameVector vector field name (if needed)
	FieldNameVector FieldName `json:"field_name_vector,omitempty"`
	// FieldName field name
	FieldName FieldName `json:"field_name,omitempty"`
	// Value original value
	Value string `json:"value,omitempty"`
}

type FieldName string

var defaultVectorFieldKeyContent = GetDefaultVectorFieldKey(DocFieldNameContent)
