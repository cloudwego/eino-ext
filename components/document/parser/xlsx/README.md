# XLSX Parser

The XLSX parser is [Eino](https://github.com/cloudwego/eino)'s document parsing component that implements the 'Parser' interface for parsing Excel (XLSX) files. The component supports flexible table parsing configurations, handles Excel files with or without headers, supports the selection of a specific worksheet, and customizes the document ID prefix.

## Features

- Support for Excel files with or without headers
- Select one of the multiple sheets to process
- Custom document id prefixes
- Automatic conversion of table data to document format
- Preservation of complete row data as metadata
- Support for additional metadata injection
- Flexible column selection for content and metadata
- Column name customization
- Optional metadata generation control

## Configuration Options

### Basic Configuration

- `SheetName`: Name of the worksheet to process (default: first sheet)
- `IDPrefix`: Custom prefix for document IDs (default: sequential numbers)

### Columns Configuration

- `NoHeader`: If true, the first row is treated as data rather than header (default: false)
- `NoRowMeta`: If true, no `_row` metadata will be generated (default: false)
- `Content`: Array of column letters to use for document content (e.g., ["A", "D", "F"])
- `Meta`: Array of column letters to add directly to the document's metadata (e.g., ["B", "C"])
- `CustomNames`: Map of column letters to custom names (e.g., {"A": "Name", "B": "Age"})

## Metadata Structure

The XLSX parser generates metadata with the following structure:

1. `_row`: Contains all columns from the Excel file (unless `NoRowMeta` is true)
   - Column names from headers or A, B, C if `NoHeader` is true
   - Custom column names are applied if specified in `CustomNames`

2. Columns specified in `Meta` are also added directly to the root metadata level for easy access

## Example of use
- Refer to xlsx_parser_test.go in the current directory, where the test data is in ./examples/testdata/
    - TestXlsxParser_Default: The default configuration uses the first worksheet with the first row as the header
    - TestXlsxParser_WithAnotherSheet: Use the second sheet with the first row as the header
    - TestXlsxParser_WithNoHeader: Use the third sheet with the first row treated as data, not as a header
    - TestXlsxParser_WithIDPrefix: Use IDPrefix to customize the ID of the output document
    - TestXlsxParser_WithColumns_Content: Specify which columns to use for document content
    - TestXlsxParser_WithColumns_Meta: Specify columns to include in metadata
    - TestXlsxParser_WithColumns_CustomNames: Use custom names for columns
    - TestXlsxParser_WithNoRowMeta: Disable _row metadata generation

## Usage Examples

### Basic Usage with Default Options

```go
parser, err := xlsx.NewXlsxParser(ctx, nil)
docs, err := parser.Parse(ctx, reader)
```

### Custom Configuration

```go
config := &xlsx.Config{
    SheetName: "Sheet2",
    IDPrefix: "doc_",
    Columns: xlsx.Columns{
        NoHeader: false,
        Content: []string{"A", "B"},
        Meta: []string{"C", "D"},
        CustomNames: map[string]string{
            "C": "Age",
            "D": "Gender",
        },
    },
}

parser, err := xlsx.NewXlsxParser(ctx, config)
docs, err := parser.Parse(ctx, reader)
```

### Disabling Row Metadata

```go
config := &xlsx.Config{
    Columns: xlsx.Columns{
        NoRowMeta: true,
    },
}

parser, err := xlsx.NewXlsxParser(ctx, config)
docs, err := parser.Parse(ctx, reader)
```

## License

This project is licensed under the [Apache-2.0 License](LICENSE.txt).