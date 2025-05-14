# XLSX Parser

English | [简体中文](README_zh.md)

The XLSX Parser is a document parsing component for [Eino](https://github.com/cloudwego/eino) that implements the `Parser` interface for parsing Excel (XLSX) files. This component supports flexible table parsing configurations, capable of handling Excel files with or without headers, and supports multiple worksheet selection.

## Features

- Support for Excel files with or without headers
- Multiple worksheet selection and processing
- Automatic conversion of table data to document format
- Preservation of complete row data as metadata
- Support for additional metadata injection

## Example of use
- Refer to xlsx_parser_test.go in the current directory and test the xlsx file in the current directory ./testdata/
    - TestXlsxParser_Default: The default configuration uses the first worksheet, and the first row does not have a header
    - TestXlsxParser_WithAnotherSheet: Use the second sheet with no header in the first row
    - TestXlsxParser_WithHeader: Use the third sheet with the first row as the header

## Metadata Description

The parsed document metadata includes the following fields, Users can get it directly from the MetaData in the doc:

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

## License

This project is licensed under the [Apache-2.0 License](LICENSE.txt).