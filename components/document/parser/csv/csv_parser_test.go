package csv

import (
	"context"
	"os"
	"testing"

	"github.com/cloudwego/eino/components/document/parser"
)

func TestCSVParser(t *testing.T) {
	f, err := os.Open("./examples/testdata/test.csv")
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()

	ctx := context.Background()
	cp, err := NewCSVParser(ctx, &Config{})
	if err != nil {
		t.Error(err)
		return
	}

	docs, err := cp.Parse(ctx, f, parser.WithURI("./examples/testdata/test.csv"), parser.WithExtraMeta(map[string]any{
		"_extension": ".csv",
		"_file_name": "test.csv",
		"_source":    "local",
	}))

	if err != nil {
		t.Error(err)
		return
	}
	t.Log(docs)
	return
}
