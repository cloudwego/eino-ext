# XLSX Parser

The XLSX parser is [Eino](https://github.com/cloudwego/eino)'s document parsing component that implements the 'Parser' interface for parsing Excel (XLSX) files. The component supports flexible table parsing configurations, handles Excel files with or without headers, supports the selection of a specific worksheet, and customizes the document ID prefix.

## Features

- Support for Excel files with or without headers
- Select one of the multiple sheets to process
- Custom document id prefixes
- Automatic conversion of table data to document format
- Preservation of complete row data as metadata
- Support for additional metadata injection

## Example of use
- Refer to xlsx_parser_test.go in the current directory, where the test data is in ./examples/testdata/
    - TestXlsxParser_Default: The default configuration uses the first worksheet with the first row as the header
    - TestXlsxParser_WithAnotherSheet: Use the second sheet with the first row as the header
    - TestXlsxParser_WithHeader: Use the third sheet with the first row is not used as the header
    - TestXlsxParser_WithIDPrefix: Use IDPrefix to customize the ID of the output document

## Metadata Description

Traversing the doc obtained by docs, doc.Metadata contains the following two types of metadata:

- `_row`: Structured mappings that contain data
- `_ext`: Additional metadata injected via parsing options
- example:
    - {
      "_row": {
          "name": "lihua",
          "age": "21"
      },
      "_ext": {
          "test": "test"
      }
      }

where '_row' has a value only if the first row is the header; 
Of course, you can also go directly through docs, starting with doc.Content: Get the content of the document line directly.

## License

This project is licensed under the [Apache-2.0 License](LICENSE.txt).