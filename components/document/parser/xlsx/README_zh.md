# XLSX 解析器

简体中文 | [English](README.md)

XLSX 解析器是Eino的一个文档解析组件的一种，它实现了 `Parser` 接口，用于解析 Excel (XLSX) 文件。该组件支持灵活的表格解析配置，可以处理有表头或无表头的 Excel 文件，并支持多工作表的选择。

## 功能特性

- 支持解析带表头和不带表头的 Excel 文件
- 支持多工作表的选择和处理
- 自动将表格数据转换为文档格式
- 保留完整的行数据作为元数据
- 支持额外元数据的注入

## 使用示例
- 参考当前目录下xlsx_parser_test.go，测试xlsx文件在当前目录./testdata/下
  - TestXlsxParser_Default: 默认配置，使用第一张工作表，第一行不作表头
  - TestXlsxParser_WithAnotherSheet: 使用第二张工作表，第一行不做表头
  - TestXlsxParser_WithHeader: 使用第三张工作表，第一行作为表头

## 元数据说明

解析后的文档元数据包含以下字段，用户可直接从doc里面的MetaData中获取：

- `_row`: 包含行数据的映射，如果设置了 `HasHeader`，则使用表头作为键
- `_ext`: 通过解析选项注入的额外元数据

- 示例:
  - {
    "_row": {
        "姓名": "李华",
        "年龄": "21"
    },
    "_ext": {
        "test": "test"
    }
    }

## License

This project is licensed under the [Apache-2.0 License](LICENSE.txt).