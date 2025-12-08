package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/document/parser/csv"
	"github.com/cloudwego/eino/components/document/parser"
)

func main() {

	// 1. Open the CSV file.
	f, err := os.Open("./testdata/test.csv")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	// 2. Create a new CSVParser.
	ctx := context.Background()
	cp, err := csv.NewCSVParser(ctx, &csv.Config{
		NoHeader: false,
		IDPrefix: "",
	})
	if err != nil {
		log.Fatalf("Failed to create CSVParser: %v", err)
	}

	// 3. Parse the CSV content.
	docs, err := cp.Parse(ctx, f, parser.WithURI("./testdata/test.csv"), parser.WithExtraMeta(map[string]any{
		"_extension": ".csv",
		"_file_name": "test.csv",
		"_source":    "local",
	}))

	if err != nil {
		log.Fatalf("Failed to parse CSV: %v", err)
	}
	log.Printf("Parsed documents content %s \n", docs)
	return
}
