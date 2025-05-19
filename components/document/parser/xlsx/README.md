# XLSX Parser

English | [简体中文](README.zh_CN.md)

The XLSX parser is a document parsing component of [Eino](https://github.com/cloudwego/eino), which implements the 'Parser' interface for parsing Excel (XLSX) files. This component supports flexible table parsing configurations, can process Excel files with or without table headers, supports the selection of multiple worksheets, and can customize the document ID prefix.

## Features

- Support for Excel files with or without headers
- Multiple worksheet selection and processing
- Automatic conversion of table data to document format
- Preservation of complete row data as metadata
- Support for additional metadata injection

## Example of use
- Refer to xlsx_parser_test.go in the current directory and test the xlsx file in the current directory ./testdata/
    - TestXlsxParser_Default: The default configuration uses the first worksheet with the first row as the header
    - TestXlsxParser_WithAnotherSheet: Use the second sheet with the first row as the header
    - TestXlsxParser_WithHeader: Use the third sheet with the first row is not used as the header

## Metadata Description

The parsed document metadata contains the following fields, which can be obtained from the metadata in doc by directly traversing docs:

- `_row`: Mapping containing row data, using the table header as the key if `HasHeader` is set
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