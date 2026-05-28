# XLSX 解析器

XLSX 解析器是 [Eino](https://github.com/cloudwego/eino) 的文档解析组件，实现了用于解析 Excel (XLSX) 文件的 'Parser' 接口。该组件支持灵活的表格解析配置，处理带有或不带有表头的 Excel 文件，支持选择特定工作表，并自定义文档 ID 前缀。

## 功能特性

- 支持带有或不带有表头的 Excel 文件
- 可选择多个工作表中的一个进行处理
- 自定义文档 ID 前缀
- 自动将表格数据转换为文档格式
- 将完整的行数据保留为元数据
- 支持注入额外的元数据

## 使用示例

- 参考当前目录中的 xlsx_parser_test.go，其中测试数据位于 ./examples/testdata/
    - TestXlsxParser_Default: 默认配置使用第一个工作表，第一行作为表头
    - TestXlsxParser_WithAnotherSheet: 使用第二个工作表，第一行作为表头
    - TestXlsxParser_WithHeader: 使用第三个工作表，第一行不作为表头
    - TestXlsxParser_WithIDPrefix: 使用 IDPrefix 自定义输出文档的 ID

## 元数据说明

遍历通过 docs 获取的 doc，doc.Metadata 包含以下两种类型的元数据：

- `_row`: 包含数据的结构化映射
- `_ext`: 通过解析选项注入的额外元数据
- 示例：
    - {
      "_row": {
          "name": "lihua",
          "age": "21"
      },
      "_ext": {
          "test": "test"
      }
      }

其中 '_row' 仅在第一行是表头时才有值；
当然，你也可以直接遍历 docs，从 doc.Content 开始：直接获取文档行的内容。

## 示例

查看以下示例了解更多用法：

- [测试数据示例](./examples/testdata/)

## 许可证

本项目采用 [Apache-2.0 License](LICENSE.txt) 许可。
