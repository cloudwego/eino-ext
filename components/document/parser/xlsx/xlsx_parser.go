package xlsx

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino/components/document/parser"

	"github.com/xuri/excelize/v2"
)

// XlsxParser 自定义解析器，用于解析excel文件内容
type XlsxParser struct {
	Config *Config
}

// Config 用于配置xlsxParser
type Config struct {
	SheetName string // 指定要处理的工作表名称，为空则处理第一张表
	HasHeader bool   // 是否包含表头
}

// NewXlsxParser 创建一个新的xlsxParser
func NewXlsxParser(ctx context.Context, config *Config) (xlp parser.Parser, err error) {
	// 默认配置HasHeader为true，表示第一行为表头，默认配置SheetName为文件的第一张表
	if config == nil {
		config = &Config{
			HasHeader: true,
			SheetName: "Sheet1",
		}
	}
	xlp = &XlsxParser{Config: config}
	return xlp, nil
}

// Parse 实现自定义解析器接口
func (xlp *XlsxParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) ([]*schema.Document, error) {
	xlFile, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, err
	}
	defer xlFile.Close()

	// 获取所有工作表
	sheets := xlFile.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil
	}

	// 确定要处理的工作表，默认只处理第一个工作表
	sheetName := sheets[0]
	if xlp.Config.SheetName != "" {
		sheetName = xlp.Config.SheetName
	}

	// 获取所有行，表头+数据行
	rows, err := xlFile.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	var ret []*schema.Document

	// 处理表头
	startIdx := 0
	var headers []string
	if xlp.Config.HasHeader && len(rows) > 0 {
		headers = rows[0]
		startIdx = 1
	}
	// 处理数据行
	for i := startIdx; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		// 将行数据转换为字符串
		contentParts := make([]string, len(row))
		for j, cell := range row {
			contentParts[j] = strings.TrimSpace(cell)
		}
		content := strings.Join(contentParts, "\t")

		// 创建新的Document
		nDoc := &schema.Document{
			ID:       fmt.Sprintf("%d", i),
			Content:  fmt.Sprintf("%s", content),
			MetaData: map[string]any{},
		}

		// 如果有表头，将数据添加到元数据中
		if xlp.Config.HasHeader {
			if nDoc.MetaData == nil {
				nDoc.MetaData = make(map[string]any)
			}
			for j, header := range headers {
				if j < len(row) {
					nDoc.MetaData[header] = row[j]
				}
			}
		}

		ret = append(ret, nDoc)
	}

	return ret, nil
}
