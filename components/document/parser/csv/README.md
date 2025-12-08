# CSV Parser

The CSV parser is [Eino](https://github.com/cloudwego/eino)'s document parsing component that implements the 'Parser' interface for parsing CSV files. 

The component supports flexible table parsing configurations, handles CSV files with or without headers, supports the specified comma and comment, and customizes the document ID prefix. The CSV Parser refers to the existing Excel Parser from the same project and maintained consistency in the design specifications.

## Features
- Support for CSV files with or without headers
- Custom document id prefixes
- Automatic conversion of table data to document format
- Preservation of complete row data as metadata
- Support for additional metadata injection

## Example

Refer to `./csv_parser_test.go` for detailed examples:
- `TestCSVParser`: New a CSV parser by configuration and parse a CSV file from io.Reader.

## Metadata Description

Traversing the doc obtained by docs, doc.Metadata contains the following two types of metadata:

- `_row`: Structured mappings that contain data
- `_ext`: Additional metadata injected via parsing options

Example:
```json
{
    "_row": {
        "name": "lihua",
        "age": "21"
    },
    "_ext": {
        "test": "test"
    }
}
```

**Note**: The `_row` field has a value only if the first row is the header.

You can also directly access `doc.Content` to get the content of the document line.